### sqly-shell

The sqly shell starts when you run the sqly command without the `--sql` option. When you execute sqly command with file path, the sqly-shell starts after importing the file into the SQLite3 in-memory database. 
The sqly-shell also supports command history, and input completion.  

```shell
$ sqly 
sqly v0.11.0

enter "SQL query" or "sqly command that begins with a dot".
.help print usage, .exit exit sqly.

sqly:~/github/github.com/nao1215/sqly(table)$  .import actor.csv
sqly:~/github/github.com/nao1215/sqly(table)$  .import numeric.csv
sqly:~/github/github.com/nao1215/sqly(table)$  .tables
+------------+
| TABLE NAME |
+------------+
| actor      |
| numeric    |
+------------+

sqly:~/github/github.com/nao1215/sqly(table)$  SELECT actor, best_movie FROM actor LIMIT 3
+-------------------+------------------------------+
|       actor       |          best_movie          |
+-------------------+------------------------------+
| Harrison Ford     | Star Wars: The Force Awakens |
| Samuel L. Jackson | The Avengers                 |
| Morgan Freeman    | The Dark Knight              |
+-------------------+------------------------------+

sqly:~/github/github.com/nao1215/sqly(table)$  .mode ltsv
Change output mode from table to ltsv

sqly:~/github/github.com/nao1215/sqly(ltsv)$  SELECT actor, best_movie FROM actor LIMIT 3
actor:Harrison Ford     best_movie:Star Wars: The Force Awakens
actor:Samuel L. Jackson best_movie:The Avengers
actor:Morgan Freeman    best_movie:The Dark Knight
```


### Key Binding for sqly-shell

|Key Binding	|Description|
|:--|:--|
|Ctrl + A	|Go to the beginning of the line (Home)|
|Ctrl + E	|Go to the end of the line (End)|
|Ctrl + P	|Previous command (Up arrow)|
|Ctrl + N	|Next command (Down arrow)|
|Ctrl + F	|Forward one character|
|Ctrl + B	|Backward one character|
|Ctrl + D	|Delete character under the cursor|
|Ctrl + H	|Delete character before the cursor (Backspace)|
|Ctrl + W	|Cut the word before the cursor to the clipboard|
|Ctrl + K	|Cut the line after the cursor to the clipboard|
|Ctrl + U	|Cut the line before the cursor to the clipboard|
|Ctrl + L	|Clear the screen|
|TAB        |Completion|
|↑          |Previous command|
|↓          |Next command|
