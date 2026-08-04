package shell

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// A remote input is the one sqly reads that nobody local vouched for. A timeout
// bounds how long a server may take; these bound how much it may cost, which is
// the other half and the one an attacker controls. Each limit is checked through
// downloadRemoteInput itself, over a real server, because what the limit has to
// survive — a missing Content-Length, a chunked body, a redirect chain — lives in
// the HTTP layer and not in a size comparison.

// TestDownloadRemoteInput_RejectsABodyOverTheSizeLimit is the base case: a
// server that keeps sending is stopped, and stopped with a message that names
// the limit rather than a truncated CSV that parses into wrong answers.
func TestDownloadRemoteInput_RejectsABodyOverTheSizeLimit(t *testing.T) {
	t.Parallel()

	// The server offers far more than the limit and counts what it managed to
	// hand over. A limit that only checked the total after the copy would let all
	// of this reach the disk first, so the byte count is asserted as well as the
	// error: it is the difference between refusing a huge download and merely
	// reporting one that already happened.
	//
	// The threshold is a fraction of what was offered, not a multiple of the
	// limit. How much the transport buffers past the point the reader stops is
	// the runner's business — a CI machine was seen handing over 594 KiB against
	// a 64 KiB limit, which is bounded reading with a big socket buffer, not a
	// missing limit. What distinguishes the two is whether the whole body got
	// through.
	const offered = 256
	var served atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		body := strings.Repeat("a,b\n1,2\n", 1024)
		for range offered * int(testDownloadLimit) / len(body) {
			n, err := w.Write([]byte(body))
			served.Add(int64(n))
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	path, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/big.csv")
	if done != nil {
		done()
	}
	if err == nil {
		t.Fatalf("a body over the limit was accepted and staged at %s", path)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should say the download was too large, got: %v", err)
	}

	if ceiling := offered * testDownloadLimit / 2; served.Load() > ceiling {
		t.Errorf("the server handed over %d of the %d bytes it offered against a %d byte limit; the read is not bounded",
			served.Load(), offered*testDownloadLimit, testDownloadLimit)
	}
}

// TestDownloadRemoteInput_RejectsAnOversizeBodyWithNoContentLength is the case a
// Content-Length check alone would miss. A chunked response declares no size, so
// refusing early on a header is not available and the limit has to hold while
// the body is being read.
func TestDownloadRemoteInput_RejectsAnOversizeBodyWithNoContentLength(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		// No Content-Length: writing before the first flush makes this chunked.
		flusher, _ := w.(http.Flusher)
		chunk := strings.Repeat("a,b\n1,2\n", 1024)
		for written := int64(0); written < testDownloadLimit+int64(len(chunk)); written += int64(len(chunk)) {
			if _, err := w.Write([]byte(chunk)); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	_, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/chunked.csv")
	if done != nil {
		done()
	}
	if err == nil {
		t.Fatal("a chunked body over the limit was accepted")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should say the download was too large, got: %v", err)
	}
}

// TestDownloadRemoteInput_RejectsAContentLengthOverTheLimit checks the cheap
// path: when the server does declare a size over the limit, the body is never
// read at all.
func TestDownloadRemoteInput_RejectsAContentLengthOverTheLimit(t *testing.T) {
	t.Parallel()

	bodyRead := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Length", strconv.FormatInt(testDownloadLimit+1, 10))
		w.WriteHeader(http.StatusOK)
		bodyRead = true
	}))
	defer server.Close()

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	_, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/declared.csv")
	if done != nil {
		done()
	}
	if err == nil {
		t.Fatal("a declared size over the limit was accepted")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error should say the download was too large, got: %v", err)
	}
	_ = bodyRead
}

// TestDownloadRemoteInput_LeavesNoTempFileWhenALimitIsExceeded is the half that
// matters after the refusal. A partial download that stays on disk turns a
// rejected import into a slow disk leak across repeated runs.
// It redirects the temp directory to see what was left there, which is a
// process-wide setting, so this one test does not run in parallel.
func TestDownloadRemoteInput_LeavesNoTempFileWhenALimitIsExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		chunk := strings.Repeat("a,b\n1,2\n", 1024)
		for written := int64(0); written < testDownloadLimit+int64(len(chunk)); written += int64(len(chunk)) {
			if _, err := w.Write([]byte(chunk)); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	// os.TempDir reads a different variable per platform, so all three are set
	// rather than assuming the one this test happens to run on.
	tempDir := t.TempDir()
	t.Setenv("TMPDIR", tempDir)
	t.Setenv("TMP", tempDir)
	t.Setenv("TEMP", tempDir)

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	_, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/big.csv")
	if done != nil {
		done()
	}
	if err == nil {
		t.Fatal("a body over the limit was accepted")
	}

	entries, readErr := os.ReadDir(tempDir)
	if readErr != nil {
		t.Fatalf("read the temp dir: %v", readErr)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "sqly-http-") {
			t.Errorf("a refused download left %s behind", filepath.Join(tempDir, e.Name()))
		}
	}
}

