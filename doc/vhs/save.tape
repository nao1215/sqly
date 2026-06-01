# Persist changes demo: edit a table with SQL and write it back with --save-dir.
# Regenerate with: vhs doc/vhs/save.tape  (run from the repository root)
Output "doc/img/save-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 250
Set Padding 18
Hide
Type "WORK=$(mktemp -d); cp testdata/user.csv $WORK/; export PATH=$PWD:$PATH; cd $WORK; export PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'UPDATE user SET identifier = identifier + 100' --save-dir ./out user.csv"
Sleep 600ms
Enter
Sleep 2s
Type "cat ./out/user.csv"
Sleep 600ms
Enter
Sleep 2500ms
