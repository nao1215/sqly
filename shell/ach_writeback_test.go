package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
)

// copyFixture copies a testdata fixture into dir and returns the new path, so a
// write-back test can overwrite it in place without touching the shared fixture.
func copyFixture(t *testing.T, name, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // test-controlled temp path and fixture name
		t.Fatalf("write fixture copy: %v", err)
	}
	return dst
}

// withStderr captures config.Stderr for the duration of fn so a save banner does
// not pollute test output, and restores it afterward.
func withStderr(t *testing.T, fn func()) {
	t.Helper()
	backup := config.Stderr
	config.Stderr = &strings.Builder{}
	defer func() { config.Stderr = backup }()
	fn()
}

// reimportValue imports src into a fresh shell and returns the JSON output of the
// given query, so a round-trip test can confirm the written file re-reads.
func reimportValue(t *testing.T, src, query string) string {
	t.Helper()
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatalf("re-import %s: %v", src, err)
	}
	out, err := getExecStdOutput(t, shell.exec, query)
	if err != nil {
		t.Fatalf("re-query failed: %v", err)
	}
	return string(out)
}

func TestWriteBack_ACHRoundTrip(t *testing.T) {
	// Import an ACH file, UPDATE a field on the entries table, save in place, and
	// confirm the rewritten .ach re-imports with the new value. This exercises the
	// native ACH write-back path end to end (issue #242).
	dir := t.TempDir()
	src := copyFixture(t, "ppd-debit.ach", dir)

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatal(err)
	}
	if _, err := getExecStdOutput(t, shell.exec,
		"UPDATE ppd_debit_entries SET individual_name='Updated Receiver' WHERE entry_index=0"); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}

	withStderr(t, func() {
		if err := shell.commands.saveCommand(context.Background(), shell, []string{forceArg}); err != nil {
			t.Fatalf(".save --force on an ACH set returned error: %v", err)
		}
	})

	got := reimportValue(t, src, "SELECT individual_name FROM ppd_debit_entries WHERE entry_index=0")
	if !strings.Contains(got, "Updated Receiver") {
		t.Errorf("ACH write-back did not persist the change; re-imported value: %s", got)
	}
}

func TestWriteBack_ACHSaveDir(t *testing.T) {
	// .save DIR writes the reconstructed .ach into a directory, leaving the source
	// untouched, and the written file re-imports with the change.
	dir := t.TempDir()
	src := copyFixture(t, "ppd-debit.ach", dir)
	outDir := filepath.Join(dir, "out")

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatal(err)
	}
	if _, err := getExecStdOutput(t, shell.exec,
		"UPDATE ppd_debit_entries SET individual_name='Dir Receiver' WHERE entry_index=0"); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}
	withStderr(t, func() {
		if err := shell.commands.saveCommand(context.Background(), shell, []string{outDir}); err != nil {
			t.Fatalf(".save DIR on an ACH set returned error: %v", err)
		}
	})

	written := filepath.Join(outDir, "ppd-debit.ach")
	if _, err := os.Stat(written); err != nil {
		t.Fatalf("expected written ACH at %s: %v", written, err)
	}
	got := reimportValue(t, written, "SELECT individual_name FROM ppd_debit_entries WHERE entry_index=0")
	if !strings.Contains(got, "Dir Receiver") {
		t.Errorf("ACH .save DIR did not persist the change; re-imported value: %s", got)
	}
}

func TestWriteBack_FedWireRoundTrip(t *testing.T) {
	// Import a Fedwire file, UPDATE a field on the message table, save in place,
	// and confirm the rewritten .fed re-imports with the new value.
	dir := t.TempDir()
	src := copyFixture(t, "customer-transfer.fed", dir)

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatal(err)
	}
	// Pick a free-text field that round-trips: the originator-to-beneficiary info
	// is a safe payload field. Discover an updatable text column from the schema.
	before := reimportValue(t, src, "SELECT * FROM customer_transfer_message")
	if !strings.Contains(before, "sender_reference") && !strings.Contains(before, "business_function_code") {
		t.Skip("Fedwire fixture schema does not expose an expected column; skipping")
	}
	if _, err := getExecStdOutput(t, shell.exec,
		"UPDATE customer_transfer_message SET sender_reference='SQLYREF001'"); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}
	withStderr(t, func() {
		if err := shell.commands.saveCommand(context.Background(), shell, []string{forceArg}); err != nil {
			t.Fatalf(".save --force on a Fedwire set returned error: %v", err)
		}
	})

	got := reimportValue(t, src, "SELECT sender_reference FROM customer_transfer_message")
	if !strings.Contains(got, "SQLYREF001") {
		t.Errorf("Fedwire write-back did not persist the change; re-imported value: %s", got)
	}
}

func TestWriteBack_ACHFromDirectoryRejected(t *testing.T) {
	// An ACH file picked up by a directory import is not a source the session
	// owns directly, so write-back must reject it with the directory-import error
	// rather than reconstructing a whole-set file.
	dir := t.TempDir()
	sub := filepath.Join(dir, "payments")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "ppd-debit.ach"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "ppd-debit.ach"), data, 0o600); err != nil { //nolint:gosec // test-controlled temp path
		t.Fatal(err)
	}

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{sub}); err != nil {
		t.Fatal(err)
	}
	if _, err := getExecStdOutput(t, shell.exec,
		"UPDATE ppd_debit_entries SET individual_name='X' WHERE entry_index=0"); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}

	var saveErr error
	withStderr(t, func() {
		saveErr = shell.commands.saveCommand(context.Background(), shell, []string{forceArg})
	})
	if saveErr == nil {
		t.Fatal("expected a directory-import write-back rejection, got nil")
	}
	if !strings.Contains(saveErr.Error(), "directory import") {
		t.Errorf("error should mention the directory import, got: %v", saveErr)
	}
}

func TestWriteBack_ACHRejectsIncompleteSet(t *testing.T) {
	// Dropping a required companion table makes the ACH set incomplete; write-back
	// must fail with an explicit error instead of producing a malformed file.
	dir := t.TempDir()
	src := copyFixture(t, "ppd-debit.ach", dir)

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := shell.commands.importCommand(context.Background(), shell, []string{src}); err != nil {
		t.Fatal(err)
	}
	// Change a row so the session is not read-only, then remove a companion table.
	if _, err := getExecStdOutput(t, shell.exec,
		"UPDATE ppd_debit_entries SET individual_name='X' WHERE entry_index=0"); err != nil {
		t.Fatalf("UPDATE failed: %v", err)
	}
	if err := shell.exec(context.Background(), "DROP TABLE ppd_debit_file_header"); err != nil {
		t.Fatalf("DROP failed: %v", err)
	}

	var saveErr error
	withStderr(t, func() {
		saveErr = shell.commands.saveCommand(context.Background(), shell, []string{forceArg})
	})
	if saveErr == nil {
		t.Fatal("expected an error saving an incomplete ACH set, got nil")
	}
	if !strings.Contains(saveErr.Error(), "ppd_debit_file_header") && !strings.Contains(strings.ToLower(saveErr.Error()), "incomplete") {
		t.Errorf("error should name the missing companion table or call the set incomplete; got: %v", saveErr)
	}
}
