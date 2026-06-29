#!/bin/sh
# shellcheck shell=sh
#
# Non-TTY batch-mode end-to-end tests. Piping into sqly (no terminal)
# runs commands from stdin; failures must surface a non-zero exit code.

Describe 'sqly batch mode (piped stdin)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'runs SQL read from stdin'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should include 'booker12'
  End

  It 'switches output mode and runs the following query'
    Data
      #|.mode ndjson
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should include '{"user_name":"booker12"}'
    The stderr should include 'Change output mode'
  End

  It 'exits non-zero and names the failing statement and its line on error'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1;
      #|SELECT * FROM no_such_table;
    End
    When run sqly testdata/user.csv
    The status should be failure
    The output should include 'booker12'
    The stderr should include 'batch statement 2 failed at line 2'
    The stderr should include 'no_such_table'
  End

  It 'reports the line span of a failing multiline statement'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1;
      #|SELECT 1;
      #|SELECT *
      #|FROM no_such_table;
    End
    When run sqly testdata/user.csv
    The status should be failure
    The output should be present
    The stderr should include 'batch statement 3 failed at lines 3-4'
    The stderr should include 'no_such_table'
  End

  It 'stops at .exit with a success status'
    Data
      #|.exit
      #|SELECT * FROM no_such_table
    End
    When run sqly testdata/user.csv
    The status should be success
  End

  It 'still exits non-zero when a failure precedes .exit'
    Data
      #|SELECT * FROM no_such_table;
      #|.exit
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include 'no_such_table'
  End

  Describe 'multiline statements'
    It 'runs a multiline SELECT terminated by a semicolon'
      Data
        #|SELECT user_name
        #|FROM user
        #|ORDER BY identifier
        #|LIMIT 1;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'booker12'
    End

    It 'runs a multiline UNION ALL across bare newlines as one statement'
      Data
        #|.mode csv
        #|SELECT 1 AS n
        #|UNION ALL
        #|SELECT 2;
      End
      When run sqly testdata/user.csv
      The status should be success
      The line 1 should equal 'n'
      The line 2 should equal '1'
      The line 3 should equal '2'
      The stderr should include 'Change output mode'
    End

    It 'runs a multiline WITH (CTE) query'
      Data
        #|WITH x AS (
        #|  SELECT user_name FROM user ORDER BY identifier LIMIT 1
        #|)
        #|SELECT * FROM x;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'booker12'
    End

    It 'runs multiple statements and helper commands in one payload'
      Data
        #|.tables
        #|SELECT COUNT(*) AS c FROM user;
        #|SELECT user_name FROM user ORDER BY identifier LIMIT 1;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'TABLE NAME'
      The output should include 'booker12'
    End

    It 'ignores a semicolon inside a leading line comment'
      Data
        #|-- comment ;
        #|SELECT 'v' AS x;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'v'
    End

    It 'ignores a semicolon inside a block comment'
      Data
        #|/* comment ; */
        #|SELECT 'v' AS x;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'v'
    End

    It 'ignores a semicolon inside a trailing line comment'
      Data
        #|SELECT 'first' AS x; -- trailing ; comment
        #|SELECT 'second' AS y;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'first'
      The output should include 'second'
    End

    It 'does not split on a semicolon inside a bracket-quoted identifier'
      Data
        #|SELECT 'v' AS [a;b];
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'a;b'
      The output should include 'v'
    End

    It 'does not split on a semicolon inside a backtick-quoted identifier'
      Data
        #|SELECT 'v' AS `a;b`;
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'a;b'
      The output should include 'v'
    End

    It 'reports an error for incomplete SQL'
      Data
        #|SELECT * FROM (
      End
      When run sqly testdata/user.csv
      The status should be failure
      The stderr should include 'batch statement'
    End
  End
End
