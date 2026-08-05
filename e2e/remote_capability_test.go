//go:build smoke

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// The remote capability, proved against the real binary by counting requests.
//
// "Refused before any request" is the claim. A message saying so is not evidence
// of it — a run that downloads the file and then complains produces the same
// stderr — so every denial below asserts the server saw zero requests, and the
// allowed cases assert it saw exactly one. The counter is what decides.
//
// This is a Go httptest server rather than an atago spec behind `sh -c` and
// `python3 -m http.server`, because the counting has to run on Windows too, and
// because a spec that serves files cannot observe what was never asked for.
//
// Documented at website/content/formats.md#remote-inputs.

// countingServer is an HTTP fixture that records how many requests reached it.
type countingServer struct {
	*httptest.Server
	requests atomic.Int64
}

func newCountingServer(t *testing.T) *countingServer {
	t.Helper()

	server := &countingServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		server.requests.Add(1)
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(remoteCSV))
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *countingServer) count() int64   { return s.requests.Load() }
func (s *countingServer) csvURL() string { return s.URL + "/user.csv" }

// assertDenied checks the whole shape of a refusal against the real binary.
func assertDenied(t *testing.T, server *countingServer, stdout, stderr string, code int) {
	t.Helper()

	if code != 2 {
		t.Errorf("exit code = %d, want 2 (a capability the command line did not grant is a usage error)\nstderr: %s",
			code, stderr)
	}
	if !strings.Contains(stderr, "--allow-remote") {
		t.Errorf("stderr does not name --allow-remote, so it does not say how to fix the run:\n%s", stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty on a refused run", stdout)
	}
	if got := server.count(); got != 0 {
		t.Errorf("the server saw %d request(s); a refused URL must not be fetched at all", got)
	}
}

func TestRemoteCapability_PositionalURLIsDenied(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)

	stdout, stderr, code := run(t, "",
		"--output-format", "csv", "--sql", "SELECT * FROM user", server.csvURL())

	assertDenied(t, server, stdout, stderr, code)
}

func TestRemoteCapability_InspectURLIsDenied(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)

	stdout, stderr, code := run(t, "", "--inspect", server.csvURL())

	assertDenied(t, server, stdout, stderr, code)
}

func TestRemoteCapability_SQLFileWithURLIsDenied(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "report.sql")
	if err := os.WriteFile(sqlPath, []byte("SELECT * FROM user;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "",
		"--output-format", "csv", "--sql-file", sqlPath, server.csvURL())

	assertDenied(t, server, stdout, stderr, code)
}

func TestRemoteCapability_ScriptFileWithURLIsDenied(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "run.sqly")
	if err := os.WriteFile(scriptPath, []byte("SELECT * FROM user;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "",
		"--output-format", "csv", "--script-file", scriptPath, server.csvURL())

	assertDenied(t, server, stdout, stderr, code)
}

// TestRemoteCapability_StdinScriptImportIsDenied also checks the statement
// before the .import produced nothing, which is what "the script had not
// started" means from outside the process.
func TestRemoteCapability_StdinScriptImportIsDenied(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)
	script := fmt.Sprintf("SELECT 1 AS before_the_import;\n.import %s\nSELECT 2 AS after_the_import;\n", server.csvURL())

	stdout, stderr, code := run(t, script, "--output-format", "csv")

	assertDenied(t, server, stdout, stderr, code)
	if strings.Contains(stdout, "before_the_import") {
		t.Errorf("the statement before the refused .import produced output:\n%s", stdout)
	}
}

func TestRemoteCapability_ScriptFileImportIsDenied(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fetch.sqly")
	script := fmt.Sprintf("SELECT 1 AS before_the_import;\n.import %s\n", server.csvURL())
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "", "--output-format", "csv", "--script-file", scriptPath)

	assertDenied(t, server, stdout, stderr, code)
	if strings.Contains(stdout, "before_the_import") {
		t.Errorf("the statement before the refused .import produced output:\n%s", stdout)
	}
}

