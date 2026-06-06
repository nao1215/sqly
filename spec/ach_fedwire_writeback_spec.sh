#!/bin/sh
# shellcheck shell=sh
#
# Native ACH and Fedwire write-back (issue #242). sqly can reconstruct a complete
# .ach/.fed file from its imported table set after an in-session UPDATE, via the
# whole-set --save/--save-dir path. The single-table --output/.dump path still
# rejects .ach/.fed because those formats need a coordinated record set.

Describe 'sqly ACH/Fedwire native write-back'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORKDIR=$(mktemp -d)
    cp "$PROJECT_ROOT/testdata/ppd-debit.ach" "$WORKDIR/payment.ach"
    cp "$PROJECT_ROOT/testdata/customer-transfer.fed" "$WORKDIR/transfer.fed"
  }
  cleanup() {
    rm -rf "$WORKDIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'round-trips an ACH file through --save --force after an UPDATE'
    When run sh -c "cd '$WORKDIR' && '$SQLY_BIN' --sql \"UPDATE payment_entries SET individual_name='E2E Receiver' WHERE entry_index=0\" --save --force payment.ach && '$SQLY_BIN' --json --sql 'SELECT individual_name FROM payment_entries WHERE entry_index=0' payment.ach"
    The status should be success
    The output should include 'E2E Receiver'
    The stderr should include 'Saved'
  End

  It 'writes a reconstructed ACH file into a directory with --save-dir'
    When run sh -c "cd '$WORKDIR' && '$SQLY_BIN' --sql \"UPDATE payment_entries SET individual_name='Dir Receiver' WHERE entry_index=0\" --save-dir out payment.ach && '$SQLY_BIN' --json --sql 'SELECT individual_name FROM payment_entries WHERE entry_index=0' out/payment.ach"
    The status should be success
    The output should include 'Dir Receiver'
    The stderr should include 'Saved'
  End

  It 'round-trips a Fedwire file through --save --force after an UPDATE'
    When run sh -c "cd '$WORKDIR' && '$SQLY_BIN' --sql \"UPDATE transfer_message SET sender_reference='E2EREF'\" --save --force transfer.fed && '$SQLY_BIN' --json --sql 'SELECT sender_reference FROM transfer_message' transfer.fed"
    The status should be success
    The output should include 'E2EREF'
    The stderr should include 'Saved'
  End

  It 'still rejects a single-table --output to .ach'
    When run sqly --sql "SELECT * FROM payment_entries" --output "$WORKDIR/out.ach" "$WORKDIR/payment.ach"
    The status should be failure
    The stderr should include 'input-only'
  End
End
