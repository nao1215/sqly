package shell

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// The remote capability, tested by counting requests rather than by reading
// error text.
//
// "Refused before any request" is the claim, and a message saying so is not
// evidence of it: a run that downloads the file and then complains produces the
// same stderr. Every denial test below asserts the server saw zero requests, and
// the allowed cases assert it saw exactly one, so the counter is what decides.

// countingRemoteServer is an HTTP fixture that records how many requests reached
// it. It serves one CSV, so a run that gets past the capability check succeeds
// and a run that does not leaves the counter at zero.
type countingRemoteServer struct {
	*httptest.Server
	requests atomic.Int64
}

func newCountingRemoteServer(t *testing.T) *countingRemoteServer {
	t.Helper()

	server := &countingRemoteServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		server.requests.Add(1)
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("user_name,identifier\nbooker12,1\n"))
	}))
	t.Cleanup(server.Close)
	return server
}

// count returns how many requests the server has seen.
func (s *countingRemoteServer) count() int64 { return s.requests.Load() }

// csvURL is the URL of the served dataset.
func (s *countingRemoteServer) csvURL() string { return s.URL + "/user.csv" }

// runShell builds a shell from args, points it at the fixture server's
// transport, and runs it, returning the error and what reached stdout.
func runShellCapturingStdout(t *testing.T, server *countingRemoteServer, stdin string, args ...string) (string, error) {
	t.Helper()

	shell, cleanup, err := newShell(t, args)
	if err != nil {
		return "", err
	}
	defer cleanup()
	shell.httpClient = server.Client()
	shell.isTTY = func() bool { return false }
	if stdin != "" {
		shell.stdin = strings.NewReader(stdin)
	}

	var runErr error
	out := captureStdout(t, func() {
		runErr = shell.Run(context.Background())
	})
	return out, runErr
}

// assertRemoteDenied checks the whole shape of a refusal: it is a usage error,
// stdout is empty, and the server was never contacted.
func assertRemoteDenied(t *testing.T, server *countingRemoteServer, stdout string, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("run succeeded, want a refusal for a remote input without --allow-remote")
	}
	if code := ExitCode(err); code != ExitUsage {
		t.Errorf("exit code = %d, want %d (a capability the command line did not grant is a usage error): %v",
			code, ExitUsage, err)
	}
	if !strings.Contains(err.Error(), remoteCapabilityFlag) {
		t.Errorf("error %q does not name %s, so it does not say how to fix the run", err, remoteCapabilityFlag)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty on a refused run", stdout)
	}
	if got := server.count(); got != 0 {
		t.Errorf("the server saw %d request(s); a refused URL must not be fetched at all", got)
	}
}

func TestRemote_PositionalURLIsDeniedWithoutTheFlag(t *testing.T) {
	server := newCountingRemoteServer(t)

	stdout, err := runShellCapturingStdout(t, server, "",
		"sqly", "--output-format", "csv", "--sql", "SELECT * FROM user", server.csvURL())

	assertRemoteDenied(t, server, stdout, err)
}

func TestRemote_InspectURLIsDeniedWithoutTheFlag(t *testing.T) {
	server := newCountingRemoteServer(t)

	stdout, err := runShellCapturingStdout(t, server, "", "sqly", "--inspect", server.csvURL())

	assertRemoteDenied(t, server, stdout, err)
}

func TestRemote_SQLFileWithURLIsDeniedWithoutTheFlag(t *testing.T) {
	server := newCountingRemoteServer(t)
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "report.sql")
	if err := os.WriteFile(sqlPath, []byte("SELECT * FROM user;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, err := runShellCapturingStdout(t, server, "",
		"sqly", "--output-format", "csv", "--sql-file", sqlPath, server.csvURL())

	assertRemoteDenied(t, server, stdout, err)
}

