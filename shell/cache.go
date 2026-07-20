package shell

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// cacheManifestVersion is the manifest schema version. A mismatch invalidates the
// cache so an older cache layout is rebuilt rather than misread. Version 2 added a
// content hash to the source signature (issue #592); a version 1 manifest is
// treated as stale and rebuilt.
const cacheManifestVersion = 2

// cacheSource is the invalidation signature of one input file: its absolute path,
// size, and a SHA-256 hash of its contents. The content hash detects edits that
// leave the file size and modification time unchanged, which a size+mtime
// signature would miss and so reuse stale cached data (issue #592).
type cacheSource struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	// Hash is the hex-encoded SHA-256 digest of the file contents.
	Hash string `json:"hash"`
}

// cacheManifest records what an import cache contains and the inputs it was built
// from, so a later run can decide whether the cache is still valid and can
// restore the table-to-source mapping without re-importing.
type cacheManifest struct {
	Version      int               `json:"version"`
	Sources      []cacheSource     `json:"sources"`
	TableSources map[string]string `json:"table_sources"`
	DirImported  []string          `json:"dir_imported"`
}

// cacheManifestPath returns the sidecar manifest path for a cache database path.
func cacheManifestPath(cachePath string) string {
	return cachePath + ".manifest.json"
}

// isCacheArtifact reports whether path is one of sqly's own cache files (the
// cache database or its manifest sidecar). When --cache points inside a directory
// that is also imported, these files must never be treated as dataset inputs:
// importing the manifest would create a stray table, and counting either file in
// the cache signature would invalidate the cache on every run. Paths are compared
// after resolving to absolute form so a relative input still matches.
func (s *Shell) isCacheArtifact(path string) bool {
	if s.argument.CachePath == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	for _, artifact := range []string{s.argument.CachePath, cacheManifestPath(s.argument.CachePath)} {
		artifactAbs, err := filepath.Abs(artifact)
		if err != nil {
			artifactAbs = artifact
		}
		if abs == artifactAbs {
			return true
		}
	}
	return false
}

// cacheEnabled reports whether this run should use the import cache. Caching is
// opt-in (--cache), needs file inputs, and is skipped for a --stdin dataset
// (ephemeral, no stable signature) and for ACH/Fedwire inputs (their write-back
// needs the live filesql registry, which a cache load would not restore).
func (s *Shell) cacheEnabled(paths []string) bool {
	if s.argument.CachePath == "" || len(paths) == 0 || s.argument.StdinFormat != "" {
		return false
	}
	for _, p := range paths {
		if isRemoteURL(p) {
			return false
		}
	}
	// Disable caching when any input is ACH/Fedwire, including one nested inside a
	// directory argument: a warm cache load restores plain tables but not the
	// filesql registry those formats need for write-back, so caching them would
	// silently break a later .save.
	for _, p := range paths {
		if containsInputOnlyFile(p) {
			return false
		}
	}
	return true
}

// containsInputOnlyFile reports whether p is an ACH/Fedwire file or a directory
// that contains one (searched recursively). A path that cannot be stat-ed or
// walked is treated as not input-only; the import step will surface any real
// access error.
func containsInputOnlyFile(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return model.IsInputOnlyExtension(p)
	}
	found := false
	_ = filepath.WalkDir(p, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries; import will report real errors
		}
		if model.IsInputOnlyExtension(path) {
			found = true
		}
		return nil
	})
	return found
}

// loadOrImport runs the import step, using the cache when enabled. On a warm hit
// (cache present and inputs unchanged) it restores the session from the cache and
// skips parsing the source files; otherwise it imports normally and, when caching
// is enabled, writes a fresh cache. A cache read or write failure never fails the
// run: it falls back to a cold import and warns on stderr.
func (s *Shell) loadOrImport(ctx context.Context, paths []string) error {
	if !s.cacheEnabled(paths) {
		return s.commands.importCommand(ctx, s, paths)
	}

	cachePath := s.argument.CachePath
	if s.argument.CacheClear {
		s.clearCache(cachePath)
	}

	sigs, sigErr := collectCacheSignatures(paths, s.isCacheArtifact, s.usecases.importer.IsSupportedFile)
	if sigErr == nil {
		if s.tryWarmCache(ctx, cachePath, sigs) {
			return nil
		}
	} else {
		fmt.Fprintf(config.Stderr, "cache: cannot read input metadata (%v); importing without cache\n", sigErr)
	}

	// Cold import.
	if err := s.commands.importCommand(ctx, s, paths); err != nil {
		return err
	}
	if sigErr == nil {
		s.writeCache(ctx, cachePath, sigs)
	}
	return nil
}

