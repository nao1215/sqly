package shell

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const remoteSupportedFormatsHelp = "csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed"

// isRemoteURL reports whether raw is an absolute HTTP/HTTPS URL sqly can
// download as an input dataset.
func isRemoteURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

// sameSourceLocation compares imported sources. Local files use sameFilePath so
// symlink aliases still match; remote URLs compare after normalization.
func sameSourceLocation(a, b string) bool {
	switch {
	case isRemoteURL(a) || isRemoteURL(b):
		return normalizeRemoteURL(a) == normalizeRemoteURL(b)
	default:
		return sameFilePath(a, b)
	}
}

// normalizeRemoteURL canonicalizes an import URL for source comparisons: the
// fragment is ignored because it never affects the HTTP response body.
func normalizeRemoteURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	return u.String()
}

// remoteFilenameHint returns the filename-like hint from the URL path. It is
// used for pre-download checks like --sheet validation and workbook counting.
func remoteFilenameHint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base := path.Base(u.Path)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

// isRemoteExcelURL reports whether the URL path hints an Excel workbook.
func isRemoteExcelURL(raw string) bool {
	name := remoteFilenameHint(raw)
	return name != "" && strings.HasSuffix(strings.ToLower(name), ".xlsx")
}

// downloadRemoteInput fetches a supported remote file to a temp path and
// returns that path plus a cleanup. The staged filename preserves the source
// extension so the existing filesql import path can detect the format.
func (s *Shell) downloadRemoteInput(ctx context.Context, rawURL string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build download request for %s: %w", rawURL, err)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", nil, fmt.Errorf("download %s: unexpected HTTP status %s", rawURL, resp.Status)
	}

	filename, err := s.remoteDownloadFilename(rawURL, resp)
	if err != nil {
		return "", nil, err
	}
	if !s.usecases.importer.IsSupportedFile(filename) {
		return "", nil, fmt.Errorf("unsupported remote file format: %s (supported: %s)", filename, remoteSupportedFormatsHelp)
	}

	dir, err := os.MkdirTemp("", "sqly-http-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir for %s: %w", rawURL, err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	localPath := filepath.Join(dir, filepath.Base(filename))
	f, err := os.Create(localPath) //nolint:gosec // localPath is under a sqly-created temp dir
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create staging file for %s: %w", rawURL, err)
	}

	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("download %s: %w", rawURL, copyErr)
	}
	if closeErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("close staging file for %s: %w", rawURL, closeErr)
	}
	return localPath, cleanup, nil
}

// remoteDownloadFilename chooses a filename for a downloaded input: first a
// Content-Disposition filename when present, then the URL path, then a
// Content-Type-derived extension. Candidate ranking reuses the importer's
// supported-format check so the extension list stays in one authority.
func (s *Shell) remoteDownloadFilename(rawURL string, resp *http.Response) (string, error) {
	var first string
	for _, candidate := range []string{
		contentDispositionFilename(resp.Header.Get("Content-Disposition")),
		remoteFilenameHint(rawURL),
		filenameFromContentType(resp.Header.Get("Content-Type")),
	} {
		candidate = filepath.Base(candidate)
		if candidate == "" || candidate == "." || candidate == string(filepath.Separator) {
			continue
		}
		if s.usecases.importer.IsSupportedFile(candidate) {
			return candidate, nil
		}
		if first == "" {
			first = candidate
		}
	}
	if first != "" {
		return first, nil
	}
	return "", fmt.Errorf("download %s: could not determine a supported filename from the URL, Content-Disposition, or Content-Type", rawURL)
}

func contentDispositionFilename(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	for _, key := range []string{"filename*", "filename"} {
		if v := params[key]; v != "" {
			return v
		}
	}
	return ""
}

func filenameFromContentType(header string) string {
	if header == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	switch strings.ToLower(mediaType) {
	case "text/csv", "application/csv":
		return "download.csv"
	case "text/tab-separated-values":
		return "download.tsv"
	case "text/ltsv", "application/ltsv":
		return "download.ltsv"
	case "application/json", "text/json":
		return "download.json"
	case "application/x-ndjson", "application/ndjson", "application/jsonl":
		return "download.jsonl"
	case "application/parquet", "application/vnd.apache.parquet":
		return "download.parquet"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "download.xlsx"
	default:
		return ""
	}
}