func TestRemote_ScriptFileWithURLIsDeniedWithoutTheFlag(t *testing.T) {
	server := newCountingRemoteServer(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "run.sqly")
	if err := os.WriteFile(scriptPath, []byte("SELECT * FROM user;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, err := runShellCapturingStdout(t, server, "",
		"sqly", "--output-format", "csv", "--script-file", scriptPath, server.csvURL())

	assertRemoteDenied(t, server, stdout, err)
}

// TestRemote_StdinScriptImportIsDeniedWithoutTheFlag also checks that the
// statement before the .import did not run: nothing reached stdout, so the
// refusal happened before the script started rather than part-way through it.
func TestRemote_StdinScriptImportIsDeniedWithoutTheFlag(t *testing.T) {
	server := newCountingRemoteServer(t)
	script := fmt.Sprintf("SELECT 1 AS before_the_import;\n.import %s\nSELECT 2 AS after_the_import;\n", server.csvURL())

	stdout, err := runShellCapturingStdout(t, server, script, "sqly", "--output-format", "csv")

	assertRemoteDenied(t, server, stdout, err)
	if strings.Contains(stdout, "before_the_import") {
		t.Errorf("the statement before the refused .import produced output:\n%s", stdout)
	}
}

func TestRemote_ScriptFileImportIsDeniedWithoutTheFlag(t *testing.T) {
	server := newCountingRemoteServer(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "run.sqly")
	script := fmt.Sprintf("SELECT 1 AS before_the_import;\n.import %s\n", server.csvURL())
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, err := runShellCapturingStdout(t, server, "",
		"sqly", "--output-format", "csv", "--script-file", scriptPath)

	assertRemoteDenied(t, server, stdout, err)
	if strings.Contains(stdout, "before_the_import") {
		t.Errorf("the statement before the refused .import produced output:\n%s", stdout)
	}
}

// TestRemote_MixedLocalAndRemoteIsRefusedWholesale is the atomicity claim: a
// command line naming a local file and a URL imports neither.
func TestRemote_MixedLocalAndRemoteIsRefusedWholesale(t *testing.T) {
	server := newCountingRemoteServer(t)
	dir := t.TempDir()
	local := writeCSV(t, dir, "local.csv", "a\n1\n")

	shell, cleanup, err := newShell(t, []string{"sqly", "--output-format", "csv", "--sql", "SELECT * FROM local", local, server.csvURL()})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.httpClient = server.Client()
	shell.isTTY = func() bool { return false }

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = shell.Run(context.Background())
	})
	assertRemoteDenied(t, server, stdout, runErr)

	// The local half must not be in the session either. Asking the database
	// directly is the check that matters: an empty stdout could also mean the
	// query simply printed nothing.
	tables, err := shell.usecases.metadata.TablesName(context.Background())
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	for _, table := range tables {
		if table.Name() == "local" {
			t.Error("the local input was imported even though the run was refused; a mixed import must be all or nothing")
		}
	}
}

