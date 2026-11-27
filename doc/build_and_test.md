### Prerequisites

- Go 1.26.11 or later (`go.mod` is the source of truth for the minimum)
- `make`, `git` command

### Install tools for development

Firstly, you need to install the following tools for development. So, you clone the repository and install the tools.

```shell
git clone git@github.com:nao1215/sqly.git
cd sqly
```

If you execute the following command, the tools are installed.  The tools are used for linting, formatting, and testing.

```shell
make tools
```

| Tool | Description |
| :--- | :--- |
| [Songmu/ghch](https://github.com/Songmu/ghch) | Generate changelog from git history, tags and merged pull requests |
| [charmbracelet/vhs](https://github.com/charmbracelet/vhs) | Write terminal GIFs as code for integration testing and demoing your CLI tools. |
| [golangci/golangci-lint](https://github.com/golangci/golangci-lint) | Linters Runner for Go |
| [mock/mockgen](https://github.com/uber-go/mock) | Mocking framework for the Go programming language |
| [fe3dback/go-arch-lint](https://github.com/fe3dback/go-arch-lint) | GoLang architecture linter |

`make tools` installs exactly the table above. It does not install
[atago](https://github.com/nao1215/atago), the end-to-end runner, because the
E2E suite runs the built `sqly` binary rather than linking a library; install it
separately with the version CONTRIBUTING.md pins, which is the one CI uses:

```shell
$ go install github.com/nao1215/atago@v0.26.3
```

### Build & Test
```shell
$ make build
$ make test
```

`make test` runs the unit tests with coverage. The end-to-end suite is separate:

`make test-e2e` runs the atago specs in `e2e/atago/` against the built binary,
and `make smoke` runs the pure-Go binary smoke tests, which are portable to
Windows:

```shell
$ make test-e2e
$ make smoke
```

CI also runs the unit tests under the race detector, and as `GOARCH=386`, where
`int` is 32 bits. The 386 job leaves out the packages that import
github.com/moov-io/ach, which does not compile for 386; the release ships only
64-bit binaries.

```shell
$ go test -race ./...
$ GOARCH=386 go test ./config ./domain/... ./infrastructure ./infrastructure/memory ./infrastructure/persistence
```

`make coverage` measures the unit and E2E tests together. The Coverage workflow
fails a pull request that lowers the total or takes it below 94%.

### Generated inputs

The property-based, metamorphic and differential tests draw their inputs with
[rapid](https://pkg.go.dev/pgregory.net/rapid), a hundred per test in `make test`.
`main_property_test.go` writes one generated table as CSV, TSV, LTSV and
gzip-compressed CSV and checks that sqly answers the same query the same way
from each (metamorphic), and that what sqly prints is what the database filesql
loads from the file answers through `database/sql` (differential). Draw more, or
replay a failure from the seed it prints:

```shell
$ go test . -run '^Test(Metamorphic|Differential)_' -rapid.checks=5000
$ go test . -run '^TestMetamorphic_' -rapid.seed=<seed>
```

The fuzz targets (`FuzzQuoteRoundTrip` and the others under `shell/` and
`infrastructure/`) run their seed corpus in `make test`. The Fuzz workflow
searches each one for 15 seconds on every pull request that changes Go code
or a fuzz corpus, and for five minutes every night:

```shell
$ go test ./shell -run '^$' -fuzz '^FuzzSplitSQLStatements$' -fuzztime 1m
```

### Generated code

`make generate` runs `go generate ./...`, which today means one generator: the
`//go:generate mockgen` lines in `domain/repository/` and `usecase/` that produce
the gomock doubles under `infrastructure/mock/` and `interactor/mock/`. Run it
after changing one of those interfaces and commit the regenerated files; the
CheckAutoGenerateFiles workflow fails when they are out of date.

Application wiring is not generated. `di/di.go` is a hand-written composition
root; see CONTRIBUTING.md for what to do when you add a constructor.

### Finding code nothing reaches

Removing a feature tends to leave a function whose last caller went with it, and
tests keep such a function compiling and covered, so nothing else notices:

```shell
$ go run golang.org/x/tools/cmd/deadcode@latest ./...
```

Findings under `testutil/` are expected, because deadcode does not see test
binaries. A finding anywhere else is a function to delete or a caller to
restore. It is not part of `make lint`, since it is a whole-program analysis run
occasionally rather than a gate on every change.

### Demo GIFs

The README and GitHub Pages embed demo GIFs under `doc/img/`, each rendered from a
matching `doc/vhs/*.tape` by [vhs](https://github.com/charmbracelet/vhs). Rerun
`make demo` after you change a tape, add a new demo, or change a documented
workflow whose GIF should reflect it:

```shell
$ make demo
```

`make demo` needs vhs, ttyd, and ffmpeg, so it is not run in CI. Instead, the
`TestDemoAssetsInSync` docs-sync test guards against drift without rendering: it
fails when a tape declares an `Output` GIF that does not exist (a tape changed or
added without `make demo`), or when the README embeds a GIF that no tape produces.
Commit the regenerated GIF together with the tape change so this check stays green.

Not every workflow has its own GIF. The `--sql-file --output` workflow is
intentionally not given one: its result goes to a file rather than the terminal,
so there is nothing visually distinct to capture beyond the existing `--sql-file`
and `--output` demos. Add a tape only when a workflow has a meaningful on-screen
result.

### Mutation testing

[gremlins](https://github.com/go-gremlins/gremlins) v0.6.0 changes one
comparison, bound or arithmetic operator at a time and runs the package's tests;
a mutant no test notices is a line the tests execute without checking it. The
Mutation workflow mutates the lines a pull request changes, and every package
once a week in one job per package:

```shell
$ gremlins unleash --workers 4 --timeout-coefficient 10 --exclude-files / ./shell
$ git config diff.context 0   # one hunk per edit, so --diff sees every changed line
$ gremlins unleash --diff origin/main --workers 4 .
```

`--exclude-files /` keeps a run to the one directory it is given. Run `--diff`
from the module root, where the paths git prints match the files gremlins mutates.

Efficacy (killed / (killed + lived)) on 2026-11-27:

| Package | Killed | Lived | Efficacy |
|---|---|---|---|
| `shell` | 923 | 101 | 90.14% |
| `domain/model` | 172 | 17 | 91.01% |
| `domain/sqltext` | 93 | 1 | 98.94% |
| `config` | 79 | 6 | 92.94% |
| `infrastructure/filesql` | 51 | 4 | 92.73% |
| `infrastructure/memory` | 61 | 0 | 100% |
| `infrastructure/persistence` | 49 | 5 | 90.74% |
| `infrastructure` | 15 | 1 | 93.75% |

The root package, `di`, `domain/cleanup` and `interactor` hold fewer than
twenty mutants each and none lived. `.gremlins.yaml` fails a run at 90% or
less: a pull request fails when one mutant in ten of its changed lines lives,
and a weekly job fails when its package falls to 90%. Most of
the mutants still alive are equivalent: a bound that cannot be reached, or a
guard whose other branch gives the same result.

The workflow sets `GOFLAGS=-count=1`. setup-go restores the test cache, and a
cached test run takes milliseconds, so without it every mutant times out and the
efficacy reads 0%.
