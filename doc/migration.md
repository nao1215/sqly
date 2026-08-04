# Migration guide

What to change when moving between sqly releases. Only the versions that broke
something are listed; a version not named here needs nothing done.

The [CHANGELOG](../CHANGELOG.md) records every change. This file records the ones
that require an edit to a command line, a script, or a program that reads sqly's
output.

## v1.0.0-rc2 → v1.0.0-rc3

Two defaults changed, in the same direction and for the same reason: sqly is run
by wrappers, CI jobs, and LLM agents as often as by people, and both defaults
did something a caller had not asked for. Neither is a silent change — the old
command now fails loudly rather than behaving differently.

### Remote input needs `--allow-remote`

sqly no longer downloads an `http(s)` URL unless the session was given the
capability.

```text
Remote input before rc3:
  sqly --sql "..." https://example.com/data.csv

Remote input from rc3:
  sqly --allow-remote --sql "..." https://example.com/data.csv
```

**What to change:** add `--allow-remote` to any invocation that names a URL, and
to any session whose `.import` names one.

**What happens if you do not:** the run exits `2` with a message naming the flag,
before any HTTP request is made. Nothing is imported, no temporary file is
created, and stdout stays empty. A command that used to work now fails; it does
not silently do something else.

This covers every way a URL reaches sqly:

| Where the URL is | Needs the flag |
|:--|:--|
| a positional argument to any run mode | yes |
| a positional argument with `--inspect`, `--sql-file`, or `--script-file` | yes |
| `.import URL` at the interactive prompt | yes, granted when the session started |
| `.import URL` in a piped script or a `--script-file` | yes |

A session started with `--allow-remote` keeps the capability for the `.import`
commands typed later in it. Passing the flag on a run that has no URL is not an
error, so a wrapper can pass it unconditionally.

`--allow-remote` is an explicit network capability, **not a sandbox and not an
SSRF defense**. It decides whether sqly makes a request, not where the request
may go: with the flag given, sqly fetches `localhost`, a private range, or a
cloud metadata endpoint exactly as it fetches anything else. What it gives you is
a way for a wrapper that fixes sqly's argument list to turn sqly's downloading
off. It is no defense against a caller that can add flags itself. See
[Remote inputs](https://nao1215.github.io/sqly/formats/#remote-inputs).

The flag does not lift any existing limit: `http` and `https` only, five
redirects, the redirect-scheme check, the header and transfer timeouts, and the
2 GiB response body cap all still apply.

### `--inspect` is schema-only by default

```text
Inspect before rc3:
  --inspect included five sample rows by default.

Inspect from rc3:
  --inspect is schema-only by default.
  Use --inspect-sample N to include row data.
```

**What to change:** a caller that relied on `sample_rows` being populated must
pass `--inspect-sample N` with the number of rows it wants.

**What happens if you do not:** `sample_rows` is `[]`. It is still present, still
an array, and never `null`, so a consumer that iterates it sees zero rows rather
than failing on a missing key.

Everything else in the report is unchanged: table names, sources, row counts,
columns, and the `excel_sheets` array all mean what they meant in rc2.

A negative `--inspect-sample` is now rejected while the command line is parsed,
so it exits `2` before anything is read instead of exiting `1` after the import.
`--inspect-sample` without `--inspect` already exited `2` and still does.

### `--inspect` gained two top-level fields

```json
{
  "schema_version": 1,
  "sqly_version": "v1.0.0-rc3",
  "tables": []
}
```

These are additive: a consumer reading only `tables` is unaffected.

- `schema_version` is a JSON **number**. Branch on it. It is `1`, and it changes
  only for a change a consumer cannot absorb by ignoring unknown fields.
- `sqly_version` is a JSON **string**, the same one `sqly --version` prints.
  Report it; do not branch on it.

The two are easy to confuse and are not interchangeable. The compatibility
policy, and the formal
[JSON Schema](https://nao1215.github.io/sqly/schema/inspect-v1.schema.json), are
on the [reference page](https://nao1215.github.io/sqly/reference/#inspect-json-schema).

Because `sqly_version` moves between releases, the report's bytes are not stable
across versions. What is stable is that the same binary, given the same inputs
and the same options, produces the same bytes.

### A non-SQLite `--dialect` says so on stderr

Choosing `mysql`, `postgresql`, or `googlesql` now prints one line to stderr:

```text
Warning: PostgreSQL syntax is translated to SQLite; execution uses SQLite semantics, not PostgreSQL semantics.
```

**What to change:** most likely nothing. It goes to stderr, once per session, so
stdout — JSON, NDJSON, CSV, TSV, and the `--inspect` report — is untouched and
exit codes are unchanged.

**Check if:** you have a wrapper that treats any stderr output as a failure, or
that compares stderr against a fixture. Those need to allow the line.

It is printed at most once per session: before the first statement of a
`--dialect` run, or at the moment `.dialect` switches to a non-SQLite dialect in
the shell. Switching back to SQLite and out again does not repeat it. `sqlite`,
`--help`, `--version`, `--inspect`, and a rejected command line print nothing.

## Earlier versions

See the Migration Notes sections in the [CHANGELOG](../CHANGELOG.md).
