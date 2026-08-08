package shell

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"golang.org/x/text/encoding/japanese"
)

func TestImportPathAndDecoderHelpers(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
		want bool
	}{
		{name: "csv", path: "data.csv", want: true},
		{name: "compressed tsv", path: "data.tsv.gz", want: true},
		{name: "jsonl", path: "data.jsonl.gz", want: true},
		{name: "unsupported", path: "data.xlsx", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isTextImportPath(test.path); got != test.want {
				t.Errorf("isTextImportPath(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}

	for _, encoding := range []model.TextEncoding{
		model.TextEncodingUTF8,
		model.TextEncodingShiftJIS,
		model.TextEncodingEUCJP,
		model.TextEncodingISO2022JP,
		model.TextEncodingUTF16LE,
		model.TextEncodingUTF16BE,
		model.TextEncoding("unknown"),
	} {
		t.Run(encoding.String(), func(t *testing.T) {
			if newImportDecoder(encoding) == nil {
				t.Fatalf("newImportDecoder(%q) returned nil", encoding)
			}
		})
	}
}

func TestRemoteURLHelpers(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
		want bool
	}{
		{name: "http", raw: "http://example.test/data.csv", want: true},
		{name: "https", raw: "https://example.test/data.csv", want: true},
		{name: "missing host", raw: "https:///data.csv", want: false},
		{name: "local path", raw: "/tmp/data.csv", want: false},
		{name: "unsupported scheme", raw: "ftp://example.test/data.csv", want: false},
		{name: "invalid URL", raw: "http://[", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isRemoteURL(test.raw); got != test.want {
				t.Errorf("isRemoteURL(%q) = %v, want %v", test.raw, got, test.want)
			}
		})
	}

	for _, test := range []struct {
		raw  string
		want string
	}{
		{raw: "ftp://example.test/data.csv", want: "ftp"},
		{raw: "SSH+git://example.test/repo", want: "ssh+git"},
		{raw: "C:/data.csv", want: ""},
		{raw: "weird:name.csv", want: ""},
		{raw: "bad_scheme://example.test/data", want: ""},
	} {
		if got := unfetchableURLScheme(test.raw); got != test.want {
			t.Errorf("unfetchableURLScheme(%q) = %q, want %q", test.raw, got, test.want)
		}
	}

	if got := normalizeRemoteURL("https://example.test/data.csv#fragment"); got != "https://example.test/data.csv" {
		t.Errorf("normalizeRemoteURL removed fragment incorrectly: %q", got)
	}
	if got := normalizeRemoteURL("http://["); got != "http://[" {
		t.Errorf("normalizeRemoteURL invalid URL = %q", got)
	}
	if !sameSourceLocation("https://example.test/data.csv#one", "https://example.test/data.csv#two") {
		t.Error("sameSourceLocation should ignore remote URL fragments")
	}

	for _, test := range []struct {
		raw      string
		filename string
	}{
		{raw: "https://example.test/files/data.csv", filename: "data.csv"},
		{raw: "https://example.test/files/REPORT.XLSX", filename: "REPORT.XLSX"},
		{raw: "https://example.test/", filename: ""},
		{raw: "http://[", filename: ""},
	} {
		if got := remoteFilenameHint(test.raw); got != test.filename {
			t.Errorf("remoteFilenameHint(%q) = %q, want %q", test.raw, got, test.filename)
		}
	}
}

func TestRemoteFilenameHelpers(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, test := range []struct {
		name   string
		url    string
		header string
		want   string
	}{
		{name: "content type fallback", url: "https://example.test/download", header: "text/csv", want: "download.csv"},
		{name: "unsupported URL falls back to content type", url: "https://example.test/download.unsupported", header: "application/json", want: "download.json"},
		// A URL that names a supported file names the table, and no header may
		// take that over: a query written against sales.csv has to find "sales".
		{name: "a named URL beats a content type that disagrees", url: "https://example.test/sales.csv", header: "application/json", want: "sales.csv"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{"Content-Type": []string{test.header}}}
			got, err := s.remoteDownloadFilename(test.url, resp)
			if err != nil || got != test.want {
				t.Fatalf("remoteDownloadFilename() = %q, %v, want %q", got, err, test.want)
			}
		})
	}
	t.Run("a server cannot rename a table the URL already named", func(t *testing.T) {
		// Content-Disposition and a redirect are both the server describing what
		// it sent. Either used to win over the URL, so "SELECT * FROM sales"
		// against sales.csv became "no such table: sales" — and which name to use
		// instead was only discoverable by inspecting the download.
		redirected, err := url.Parse("https://example.test/somewhere-else.csv")
		if err != nil {
			t.Fatal(err)
		}
		resp := &http.Response{
			Header:  http.Header{"Content-Disposition": []string{`attachment; filename="renamed.csv"`}},
			Request: &http.Request{URL: redirected},
		}
		got, err := s.remoteDownloadFilename("https://example.test/sales.csv", resp)
		if err != nil {
			t.Fatalf("remoteDownloadFilename: %v", err)
		}
		if got != "sales.csv" {
			t.Errorf("remoteDownloadFilename = %q, want %q: the URL names the table", got, "sales.csv")
		}
	})

	t.Run("a URL with no name still takes the server's", func(t *testing.T) {
		// The other half of the rule: where the URL carries no filename, the
		// server's is all there is, and it is still used.
		resp := &http.Response{
			Header: http.Header{"Content-Disposition": []string{`attachment; filename="report.csv"`}},
		}
		got, err := s.remoteDownloadFilename("https://example.test/download", resp)
		if err != nil {
			t.Fatalf("remoteDownloadFilename: %v", err)
		}
		if got != "report.csv" {
			t.Errorf("remoteDownloadFilename = %q, want %q", got, "report.csv")
		}
	})

	if _, err := s.remoteDownloadFilename("https://example.test/", &http.Response{}); err == nil {
		t.Error("remoteDownloadFilename with no usable hint returned nil error")
	}

	if got := contentDispositionFilename(`attachment; filename="report.csv"`); got != "report.csv" {
		t.Errorf("contentDispositionFilename = %q", got)
	}
	if got := contentDispositionFilename(`attachment; filename*=UTF-8''report.tsv`); got != "report.tsv" {
		t.Errorf("contentDispositionFilename filename* = %q", got)
	}
	for _, header := range []string{"", "not a media type", "attachment"} {
		if got := contentDispositionFilename(header); got != "" {
			t.Errorf("contentDispositionFilename(%q) = %q, want empty", header, got)
		}
	}

	for _, test := range []struct {
		header string
		want   string
	}{
		{header: "text/csv; charset=utf-8", want: "download.csv"},
		{header: "application/csv", want: "download.csv"},
		{header: "text/tab-separated-values", want: "download.tsv"},
		{header: "text/ltsv", want: "download.ltsv"},
		{header: "application/json", want: "download.json"},
		{header: "application/x-ndjson", want: "download.jsonl"},
		{header: "application/parquet", want: "download.parquet"},
		{header: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", want: "download.xlsx"},
		{header: "application/octet-stream", want: ""},
		{header: "not a media type", want: ""},
	} {
		if got := filenameFromContentType(test.header); got != test.want {
			t.Errorf("filenameFromContentType(%q) = %q, want %q", test.header, got, test.want)
		}
	}
}

