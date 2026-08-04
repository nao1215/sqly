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

// schemeHTTP and schemeHTTPS are the two schemes sqly downloads over. They are
// named because three separate checks — is this a URL, may a redirect go here,
// is this an unfetchable scheme — have to agree on the answer.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

const (
	remoteSupportedFormatsHelp = "csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed"
	remoteCSVFilename          = "download.csv"
	remoteJSONContentType      = "application/json"
	remoteJSONFilename         = "download.json"
)

// A timeout bounds how long a remote server may take. These bound what it may
// cost, which is the other half and the one the server chooses: a URL is the one
// input to sqly that nobody local vouched for.
//
// The numbers are set where an ordinary dataset is nowhere near them and a
// runaway response is stopped before it matters. A 2 GiB CSV is far past what
// belongs in an in-memory SQLite database — the import would exhaust memory long
// before the disk — so the download limit is not what makes such a file fail, it
// is only what keeps a server from filling the disk on the way there. Both are
// documented at https://nao1215.github.io/sqly/formats/ so a user who hits one
// knows what they hit.
//
// Why not make them flags: a limit that is routinely raised is not protecting
// anything, and every input sqly reads locally is already unbounded. The remote
// case is different because the size is not the user's to know in advance.
const (
	// maxRemoteDownloadBytes caps what one URL may transfer.
	maxRemoteDownloadBytes int64 = 2 << 30 // 2 GiB
	// maxRemoteRedirects caps the redirect chain. Go's default is 10 with an
	// error that does not say sqly chose it; a smaller number and sqly's own
	// message make the refusal explainable.
	maxRemoteRedirects = 5
)

// downloadLimit returns the byte cap for one download. Tests lower it so the
// limit can be exercised without moving two gigabytes through the disk; nothing
// outside tests sets the field, so a run always uses the constant above.
func (s *Shell) downloadLimit() int64 {
	if s.maxDownloadBytes > 0 {
		return s.maxDownloadBytes
	}
	return maxRemoteDownloadBytes
}

// newRemoteClient builds the HTTP client sqly downloads inputs with. The
// policies it carries are part of the download contract, so they live with the
// rest of it rather than inline in the shell constructor — and a test that
// swaps in a server's transport keeps them.
func newRemoteClient() *http.Client {
	return &http.Client{
		// Bound the full request/response body read so a server that stalls
		// mid-download cannot hang the CLI indefinitely.
		Timeout: 15 * time.Minute,
		Transport: &http.Transport{
			// A custom Transport replaces http.DefaultTransport wholesale, which
			// is where proxy support lives. Without this a user behind a corporate
			// proxy cannot fetch a URL at all, and the failure looks like the host
			// being unreachable rather than like a setting sqly ignored.
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		CheckRedirect: checkRemoteRedirect,
	}
}

// checkRemoteRedirect decides whether to follow a redirect. It bounds the chain
// and, more importantly, keeps it inside HTTP.
//
// A redirect changes the URL sqly fetches to one the user never wrote. Go's
// default client refuses a redirect to a non-HTTP scheme in the sense that its
// transport has no handler for one, but the resulting error describes a missing
// protocol rather than a server having tried to redirect sqly at
// file:///etc/passwd. Saying that plainly is worth the four lines.
func checkRemoteRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRemoteRedirects {
		return fmt.Errorf("stopped after %d redirects; the chain starting at %s does not settle",
			maxRemoteRedirects, via[0].URL)
	}
	switch strings.ToLower(req.URL.Scheme) {
	case schemeHTTP, schemeHTTPS:
		return nil
	default:
		return fmt.Errorf("refused a redirect to the %q scheme (%s); sqly downloads over http and https only",
			req.URL.Scheme, req.URL)
	}
}

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
	case schemeHTTP, schemeHTTPS:
		return true
	default:
		return false
	}
}

// unfetchableURLScheme returns the scheme of an input written as a URL that sqly
// cannot download, and "" for anything else. Only "<scheme>://" counts, and the
// scheme must be at least two characters, so a Windows drive path ("C:/data") and
// a local file name that merely contains a colon ("weird:name.csv") stay local
// paths. Recognizing these is what lets the import error say the scheme is
// unsupported instead of reporting the URL as a missing file.
func unfetchableURLScheme(raw string) string {
	if isRemoteURL(raw) {
		return ""
	}
	sep := strings.Index(raw, "://")
	if sep < 2 {
		return ""
	}
	scheme := raw[:sep]
	for i, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return ""
		}
	}
	return strings.ToLower(scheme)
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

// remoteFilenameHint returns the filename-like hint from the URL path, which is
// what a check that needs the format has to go on before the download.
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

	// A declared size over the limit is refused before a byte of the body is
	// read. ContentLength is -1 for a chunked response, which is why this is the
	// cheap path and not the check: the one that has to hold is below, while the
	// body is being copied.
	limit := s.downloadLimit()
	if resp.ContentLength > limit {
		return "", nil, fmt.Errorf("download %s: too large: the server declared %d bytes and the limit is %d",
			rawURL, resp.ContentLength, limit)
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

	// Read one byte past the limit rather than exactly the limit, so a body that
	// is precisely at it is still distinguishable from one that ran over.
	written, copyErr := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	closeErr := f.Close()
	if copyErr != nil {
		cleanup()
		return "", nil, fmt.Errorf("download %s: %w", rawURL, copyErr)
	}
	if written > limit {
		// The staged file goes with the refusal. A partial download left on disk
		// turns a rejected import into a slow leak across repeated runs, and half a
		// CSV is worse than none: it parses.
		cleanup()
		return "", nil, fmt.Errorf("download %s: too large: the response exceeded the %d byte limit",
			rawURL, limit)
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
//
// The URL it reads is the one the response came from, not the one the user
// typed. A dataset published behind a redirect — a short link, a "latest"
// alias — would otherwise be named after the alias, so `sqly host/latest`
// landing on `sales.csv` gave a table named after the Content-Type fallback
// rather than after the file that actually arrived.
func (s *Shell) remoteDownloadFilename(rawURL string, resp *http.Response) (string, error) {
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	var first string
	for _, candidate := range []string{
		contentDispositionFilename(resp.Header.Get("Content-Disposition")),
		remoteFilenameHint(finalURL),
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
		return remoteCSVFilename
	case "text/tab-separated-values":
		return "download.tsv"
	case "text/ltsv", "application/ltsv":
		return "download.ltsv"
	case remoteJSONContentType, "text/json":
		return remoteJSONFilename
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
