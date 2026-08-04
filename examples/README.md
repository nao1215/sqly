# Examples

Two files you can run against a clone of this repository without editing
anything. They are the same two the [README](../README.md) and the
[cookbook](https://nao1215.github.io/sqly/cookbook/) point at, and the E2E suite
runs them, so what is shown here is what the binary does.

Run them from the repository root. Every path is relative, so the commands work
the same on Linux, macOS, and Windows.

## report.sql — SQL only

```shell
sqly --sql-file examples/report.sql examples/data/sales.csv
```

`--sql-file` takes SQL and nothing else. The result is printed and nothing on
disk changes.

```text
+--------+-------+
| region | total |
+--------+-------+
| EMEA   |  2650 |
| AMER   |  2400 |
| ASIA   |  1650 |
+--------+-------+
```

Send it somewhere instead of the terminal:

```shell
sqly --output-format csv --sql-file examples/report.sql --output report.csv examples/data/sales.csv
```

## update.sqly — SQL and dot-commands

```shell
sqly --script-file examples/update.sqly examples/data/sales.csv
```

`--script-file` takes what the shell takes, so the script can do things SQL has
no syntax for. This one renames a region and then writes the changed table out:

```text
UPDATE sales SET region = 'APAC' WHERE region = 'ASIA';
.save ./out
```

`.save ./out` writes into an `out` directory **relative to the working
directory you ran sqly from**, not relative to `examples/`. Running the command
above from the repository root leaves `./out/sales.csv` there; delete it when
you are done.

`examples/data/sales.csv` is not touched. `.save DIR` writes copies and leaves
the sources alone — overwriting them is `.save --in-place`, which this example
deliberately does not do.

Putting the same script in `--sql-file` is a usage error (exit 2), because a
`.sql` file that quietly ran `.save` would be a shell script wearing a SQL
extension. `--script-file` also rejects `--output`: a script can print several
results and take several actions, and one destination cannot carry that. Use
`.dump` inside the script when you need a file.