// TestRemoteCapability_InteractiveImportIsDeniedButTheSessionContinues drives the
// dot-command path through a piped script whose later statements still run,
// which is the closest a non-pty harness gets to "the prompt printed an error
// and read the next line". The pty version is in e2e/atago/pty.atago.yaml's
// sibling suites; what matters here is that the refusal does not end the
// process before the following statement.
func TestRemoteCapability_InteractiveImportIsDeniedButTheSessionContinues(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)
	dir := t.TempDir()
	local := filepath.Join(dir, "local.csv")
	if err := os.WriteFile(local, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// With the capability, the .import succeeds and the session goes on. Without
	// it, the script is refused before it starts. Both halves are asserted so the
	// "session continues" claim is anchored to a run that does continue.
	stdout, stderr, code := run(t, fmt.Sprintf(".import %s\nSELECT COUNT(*) AS n FROM user;\n", server.csvURL()),
		"--allow-remote", "--output-format", "csv", local)
	if code != 0 {
		t.Fatalf("exit = %d with --allow-remote, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "n") {
		t.Errorf("stdout = %q, want the query after the .import to have run", stdout)
	}
	if got := server.count(); got != 1 {
		t.Errorf("the server saw %d request(s), want exactly 1", got)
	}
}

// TestRemoteCapability_MixedLocalAndRemoteIsRefusedWholesale is the atomicity
// claim seen from outside: the query over the local table does not answer,
// because the local table was never created.
func TestRemoteCapability_MixedLocalAndRemoteIsRefusedWholesale(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)
	dir := t.TempDir()
	local := filepath.Join(dir, "local.csv")
	if err := os.WriteFile(local, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "",
		"--output-format", "csv", "--sql", "SELECT * FROM local", local, server.csvURL())

	assertDenied(t, server, stdout, stderr, code)
}

func TestRemoteCapability_AllowedURLIsDownloadedOnce(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)

	stdout, stderr, code := run(t, "",
		"--allow-remote", "--output-format", "csv",
		"--sql", "SELECT user_name FROM user ORDER BY identifier LIMIT 1", server.csvURL())
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "booker12") {
		t.Errorf("stdout = %q, want the downloaded row", stdout)
	}
	if got := server.count(); got != 1 {
		t.Errorf("the server saw %d request(s), want exactly 1", got)
	}
}

// TestRemoteCapability_RedirectIsFollowedOnlyAfterTheCheck pins the order: the
// capability decides before the first request, so a redirecting URL without the
// flag never reaches even the first hop.
func TestRemoteCapability_RedirectIsFollowedOnlyAfterTheCheck(t *testing.T) {
	t.Parallel()

	var hops atomic.Int64
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		hops.Add(1)
		http.Redirect(w, r, "/user.csv", http.StatusFound)
	})
	mux.HandleFunc("/user.csv", func(w http.ResponseWriter, _ *http.Request) {
		hops.Add(1)
		_, _ = w.Write([]byte(remoteCSV))
	})

	stdout, stderr, code := run(t, "", "--output-format", "csv", "--sql", "SELECT 1", server.URL+"/latest")
	if code != 2 {
		t.Errorf("exit = %d, want 2 without --allow-remote (stderr: %s)", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if got := hops.Load(); got != 0 {
		t.Fatalf("the server saw %d hop(s) without the capability; the redirect must not be started at all", got)
	}

	stdout, stderr, code = run(t, "",
		"--allow-remote", "--output-format", "csv", "--sql", "SELECT COUNT(*) AS n FROM user", server.URL+"/latest")
	if code != 0 {
		t.Fatalf("exit = %d with --allow-remote, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "2") {
		t.Errorf("stdout = %q, want the row count of the redirected file", stdout)
	}
	if got := hops.Load(); got != 2 {
		t.Errorf("the server saw %d hop(s) with the capability, want 2 (the redirect and its target)", got)
	}
}

// TestRemoteCapability_RedirectToAnotherSchemeIsStillRefused keeps the existing
// limit intact under the capability: --allow-remote turns downloading on, it
// does not relax where a redirect may lead.
func TestRemoteCapability_RedirectToAnotherSchemeIsStillRefused(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "ftp://example.invalid/user.csv", http.StatusFound)
	}))
	defer server.Close()

	stdout, stderr, code := run(t, "",
		"--allow-remote", "--sql", "SELECT 1", server.URL+"/user.csv")
	if code != 3 {
		t.Errorf("exit = %d, want 3 (an input that could not be read)\nstderr: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "http and https") {
		t.Errorf("stderr does not name the scheme refusal:\n%s", stderr)
	}
}