func TestPrepareImportLoadPathDecodesTextInput(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly", "--encoding", "shift-jis"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	path := filepath.Join(t.TempDir(), "people.csv")
	// Use the shared test helper so this test exercises the real staging path.
	encodedBytes := mustEncodeString(t, japanese.ShiftJIS.NewEncoder(), "id,name\n1,太郎\n")
	if err := os.WriteFile(path, encodedBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	if got, cleanupStaged, err := s.prepareImportLoadPath(path); err != nil || got == "" || cleanupStaged == nil {
		t.Fatalf("prepareImportLoadPath() = %q, %v, cleanup=%v", got, err, cleanupStaged != nil)
	} else {
		defer cleanupStaged()
		decoded, err := os.ReadFile(got) //nolint:gosec // got is a staging path created by sqly
		if err != nil {
			t.Fatal(err)
		}
		if string(decoded) != "id,name\n1,太郎\n" {
			t.Errorf("staged content = %q, want decoded UTF-8 CSV", decoded)
		}
	}

	if got, cleanupStaged, err := s.prepareImportLoadPath("data.xlsx"); err != nil || got != "data.xlsx" || cleanupStaged != nil {
		t.Errorf("non-text input = %q, cleanup=%v, err=%v", got, cleanupStaged != nil, err)
	}

	utf8Shell, utf8Cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer utf8Cleanup()
	if got, cleanupStaged, err := utf8Shell.prepareImportLoadPath(path); err != nil || got != path || cleanupStaged != nil {
		t.Errorf("UTF-8 input = %q, cleanup=%v, err=%v", got, cleanupStaged != nil, err)
	}
	if _, _, err := s.prepareImportLoadPath(filepath.Join(t.TempDir(), "missing.csv")); err == nil {
		t.Error("prepareImportLoadPath(missing file) returned nil error")
	}
}

// TestPrepareImportLoadPathRefusesBytesTheEncodingCannotDecode covers the half of
// the encoding contract the flag used to skip. A text input that is not UTF-8 is
// refused, but naming a legacy encoding turned the same corruption back on: the
// x/text decoders substitute U+FFFD for a byte the encoding has no meaning for,
// so the import succeeded and the table held replacement characters.
func TestPrepareImportLoadPathRefusesBytesTheEncodingCannotDecode(t *testing.T) {
	for _, test := range []struct {
		name     string
		encoding string
		content  []byte
	}{
		// 0xFF is not a legal Shift-JIS lead byte, and EUC-JP has no meaning for
		// it either.
		{name: "shift-jis lead byte", encoding: "shift-jis", content: []byte("a\n\xff\xfe\x01\n")},
		{name: "euc-jp lead byte", encoding: "euc-jp", content: []byte("a\n\xff\xff\n")},
		// ISO-2022-JP is 7-bit, so a byte with the high bit set is outside it
		// whatever shift state the stream is in.
		{name: "iso-2022-jp high bit", encoding: "iso-2022-jp", content: []byte("a\n\xff\xfe\n")},
		// A UTF-16 code unit is two bytes, so an odd length means the last one was
		// cut in half.
		{name: "utf-16le odd length", encoding: "utf-16le", content: []byte("a\x00\n\x00\x41")},
		// A high surrogate with nothing after it is not a character.
		{name: "utf-16le unpaired surrogate", encoding: "utf-16le", content: []byte("a\x00\n\x00\x00\xd8\x41\x00")},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, cleanup, err := newShell(t, []string{"sqly", "--encoding", test.encoding})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			path := filepath.Join(t.TempDir(), "bad.csv")
			if err := os.WriteFile(path, test.content, 0o600); err != nil {
				t.Fatal(err)
			}

			staged, cleanupStaged, err := s.prepareImportLoadPath(path)
			if cleanupStaged != nil {
				cleanupStaged()
			}
			if err == nil {
				t.Fatalf("prepareImportLoadPath() = %q, nil error; want a refusal for bytes %s cannot decode", staged, test.encoding)
			}
			if !strings.Contains(err.Error(), test.encoding) {
				t.Errorf("error = %v, want it to name the declared encoding %q", err, test.encoding)
			}
		})
	}
}

