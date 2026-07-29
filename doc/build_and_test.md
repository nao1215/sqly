### Prerequisites

- Go 1.22 or later
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
| [google/wire](https://github.com/google/wire) | Compile-time Dependency Injection for Go |
| [charmbracelet/vhs](https://github.com/charmbracelet/vhs) | Write terminal GIFs as code for integration testing and demoing your CLI tools. |
| [nikolaydubina/go-cover-treemap](https://github.com/nikolaydubina/go-cover-treemap) | Go code coverage to SVG treemap |
| [golangci/golangci-lint](https://github.com/golangci/golangci-lint) | Linters Runner for Go |
| [mock/mockgen](https://github.com/uber-go/mock) | Mocking framework for the Go programming language |
| [fe3dback/go-arch-lint](https://github.com/fe3dback/go-arch-lint) | GoLang architecture linter |



### Build & Test
```shell
$ make build
$ make test
```

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
