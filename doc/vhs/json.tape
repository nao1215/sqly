# JSON demo: query a JSONL file with json_extract (rows land in a "data" column).
# Regenerate with: vhs doc/vhs/json.tape  (run from the repository root)
Output "doc/img/json-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 520
Set Padding 18
Set CursorBlink false
Set Framerate 24
Hide
Type "WORK=$(mktemp -d); cp doc/vhs/json.sql testdata/sample.jsonl $WORK/; export PATH=$PWD:$PATH; cd $WORK; export PS1='$ '; clear"
Enter
Show
Type "cat sample.jsonl"
Sleep 600ms
Enter
Sleep 2s
Type "cat json.sql"
Sleep 600ms
Enter
Sleep 2s
Type "sqly --sql-file json.sql sample.jsonl"
Sleep 600ms
Enter
Sleep 3500ms
