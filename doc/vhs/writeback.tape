# Write-back safety demo: --save needs --force, schema changes are rejected,
# and --save --force overwrites the source in place.
# Regenerate with: vhs doc/vhs/writeback.tape  (run from the repository root)
Output "doc/img/writeback-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1180
Set Height 460
Set Padding 18
Hide
Type "WORK=$(mktemp -d); cp testdata/user.csv $WORK/; export PATH=$PWD:$PATH; cd $WORK; export PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'UPDATE user SET identifier = identifier + 100' --save user.csv"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'CREATE TABLE backup AS SELECT * FROM user' --save-dir ./out user.csv"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'UPDATE user SET identifier = identifier + 100' --save --force user.csv"
Sleep 600ms
Enter
Sleep 2s
Type "head -2 user.csv"
Sleep 600ms
Enter
Sleep 2s
