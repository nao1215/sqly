# Mixed-format demo: read a gzipped CSV transparently, query Parquet, and JOIN a
# compressed CSV with a plain CSV in one query.
# Regenerate with: vhs doc/vhs/mixed.tape  (run from the repository root)
Output "doc/img/mixed-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1150
Set Height 560
Set Padding 18
Set CursorBlink false
Set Framerate 24
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'SELECT name, price FROM products ORDER BY CAST(price AS REAL) DESC' testdata/products.parquet"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'SELECT * FROM user' testdata/user.csv.gz"
Sleep 600ms
Enter
Sleep 2500ms
Type "sqly --sql 'SELECT user_name, position FROM user JOIN identifier ON user.identifier = identifier.id' testdata/user.csv.gz testdata/identifier.csv"
Sleep 600ms
Enter
Sleep 3500ms
