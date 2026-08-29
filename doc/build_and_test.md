### Prerequisites

- Go 1.25 or later (`go.mod` is the source of truth for the minimum)
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
$ go install github.com/nao1215/atago@v0.21.0
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