// TestRemote_InteractiveImportIsDeniedButTheSessionContinues is the interactive
// half: the command fails, and the shell is still usable and unchanged.
func TestRemote_InteractiveImportIsDeniedButTheSessionContinues(t *testing.T) {
	server := newCountingRemoteServer(t)
	dir := t.TempDir()
	local := writeCSV(t, dir, "local.csv", "a\n1\n")

	shell, cleanup, err := newShell(t, []string{"sqly", "--output-format", "csv", local})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.httpClient = server.Client()
	ctx := context.Background()
	if err := shell.init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	before, err := shell.usecases.metadata.TablesName(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// exec is what the prompt loop calls for a typed line, so this is the same
	// path an interactive .import takes.
	importErr := shell.exec(ctx, ".import "+server.csvURL())
	if importErr == nil {
		t.Fatal(".import of a URL succeeded without --allow-remote")
	}
	if !strings.Contains(importErr.Error(), remoteCapabilityFlag) {
		t.Errorf("error %q does not name %s", importErr, remoteCapabilityFlag)
	}
	if got := server.count(); got != 0 {
		t.Errorf("the server saw %d request(s) for a refused interactive .import", got)
	}

	// The prompt loop treats a command error as recoverable — it prints and reads
	// the next line — so the session must still work and must be unchanged.
	after, err := shell.usecases.metadata.TablesName(ctx)
	if err != nil {
		t.Fatalf("the session is unusable after a refused .import: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("table count changed from %d to %d after a refused .import", len(before), len(after))
	}
	if err := shell.exec(ctx, "SELECT * FROM local"); err != nil {
		t.Errorf("the session cannot run a query after a refused .import: %v", err)
	}
	if _, recorded := shell.tableSources["user"]; recorded {
		t.Error("a refused .import recorded a table source")
	}
}

// TestRemote_AllowedURLIsDownloadedExactlyOnce is the positive control: with the
// capability the same run works, and the server sees one request.
func TestRemote_AllowedURLIsDownloadedExactlyOnce(t *testing.T) {
	server := newCountingRemoteServer(t)

	stdout, err := runShellCapturingStdout(t, server, "",
		"sqly", "--allow-remote", "--output-format", "csv",
		"--sql", "SELECT user_name FROM user", server.csvURL())
	if err != nil {
		t.Fatalf("run with --allow-remote failed: %v", err)
	}
	if !strings.Contains(stdout, "booker12") {
		t.Errorf("stdout = %q, want the downloaded row", stdout)
	}
	if got := server.count(); got != 1 {
		t.Errorf("the server saw %d request(s), want exactly 1", got)
	}
}

// TestRemote_CapabilityCarriesIntoTheSession checks that a session started with
// the flag keeps it for the .import typed later, which is the interactive shape
// of the same grant.
func TestRemote_CapabilityCarriesIntoTheSession(t *testing.T) {
	server := newCountingRemoteServer(t)

	shell, cleanup, err := newShell(t, []string{"sqly", "--allow-remote", "--output-format", "csv"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	shell.httpClient = server.Client()
	ctx := context.Background()

	if err := shell.exec(ctx, ".import "+server.csvURL()); err != nil {
		t.Fatalf(".import with --allow-remote failed: %v", err)
	}
	if got := server.count(); got != 1 {
		t.Errorf("the server saw %d request(s), want exactly 1", got)
	}
}

// TestRemote_AllowRemoteWithoutAURLIsFine states that granting a capability the
// run does not use is not an error. A wrapper that always passes the flag must
// not have to know in advance whether the command line holds a URL.
func TestRemote_AllowRemoteWithoutAURLIsFine(t *testing.T) {
	server := newCountingRemoteServer(t)
	dir := t.TempDir()
	local := writeCSV(t, dir, "local.csv", "a\n1\n")

	stdout, err := runShellCapturingStdout(t, server, "",
		"sqly", "--allow-remote", "--output-format", "csv", "--sql", "SELECT * FROM local", local)
	if err != nil {
		t.Fatalf("--allow-remote with no URL failed: %v", err)
	}
	if !strings.Contains(stdout, "a") {
		t.Errorf("stdout = %q, want the local result", stdout)
	}
	if got := server.count(); got != 0 {
		t.Errorf("the server saw %d request(s) for a run with no URL", got)
	}
}

// TestRemote_CapabilityDoesNotUnlockOtherSchemes keeps --allow-remote to what it
// says: http and https. A scheme sqly cannot fetch is refused with the flag
// exactly as without it, and no request is attempted.
func TestRemote_CapabilityDoesNotUnlockOtherSchemes(t *testing.T) {
	for _, url := range []string{
		"ftp://example.test/data.csv",
		"file:///etc/passwd",
		"ssh://example.test/data.csv",
		"gopher://example.test/data.csv",
	} {
		t.Run(url, func(t *testing.T) {
			for _, allow := range []bool{false, true} {
				args := []string{"sqly", "--sql", "SELECT 1", url}
				if allow {
					args = []string{"sqly", "--allow-remote", "--sql", "SELECT 1", url}
				}
				shell, cleanup, err := newShell(t, args)
				if err != nil {
					t.Fatal(err)
				}
				shell.isTTY = func() bool { return false }
				runErr := shell.Run(context.Background())
				cleanup()
				if runErr == nil {
					t.Fatalf("--allow-remote=%v accepted %s, want a refusal", allow, url)
				}
				if !strings.Contains(runErr.Error(), "http and https") {
					t.Errorf("--allow-remote=%v: error %q does not say only http and https are downloaded", allow, runErr)
				}
			}
		})
	}
}

// TestRemote_LocalPathsThatLookLikeURLsStayLocal keeps the capability check from
// widening what counts as a URL. A Windows drive path and a file name holding a
// colon are local paths, and refusing them for want of a network capability
// would be a regression in a different direction.
func TestRemote_LocalPathsThatLookLikeURLsStayLocal(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for _, path := range []string{`C:\data\sales.csv`, "weird:name.csv", "./relative.csv", "/tmp/plain.csv"} {
		if err := shell.authorizeRemoteInputs([]string{path}); err != nil {
			t.Errorf("authorizeRemoteInputs(%q) = %v, want nil: it is a local path, not a URL", path, err)
		}
	}
}

// TestRemote_DenialIsAnInvocationError pins the classification the exit code is
// derived from, so a later refactor of the message cannot quietly move a refused
// run into another exit class.
func TestRemote_DenialIsAnInvocationError(t *testing.T) {
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	denied := shell.authorizeRemoteInputs([]string{"https://example.test/a.csv"})
	var invocationErr *invocationError
	if !errors.As(denied, &invocationErr) {
		t.Fatalf("error %v is not an invocationError", denied)
	}
	if code := ExitCode(denied); code != ExitUsage {
		t.Errorf("exit code = %d, want %d", code, ExitUsage)
	}
}
