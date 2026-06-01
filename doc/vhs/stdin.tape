# Piped stdin demo: treat stdin as a dataset and JOIN it with a file.
# Regenerate with: vhs doc/vhs/stdin.tape  (run from the repository root)
Output "doc/img/stdin-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 380
Set Padding 18
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "cat testdata/user.csv | sqly --stdin csv --sql 'SELECT user_name FROM stdin LIMIT 1'"
Sleep 600ms
Enter
Sleep 2s
Type "cat testdata/user.csv | sqly --stdin csv --sql 'SELECT s.user_name, i.position FROM stdin s JOIN identifier i ON s.identifier = i.id' testdata/identifier.csv"
Sleep 600ms
Enter
Sleep 3s
