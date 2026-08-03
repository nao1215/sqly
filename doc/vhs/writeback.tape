# Write-back safety demo: .save DIR leaves the source alone, a schema change is
# rejected before anything is written, and .save --in-place overwrites the source.
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
Type "printf 'UPDATE user SET identifier = identifier + 100;\n.save ./copies\n' | sqly user.csv"
Sleep 600ms
Enter
Sleep 2500ms
Type "printf 'CREATE TABLE backup AS SELECT * FROM user;\n.save ./out\n' | sqly user.csv"
Sleep 600ms
Enter
Sleep 2500ms
Type "printf 'UPDATE user SET identifier = identifier + 100;\n.save --in-place\n' | sqly user.csv"
Sleep 600ms
Enter
Sleep 2s
Type "head -2 user.csv"
Sleep 600ms
Enter
Sleep 2s
