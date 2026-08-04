//go:build smoke

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The remote-input contract, driven against the real binary over a real HTTP
// server.
//
// The atago specs cover this too, but they serve fixtures with `python3 -m
// http.server` behind `sh -c`, so they skip Windows and can only serve files —
// there is no way to write a response that stops halfway, or a redirect chain
// six links long. httptest gives both, and being pure Go it runs on Linux,
// macOS, and Windows alike, which is where the download path had no binary-level
// coverage at all.
//
// What is checked here is what a user sees: the exit class, whether stdout stays
// clean, and whether the temp directory a download staged is gone afterwards.
// The 2 GiB body limit is not among them — generating that much data in CI to
// watch a counter trip would be wasteful, and the limit's arithmetic is a unit
// test's job. Documented at website/content/formats.md#remote-inputs.

const remoteCSV = "user_name,identifier\nbooker12,1\njenkins46,2\n"

// TestRemoteImportDownloadsAndQueries is the baseline the rest are read against:
// an ordinary URL imports and answers.
func TestRemoteImportDownloadsAndQueries(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(remoteCSV))
	}))
	defer server.Close()

	stdout, stderr, code := run(t, "",
		"--output-format", "csv",
		"--sql", "SELECT user_name FROM user ORDER BY identifier LIMIT 1",
		server.URL+"/user.csv")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "booker12") {
		t.Errorf("stdout = %q, want the first row", stdout)
	}
}

// TestRemoteImportFollowsARedirectChain checks the chain is followed at all, and
// that the table is named after where it ended up rather than what was typed —
// which is the behavior a short link or a "latest" alias relies on.
func TestRemoteImportFollowsARedirectChain(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/v2", http.StatusFound)
	})
	mux.HandleFunc("/v2", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user.csv", http.StatusFound)
	})
	mux.HandleFunc("/user.csv", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(remoteCSV))
	})

	stdout, stderr, code := run(t, "",
		"--output-format", "csv",
		"--sql", "SELECT COUNT(*) AS n FROM user",
		server.URL+"/latest")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("stdout = %q, want the row count of the redirect target", stdout)
	}
}

// TestRemoteImportRefusesAnEndlessRedirectChain pins the bound. Without it a
// server can keep a client going forever, which is a hang rather than an error.
func TestRemoteImportRefusesAnEndlessRedirectChain(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Each hop points at the next, past the limit, so the chain never resolves.
	for i := range 12 {
		hop := i
		mux.HandleFunc(fmt.Sprintf("/hop%d", hop), func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, fmt.Sprintf("/hop%d", hop+1), http.StatusFound)
		})
	}

	stdout, _, code := run(t, "", "--sql", "SELECT 1", server.URL+"/hop0")
	if code != 3 {
		t.Errorf("exit = %d, want 3 (an input that could not be read)", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want nothing; a failed import must not print a result", stdout)
	}
}

// TestRemoteImportRefusesARedirectToAnotherScheme is the one bound whose absence
// would be a security problem rather than a resource one: an http URL that ends
// up reading file:// is not the request anyone made.
func TestRemoteImportRefusesARedirectToAnotherScheme(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/bounce", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	})

	_, stderr, code := run(t, "", "--sql", "SELECT 1", server.URL+"/bounce")
	if code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr, "file") {
		t.Errorf("stderr = %q, want the refused scheme named", stderr)
	}
}

// TestRemoteImportReadsAChunkedResponse covers the response with no declared
// size. The cheap size check has nothing to look at there, so this is the path
// where the limit has to hold while the body is being read; what is asserted is
// that an ordinary chunked response still imports.
func TestRemoteImportReadsAChunkedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// No Content-Length, flushed in pieces: net/http then uses chunked
		// transfer encoding.
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for _, line := range strings.SplitAfter(remoteCSV, "\n") {
			_, _ = w.Write([]byte(line))
			flusher.Flush()
		}
	}))
	defer server.Close()

	stdout, stderr, code := run(t, "",
		"--output-format", "csv",
		"--sql", "SELECT COUNT(*) AS n FROM user",
		server.URL+"/user.csv")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("stdout = %q, want both rows", stdout)
	}
}

// TestRemoteImportFailsOnATruncatedResponse is the case a length check cannot
// catch: the server promises more than it sends and then hangs up. Reporting
// that as a successful import of a short file would be silent data loss.
func TestRemoteImportFailsOnATruncatedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4096")
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(remoteCSV))
		// Hijack and close so the client sees the connection drop mid-body.
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	stdout, _, code := run(t, "", "--sql", "SELECT 1", server.URL+"/user.csv")
	if code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
}

