# Format-converter demo: query a CSV once and write the result out as JSON,
# Parquet, and Excel, then re-query the Parquet file in place.
# Regenerate with: vhs doc/vhs/convert.tape  (run from the repository root)
Output "doc/img/convert-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1200
Set Height 480
Set Padding 18
Set CursorBlink false
Set Framerate 24
Hide
Type "WORK=$(mktemp -d); cp testdata/user.csv $WORK/; export PATH=$PWD:$PATH; cd $WORK; export PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'SELECT user_name, identifier FROM user' --output users.json user.csv"
Sleep 600ms
Enter
Sleep 1800ms
Type "cat users.json"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'SELECT user_name, identifier FROM user' --output users.parquet user.csv"
Sleep 600ms
Enter
Sleep 1800ms
Type "sqly --sql 'SELECT * FROM users' users.parquet"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'SELECT user_name, identifier FROM user' --output users.xlsx user.csv"
Sleep 600ms
Enter
Sleep 1800ms
Type "ls users.*"
Sleep 600ms
Enter
Sleep 3000ms
