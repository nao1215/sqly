# HTTP import demo: query a remote CSV and then pull the same URL into .import.
# Regenerate with: vhs doc/vhs/http.tape  (run from the repository root)
Output "doc/img/http-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 620
Set Padding 18
Set CursorBlink false
Set Framerate 24
Set TypingSpeed 60ms
Hide
Type "WORK=$(mktemp -d); cp testdata/user.csv $WORK/; python3 -m http.server 18080 --bind 127.0.0.1 --directory $WORK >/tmp/sqly-http-demo.log 2>&1 & HTTP_PID=$!; trap 'kill $HTTP_PID' EXIT; export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Type "until curl -fsS http://127.0.0.1:18080/user.csv >/dev/null; do sleep 0.1; done"
Enter
Show
Type "sqly --csv --sql 'SELECT user_name FROM user ORDER BY identifier LIMIT 2' http://127.0.0.1:18080/user.csv"
Enter
Sleep 2s
Type "sqly"
Enter
Sleep 1600ms
Type ".import http://127.0.0.1:18080/user.csv"
Enter
Sleep 2s
Type "SELECT COUNT(*) AS c FROM user;"
Enter
Sleep 2s
Type ".exit"
Enter
Sleep 1s
