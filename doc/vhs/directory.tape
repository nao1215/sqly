# Directory-import demo: point sqly at a folder of mixed-format files and join
# across them in one query.
# Regenerate with: vhs doc/vhs/directory.tape  (run from the repository root)
Output "doc/img/directory-demo.gif"
Set Shell "bash"
Set FontSize 16
Set Width 1320
Set Height 360
Set Padding 18
Set CursorBlink false
Set Framerate 24
Hide
Type "WORK=$(mktemp -d); mkdir $WORK/shop; cp testdata/products.parquet testdata/sales.csv $WORK/shop/; export PATH=$PWD:$PATH; cd $WORK; export PS1='$ '; clear"
Enter
Show
Type "ls shop/"
Sleep 600ms
Enter
Sleep 2000ms
Type "sqly ./shop --sql 'SELECT p.name, s.quantity FROM products p JOIN sales s ON p.id = s.product_id ORDER BY p.name'"
Sleep 800ms
Enter
Sleep 4000ms
