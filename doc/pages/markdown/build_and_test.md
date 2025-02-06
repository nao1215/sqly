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
make install tools
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
