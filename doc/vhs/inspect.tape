# --inspect demo: machine-readable JSON report, with and without sample rows.
# Regenerate with: vhs doc/vhs/inspect.tape  (run from the repository root)
Output "doc/img/inspect-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 560
Set Padding 18
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "sqly --inspect --inspect-sample 1 testdata/identifier.csv"
Sleep 600ms
Enter
Sleep 3s
Type "sqly --inspect --inspect-sample 0 testdata/identifier.csv"
Sleep 600ms
Enter
Sleep 3s
