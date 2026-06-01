# Output format demo: the same query rendered as table, CSV, JSON, NDJSON.
# Regenerate with: vhs doc/vhs/formats.tape  (run from the repository root)
Output "doc/img/formats-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 450
Set Padding 18
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'SELECT user_name, identifier FROM user LIMIT 2' testdata/user.csv"
Sleep 600ms
Enter
Sleep 1800ms
Type "sqly --csv --sql 'SELECT user_name, identifier FROM user LIMIT 2' testdata/user.csv"
Sleep 600ms
Enter
Sleep 1800ms
Type "sqly --json --sql 'SELECT user_name, identifier FROM user LIMIT 2' testdata/user.csv"
Sleep 600ms
Enter
Sleep 1800ms
Type "sqly --ndjson --sql 'SELECT user_name, identifier FROM user LIMIT 2' testdata/user.csv"
Sleep 600ms
Enter
Sleep 2500ms
