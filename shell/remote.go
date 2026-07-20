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
	"time"
)

const remoteDownloadReportStep int64 = 4 << 20 // 4 MiB

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

	filename, err := remoteDownloadFilename(rawURL, resp)
	if err != nil {
		return "", nil, err
	}
	if !s.usecases.importer.IsSupportedFile(filename) {
		return "", nil, fmt.Errorf("unsupported remote file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed)", filename)
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

	progress := newDownloadProgressWriter(s.importStatusWriter(), rawURL, filename, resp.ContentLength)
	_, copyErr := io.Copy(io.MultiWriter(f, progress), resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("download %s: %w", rawURL, copyErr)
	}
	if closeErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("close staging file for %s: %w", rawURL, closeErr)
	}
	progress.finish()
	return localPath, cleanup, nil
}

// remoteDownloadFilename chooses a filename for a downloaded input: first a
// Content-Disposition filename when present, then the URL path, then a
// Content-Type-derived extension. The name must end in a sqly-supported format.
func remoteDownloadFilename(rawURL string, resp *http.Response) (string, error) {
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
		if supportedRemoteFilename(candidate) {
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

func supportedRemoteFilename(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".gz", ".bz2", ".xz", ".zst", ".z", ".snappy", ".s2", ".lz4"} {
		if before, ok := strings.CutSuffix(lower, ext); ok {
			lower = before
			break
		}
	}
	for _, ext := range []string{".csv", ".tsv", ".ltsv", ".parquet", ".xlsx", ".json", ".jsonl", ".ach", ".fed"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
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

type downloadProgressWriter struct {
	out         io.Writer
	url         string
	filename    string
	total       int64
	written     int64
	nextReport  int64
	lastReport  time.Time
	reportedAny bool
}

func newDownloadProgressWriter(out io.Writer, rawURL, filename string, total int64) *downloadProgressWriter {
	fmt.Fprintf(out, "Downloading %s\n", rawURL)
	return &downloadProgressWriter{
		out:        out,
		url:        rawURL,
		filename:   filename,
		total:      total,
		nextReport: remoteDownloadReportStep,
		lastReport: time.Now(),
	}
}

func (w *downloadProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.written += int64(n)
	if w.written >= w.nextReport || time.Since(w.lastReport) >= time.Second {
		w.report()
		w.nextReport = w.written + remoteDownloadReportStep
		w.lastReport = time.Now()
	}
	return n, nil
}

func (w *downloadProgressWriter) report() {
	w.reportedAny = true
	if w.total > 0 {
		pct := float64(w.written) * 100 / float64(w.total)
		fmt.Fprintf(w.out, "Downloading %s: %s / %s (%.0f%%)\n", w.url, humanBytes(w.written), humanBytes(w.total), pct)
		return
	}
	fmt.Fprintf(w.out, "Downloading %s: %s\n", w.url, humanBytes(w.written))
}

func (w *downloadProgressWriter) finish() {
	if w.reportedAny {
		fmt.Fprintf(w.out, "Downloaded %s -> %s (%s)\n", w.url, w.filename, humanBytes(w.written))
		return
	}
	if w.total > 0 {
		fmt.Fprintf(w.out, "Downloaded %s -> %s (%s)\n", w.url, w.filename, humanBytes(w.total))
		return
	}
	fmt.Fprintf(w.out, "Downloaded %s -> %s (%s)\n", w.url, w.filename, humanBytes(w.written))
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
