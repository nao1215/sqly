# --stdin + --sql-file demo: piped dataset queried by a script read from a file.
# Regenerate with: vhs doc/vhs/sql_file.tape  (run from the repository root)
Output "doc/img/sql-file-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 320
Set Padding 18
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "cat testdata/user.csv | sqly --stdin csv --sql-file doc/vhs/join.sql testdata/identifier.csv"
Sleep 600ms
Enter
Sleep 3s