// tryWarmCache attempts to restore the session from an existing, still-valid
// cache. It returns true only when the cache was loaded and the session state
// (tables, source mapping, change baselines) was restored.
func (s *Shell) tryWarmCache(ctx context.Context, cachePath string, sigs []cacheSource) bool {
	manifest, err := readCacheManifest(cacheManifestPath(cachePath))
	if err != nil {
		return false // no usable cache yet
	}
	if manifest.Version != cacheManifestVersion || !cacheSignaturesMatch(manifest.Sources, sigs) {
		return false // stale: inputs changed or layout differs
	}
	if _, statErr := os.Stat(cachePath); statErr != nil {
		return false // manifest without its database
	}
	if err := s.usecases.persistence.LoadFromCache(ctx, cachePath); err != nil {
		fmt.Fprintf(config.Stderr, "cache: load failed (%v); importing from source\n", err)
		return false
	}
	s.restoreFromManifest(ctx, manifest)
	fmt.Fprintf(config.Stderr, "cache: reused %s\n", cachePath)
	return true
}

// restoreFromManifest rebuilds the per-table session state that a normal import
// would have recorded: the table-to-source mapping, the directory-import marks,
// and the change baselines used by write-back.
func (s *Shell) restoreFromManifest(ctx context.Context, manifest cacheManifest) {
	if s.tableSources == nil {
		s.tableSources = make(map[string]string)
	}
	for name, src := range manifest.TableSources {
		s.tableSources[name] = src
	}
	if s.dirImported == nil {
		s.dirImported = make(map[string]bool)
	}
	for _, name := range manifest.DirImported {
		s.dirImported[name] = true
	}
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return
	}
	for _, t := range tables {
		s.snapshotBaseline(ctx, t.Name())
	}
}

// writeCache snapshots the session and records a manifest for a later warm run.
// Any failure is reported on stderr but does not fail the run, since the query
// already succeeded against the in-memory session.
func (s *Shell) writeCache(ctx context.Context, cachePath string, sigs []cacheSource) {
	if dir := filepath.Dir(cachePath); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			fmt.Fprintf(config.Stderr, "cache: cannot create cache directory (%v); not caching\n", err)
			return
		}
	}
	if err := s.usecases.persistence.SnapshotToCache(ctx, cachePath); err != nil {
		fmt.Fprintf(config.Stderr, "cache: write failed (%v); continuing without cache\n", err)
		return
	}
	manifest := cacheManifest{
		Version:      cacheManifestVersion,
		Sources:      sigs,
		TableSources: s.tableSources,
		DirImported:  sortedTrueKeys(s.dirImported),
	}
	if err := writeCacheManifest(cacheManifestPath(cachePath), manifest); err != nil {
		fmt.Fprintf(config.Stderr, "cache: manifest write failed (%v); removing partial cache\n", err)
		s.clearCache(cachePath)
	}
}

// clearCache removes a cache database and its manifest, ignoring missing files.
func (s *Shell) clearCache(cachePath string) {
	for _, p := range []string{cachePath, cacheManifestPath(cachePath)} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(config.Stderr, "cache: cannot remove %s (%v)\n", p, err)
		}
	}
}

// collectCacheSignatures returns the invalidation signature for every input file,
// expanding directories recursively. Files for which skip returns true are
// excluded, so sqly's own cache artifacts inside an imported directory do not
// enter the signature. Inside a directory, only files for which dirSupported
// returns true are included, so a change to an unsupported sibling (a README, a
// .txt note) does not invalidate the cache when the imported dataset is
// unchanged; a directly named file is always included. The result is sorted by
// path so the signature is order-independent.
func collectCacheSignatures(paths []string, skip func(string) bool, dirSupported func(string) bool) ([]cacheSource, error) {
	var sigs []cacheSource
	add := func(p string) error {
		if skip != nil && skip(p) {
			return nil
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		info, err := os.Stat(abs)
		if err != nil {
			return err
		}
		hash, err := hashFile(abs)
		if err != nil {
			return err
		}
		sigs = append(sigs, cacheSource{Path: abs, Size: info.Size(), Hash: hash})
		return nil
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if err := add(p); err != nil {
				return nil, err
			}
			continue
		}
		walkErr := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if dirSupported != nil && !dirSupported(path) {
				return nil // an unsupported sibling is not part of the cache key
			}
			return add(path)
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	sort.Slice(sigs, func(i, j int) bool { return sigs[i].Path < sigs[j].Path })
	return sigs, nil
}

// hashFile returns the hex-encoded SHA-256 digest of a file's contents. It
// streams the file through the hash so memory use stays constant regardless of
// file size.
func hashFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // cache inputs are paths the user passed on the command line
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cacheSignaturesMatch reports whether two signature sets are identical (same
// files, sizes, and content hashes). Either set is assumed sorted by path.
func cacheSignaturesMatch(a, b []cacheSource) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// readCacheManifest decodes a cache manifest from disk.
func readCacheManifest(path string) (cacheManifest, error) {
	data, err := os.ReadFile(path) //nolint:gosec // cache path chosen by the user
	if err != nil {
		return cacheManifest{}, err
	}
	var m cacheManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return cacheManifest{}, err
	}
	return m, nil
}

// writeCacheManifest encodes a cache manifest to disk.
func writeCacheManifest(path string, m cacheManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// sortedTrueKeys returns the keys of a bool map whose value is true, sorted.
func sortedTrueKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