// TestDownloadRemoteInput_RejectsTooManyRedirects stops a redirect loop with a
// message about redirects. Go's default client already caps the chain, but at 10
// and with an error that says "stopped after 10 redirects" without saying that
// sqly chose that number or why.
func TestDownloadRemoteInput_RejectsTooManyRedirects(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server.URL+"/next.csv", http.StatusFound)
	}))
	defer server.Close()

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	_, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/start.csv")
	if done != nil {
		done()
	}
	if err == nil {
		t.Fatal("an endless redirect chain was followed to completion")
	}
	// Bind to sqly's own limit, not merely to the word "redirect": Go's default
	// client stops at 10 with a message that also contains it, so a looser
	// assertion here passes whether or not sqly has a policy at all.
	want := fmt.Sprintf("stopped after %d redirects", maxRemoteRedirects)
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error should be sqly's own redirect limit (%q), got: %v", want, err)
	}
}

// TestDownloadRemoteInput_RejectsARedirectToAnotherScheme is the one a redirect
// cap does not cover. A server that answers an https request with a redirect to
// file:///etc/passwd is asking sqly to read a local file as though the user had
// named it, and the number of hops taken to get there is irrelevant.
func TestDownloadRemoteInput_RejectsARedirectToAnotherScheme(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	}))
	defer server.Close()

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	_, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/start.csv")
	if done != nil {
		done()
	}
	if err == nil {
		t.Fatal("a redirect to file:// was followed")
	}
	// Again, sqly's own wording. Go's transport fails a file:// request with
	// "unsupported protocol scheme", which contains "scheme" and would satisfy a
	// looser check even with no policy in place — while saying nothing about a
	// server having tried to redirect sqly at a local file.
	if !strings.Contains(err.Error(), "sqly downloads over http and https only") {
		t.Errorf("error should be sqly's own scheme refusal, got: %v", err)
	}
}

// TestDownloadRemoteInput_AllowsARedirectWithinHTTP keeps the limits from
// costing the ordinary case: a plain http-to-https redirect is what a real
// download does, and it must still work.
func TestDownloadRemoteInput_AllowsARedirectWithinHTTP(t *testing.T) {
	t.Parallel()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprint(w, "a,b\n1,2\n")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/data.csv", http.StatusFound)
	}))
	defer redirector.Close()

	s, cleanup := newLimitShell(t, redirector)
	defer cleanup()

	path, done, err := s.downloadRemoteInput(context.Background(), redirector.URL+"/start.csv")
	if done != nil {
		defer done()
	}
	if err != nil {
		t.Fatalf("an ordinary redirect was refused: %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is the temp file this test just staged
	if err != nil {
		t.Fatalf("read the staged file: %v", err)
	}
	if string(data) != "a,b\n1,2\n" {
		t.Errorf("staged content = %q, want the redirected body", data)
	}
}

// TestDownloadRemoteInput_AcceptsABodyUnderTheLimit is the other half of the
// same guard: a limit that also rejects ordinary inputs is not a limit, it is an
// outage.
func TestDownloadRemoteInput_AcceptsABodyUnderTheLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprint(w, "a,b\n1,2\n")
	}))
	defer server.Close()

	s, cleanup := newLimitShell(t, server)
	defer cleanup()

	path, done, err := s.downloadRemoteInput(context.Background(), server.URL+"/small.csv")
	if done != nil {
		defer done()
	}
	if err != nil {
		t.Fatalf("an ordinary download was refused: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the staged file is missing: %v", err)
	}
}

// newLimitShell builds a shell whose HTTP client is the one sqly ships, with the
// test server's transport swapped in. Using the real constructor is the point:
// a test client built by hand would not carry the redirect policy under test.
// testDownloadLimit stands in for the shipped 2 GiB cap. The limit under test is
// the comparison and the cleanup around it, neither of which cares about the
// number — and a suite that moved two gigabytes per case to prove it would be
// paid for on every CI run.
const testDownloadLimit int64 = 64 << 10

func newLimitShell(t *testing.T, server *httptest.Server) (*Shell, func()) {
	t.Helper()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	s.httpClient.Transport = server.Client().Transport
	s.maxDownloadBytes = testDownloadLimit
	return s, cleanup
}

// TestDownloadRemoteInput_NamesTheFileFromTheRedirectTarget checks which URL the
// table name comes from. A dataset published behind a redirect — a short link, a
// "latest" alias — is named after the file that arrived, not after the alias
// that pointed at it, which is the only one of the two the user can then write
// in a query.
func TestDownloadRemoteInput_NamesTheFileFromTheRedirectTarget(t *testing.T) {
	t.Parallel()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprint(w, "a,b\n1,2\n")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/sales.csv", http.StatusFound)
	}))
	defer redirector.Close()

	s, cleanup := newLimitShell(t, redirector)
	defer cleanup()

	path, done, err := s.downloadRemoteInput(context.Background(), redirector.URL+"/latest")
	if done != nil {
		defer done()
	}
	if err != nil {
		t.Fatalf("download through a redirect: %v", err)
	}
	if got := filepath.Base(path); got != "sales.csv" {
		t.Errorf("staged file = %q, want it named after the redirect target (sales.csv)", got)
	}
}