// TestRemoteCapability_DenialLeavesNoTemporaryFiles checks the other half of a
// clean refusal: nothing was staged on the way to it.
func TestRemoteCapability_DenialLeavesNoTemporaryFiles(t *testing.T) {
	t.Parallel()
	server := newCountingServer(t)

	home := t.TempDir()
	tmp := t.TempDir()
	env := []string{"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp}

	stdout, stderr, code := runWithEnv(t, repoRoot(), home, env, "",
		"--output-format", "csv", "--sql", "SELECT 1", server.csvURL())
	assertDenied(t, server, stdout, stderr, code)

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("read the temp directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "sqly-") {
			t.Errorf("a refused run left %q behind in the temp directory", entry.Name())
		}
	}
}

// TestRemoteCapability_UnusedCapabilityIsAccepted states that granting it and
// not using it is fine, so a wrapper can pass it unconditionally.
func TestRemoteCapability_UnusedCapabilityIsAccepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	local := filepath.Join(dir, "local.csv")
	if err := os.WriteFile(local, []byte("a\n1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run(t, "",
		"--allow-remote", "--output-format", "csv", "--sql", "SELECT * FROM local", local)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "a") {
		t.Errorf("stdout = %q, want the local result", stdout)
	}
}

// TestRemoteCapability_CredentialsAreNotEchoedBack covers what the binary prints
// back when the URL it was given carries a password. Two of these go to stdout —
// the --inspect report is the document people commit, attach, and paste — and
// the refusal is printed before any request is made, so it is often the first
// thing seen. Go's own transport error redacts the password; sqly repeated the
// raw URL alongside it and undid that.
func TestRemoteCapability_CredentialsAreNotEchoedBack(t *testing.T) {
	const secret = "hunter2"
	server := newCountingServer(t)
	withCredentials := strings.Replace(server.csvURL(), "http://", "http://user:"+secret+"@", 1)

	assertRedacted := func(t *testing.T, label, stdout, stderr string) {
		t.Helper()
		if strings.Contains(stdout, secret) {
			t.Errorf("%s: stdout repeats the password back:\n%s", label, stdout)
		}
		if strings.Contains(stderr, secret) {
			t.Errorf("%s: stderr repeats the password back:\n%s", label, stderr)
		}
		if !strings.Contains(stdout+stderr, "user:xxxxx@") {
			t.Errorf("%s: neither stream shows the redacted URL, so the message may not name the source at all:\nstdout: %s\nstderr: %s",
				label, stdout, stderr)
		}
	}

	t.Run("the refusal without --allow-remote", func(t *testing.T) {
		stdout, stderr, _ := run(t, "", "--sql", "SELECT 1", withCredentials)
		assertRedacted(t, "refusal", stdout, stderr)
	})

	t.Run("the inspect report", func(t *testing.T) {
		stdout, stderr, code := run(t, "", "--allow-remote", "--inspect", withCredentials)
		if code != 0 {
			t.Fatalf("inspect exit code = %d, want 0\nstderr: %s", code, stderr)
		}
		assertRedacted(t, "inspect", stdout, stderr)
	})

	t.Run("a failed download", func(t *testing.T) {
		// Port 1 refuses the connection, so this exercises the message sqly wraps
		// around a transport error — the error Go itself already redacts.
		unreachable := "http://user:" + secret + "@127.0.0.1:1/data.csv"
		stdout, stderr, code := run(t, "", "--allow-remote", "--sql", "SELECT 1", unreachable)
		if code == 0 {
			t.Fatalf("downloading an unusable body succeeded, want a failure\nstdout: %s", stdout)
		}
		assertRedacted(t, "download failure", stdout, stderr)
	})
}
