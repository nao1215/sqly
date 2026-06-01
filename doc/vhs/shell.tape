# Interactive shell demo: dot commands, a query, and switching output mode.
# Regenerate with: vhs doc/vhs/shell.tape  (run from the repository root)
Output "doc/img/shell-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 640
Set Padding 18
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "sqly testdata/user.csv"
Enter
Sleep 1800ms
Type ".tables"
Enter
Sleep 1200ms
Type ".schema user"
Enter
Sleep 1500ms
Type "SELECT user_name, first_name FROM user"
Enter
Sleep 2s
Type ".mode json"
Enter
Sleep 800ms
Type "SELECT user_name FROM user LIMIT 2"
Enter
Sleep 2s
Type ".exit"
Enter
Sleep 1s
