# Cross-format JOIN demo: join a Parquet table to a CSV table in one SQL query
# and compute revenue on the fly.
# Regenerate with: vhs doc/vhs/crossjoin.tape  (run from the repository root)
Output "doc/img/crossjoin-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1280
Set Height 300
Set Padding 18
Set CursorBlink false
Set Framerate 24
Hide
Type "export PATH=$PWD:$PATH PS1='$ '; clear"
Enter
Show
Type "sqly --sql 'SELECT p.name, p.price, s.quantity, ROUND(p.price * s.quantity, 2) AS revenue FROM products p JOIN sales s ON p.id = s.product_id ORDER BY revenue DESC' testdata/products.parquet testdata/sales.csv"
Sleep 800ms
Enter
Sleep 4000ms
