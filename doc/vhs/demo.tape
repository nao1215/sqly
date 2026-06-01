# Hero demo: run SQL against CSV files, including a JOIN across two files.
# Regenerate with: vhs doc/vhs/demo.tape  (run from the repository root)
Output "doc/img/demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 420
Set Padding 18
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'SELECT * FROM user' testdata/user.csv"
Sleep 600ms
Enter
Sleep 2s
Type "sqly --sql 'SELECT user_name, position FROM user JOIN identifier ON user.identifier = identifier.id' testdata/user.csv testdata/identifier.csv"
Sleep 600ms
Enter
Sleep 3s