// TestPrepareImportLoadPathKeepsAGenuineReplacementCharacter is the other side of
// the check: U+FFFD is a real character in UTF-16, so a file that holds one is
// data rather than a decode that went wrong.
func TestPrepareImportLoadPathKeepsAGenuineReplacementCharacter(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly", "--encoding", "utf-16le"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	path := filepath.Join(t.TempDir(), "fffd.csv")
	// "a\n" then U+FFFD, each as a little-endian code unit.
	if err := os.WriteFile(path, []byte("a\x00\n\x00\xfd\xff"), 0o600); err != nil {
		t.Fatal(err)
	}

	staged, cleanupStaged, err := s.prepareImportLoadPath(path)
	if err != nil {
		t.Fatalf("prepareImportLoadPath() error = %v, want a file holding U+FFFD to load", err)
	}
	defer cleanupStaged()
	decoded, err := os.ReadFile(staged) //nolint:gosec // staged is a path sqly created
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decoded), "�") {
		t.Errorf("staged content = %q, want the replacement character the file holds", decoded)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadRemoteInputErrors(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for _, test := range []struct {
		name    string
		client  *http.Client
		url     string
		wantErr string
	}{
		{
			name: "invalid request URL",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, io.ErrUnexpectedEOF
			})},
			url:     "http://[",
			wantErr: "build download request",
		},
		{
			name: "transport error",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, io.ErrUnexpectedEOF
			})},
			url:     "https://example.test/data.csv",
			wantErr: "download https://example.test/data.csv",
		},
		{
			name: "HTTP status",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("missing")), Header: make(http.Header)}, nil
			})},
			url:     "https://example.test/data.csv",
			wantErr: "unexpected HTTP status",
		},
		{
			name: "unsupported filename",
			client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("data")), Header: make(http.Header)}, nil
			})},
			url:     "https://example.test/data.unsupported",
			wantErr: "unsupported remote file format",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			s.httpClient = test.client
			_, _, err := s.downloadRemoteInput(context.Background(), test.url)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("downloadRemoteInput() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

// TestLocalPathFromFileURL covers the path a file: URL is told to be pasted
// instead of. What it returns is meant to be typed back on a command line, so a
// percent sequence left in it, or an authority that names another machine, would
// send the reader to a file that is not there.
func TestLocalPathFromFileURL(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{name: "a rooted path", raw: "file:///etc/hostname", want: "/etc/hostname", wantOK: true},
		{name: "an escaped space is decoded", raw: "file:///tmp/my%20file.csv", want: "/tmp/my file.csv", wantOK: true},
		{name: "a percent in a name survives escaping", raw: "file:///tmp/100%25.csv", want: "/tmp/100%.csv", wantOK: true},
		{name: "a windows drive path keeps its drive", raw: "file:///C:/data.csv", want: "C:/data.csv", wantOK: true},
		{name: "the scheme is matched case-insensitively", raw: "FILE:///etc/hostname", want: "/etc/hostname", wantOK: true},
		// Not local: the authority names a machine sqly cannot read from.
		{name: "a host makes it someone else's file", raw: "file://server/share/data.csv", wantOK: false},
		// Not a path anyone can paste, so the general scheme message is better.
		{name: "invalid escaping is not a path", raw: "file:///tmp/my%2Gfile.csv", wantOK: false},
		{name: "another scheme is not a file URL", raw: "ftp://example.test/data.csv", wantOK: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := localPathFromFileURL(test.raw)
			if ok != test.wantOK {
				t.Fatalf("localPathFromFileURL(%q) ok = %v, want %v", test.raw, ok, test.wantOK)
			}
			if got != test.want {
				t.Errorf("localPathFromFileURL(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}
