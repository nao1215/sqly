# Analytics demo: a real SQL script (CTE + window function + GROUP BY) over a
# 50-row dataset, loaded from a file with --sql-file.
# Regenerate with: vhs doc/vhs/analytics.tape  (run from the repository root)
Output "doc/img/analytics-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 1000
Set Padding 18
Set CursorBlink false
Set Framerate 24
Hide
Type "WORK=$(mktemp -d); cp doc/vhs/analytics.sql testdata/actor.csv $WORK/; export PATH=$PWD:$PATH; cd $WORK; export PS1='$ '; clear"
Enter
Show
Type "cat analytics.sql"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql-file analytics.sql actor.csv"
Sleep 600ms
Enter
Sleep 4s
