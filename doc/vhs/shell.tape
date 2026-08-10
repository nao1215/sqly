# Interactive shell demo: dot commands, a multi-line statement, and switching
# output mode. The session runs in a throwaway HOME so the prompt shows
# "sqly:~/data(table)$", the same session the website documents.
# Regenerate with: vhs doc/vhs/shell.tape  (run from the repository root)
Output "doc/img/shell-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 880
Set Padding 18
Set CursorBlink false
Set Framerate 24
Set TypingSpeed 60ms
Hide
Type "export PATH=$PWD:$PATH PS1='$ ' HOME=$(mktemp -d); mkdir -p $HOME/data; cp testdata/user.csv $HOME/data/; cd $HOME/data; clear"
Enter
Show
Type "sqly user.csv"
Enter
Sleep 1800ms
Type ".tables"
Enter
Sleep 1200ms
Type ".schema user"
Enter
Sleep 1500ms
Type "SELECT user_name, first_name FROM user;"
Enter
Sleep 2s
Type "SELECT user_name FROM user"
Enter
Sleep 800ms
Type "WHERE identifier = 1;"
Enter
Sleep 2s
Type ".mode json"
Enter
Sleep 800ms
Type "SELECT user_name FROM user LIMIT 2;"
Enter
Sleep 2s
Type ".exit"
Enter
Sleep 1s
