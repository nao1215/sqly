# Write-back safety demo: a schema change is rejected before anything is written,
# --save-tables leaves the source alone, and --save-in-place overwrites it.
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
Type "sqly --sql 'UPDATE user SET identifier = identifier + 100' --save-tables ./copies user.csv"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'CREATE TABLE backup AS SELECT * FROM user' --save-tables ./out user.csv"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'UPDATE user SET identifier = identifier + 100' --save-in-place user.csv"
Sleep 600ms
Enter
Sleep 2s
Type "head -2 user.csv"
Sleep 600ms
Enter
Sleep 2s