// TestRemoteImportRefusesAnOversizedDeclaredBody uses the header alone, so no
// large body is ever produced: the declared size is checked before a byte is
// read, and that check is the one CI can afford to exercise.
func TestRemoteImportRefusesAnOversizedDeclaredBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Three gibibytes, declared and never sent.
		w.Header().Set("Content-Length", "3221225472")
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, stderr, code := run(t, "", "--sql", "SELECT 1", server.URL+"/user.csv")
	if code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr, "too large") {
		t.Errorf("stderr = %q, want the size refusal", stderr)
	}
}

// TestRemoteImportRejectsAnUnsupportedScheme keeps the "only http and https"
// promise checked from the outside, including on Windows, where such an argument
// is rejected by the filesystem for an unrelated reason and the message has to
// still name the scheme.
func TestRemoteImportRejectsAnUnsupportedScheme(t *testing.T) {
	t.Parallel()

	_, stderr, code := run(t, "", "--sql", "SELECT 1", "s3://bucket/user.csv")
	if code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
	if !strings.Contains(stderr, "s3") {
		t.Errorf("stderr = %q, want the scheme named", stderr)
	}
}

// TestRemoteImportOfAWorkbookAppliesTheSheetPolicy is the one place the Excel
// default and the download path meet. A workbook arriving over HTTP is staged
// into a temp file whose name comes from the URL, so nothing about the local
// path is under the user's control, and the policy has to hold anyway.
func TestRemoteImportOfAWorkbookAppliesTheSheetPolicy(t *testing.T) {
	t.Parallel()

	workbook, err := os.ReadFile(filepath.Join("atago", "testdata", "sheets_visibility.xlsx"))
	if err != nil {
		t.Fatalf("read the visibility fixture: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		_, _ = w.Write(workbook)
	}))
	defer server.Close()

	t.Run("the hidden sheets are left out by default", func(t *testing.T) {
		stdout, stderr, code := run(t, "", "--output-format", "csv",
			"--sql", "SELECT v FROM book_Visible", server.URL+"/book.xlsx")
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
		}
		if !strings.Contains(stdout, "shown") {
			t.Errorf("stdout = %q, want the shown sheet's row", stdout)
		}
		if !strings.Contains(stderr, "Skipped 2 hidden sheets") {
			t.Errorf("stderr = %q, want the skip notice", stderr)
		}
		if strings.Contains(stdout, "Skipped") {
			t.Errorf("stdout = %q; the notice must not reach the result stream", stdout)
		}
	})

	t.Run("a hidden sheet has no table", func(t *testing.T) {
		_, _, code := run(t, "", "--sql", "SELECT v FROM book_Internal", server.URL+"/book.xlsx")
		if code != 1 {
			t.Errorf("exit = %d, want 1 (the query failed on a table that was not imported)", code)
		}
	})

	t.Run("--include-hidden-sheets imports them", func(t *testing.T) {
		stdout, stderr, code := run(t, "", "--include-hidden-sheets", "--output-format", "csv",
			"--sql", "SELECT v FROM book_Secret", server.URL+"/book.xlsx")
		if code != 0 {
			t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
		}
		if !strings.Contains(stdout, "very-hidden") {
			t.Errorf("stdout = %q, want the very hidden sheet's row", stdout)
		}
	})
}

// TestRemoteImportLeavesNoTemporaryFileBehind checks the promise that outlives
// the run: the download is staged somewhere, and that somewhere is gone whether
// the import succeeded or failed.
func TestRemoteImportLeavesNoTemporaryFileBehind(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(remoteCSV))
	}))
	defer server.Close()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("this,is\nnot,a,valid,row,count\n"))
	}))
	defer badServer.Close()

	for _, tt := range []struct {
		name string
		url  string
	}{
		{name: "after a successful import", url: server.URL + "/user.csv"},
		{name: "after a failed import", url: badServer.URL + "/user.csv"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			home := t.TempDir()
			stdout, _, _ := runWithEnv(t, repoRoot(), home, []string{"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp},
				"", "--sql", "SELECT 1", tt.url)
			_ = stdout

			entries, err := os.ReadDir(tmp)
			if err != nil {
				t.Fatalf("read the temp directory: %v", err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "sqly-http-") {
					t.Errorf("the download staging directory %q was left behind", entry.Name())
				}
			}
		})
	}
}
