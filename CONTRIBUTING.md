## Contributing to sqly
Thank you for building sqly with us.
Every report, patch, test, and review helps people query CSV/TSV/LTSV/JSON and
Excel files with SQL more easily. Let's keep sqly fast, safe, and reliable
together.

## Contributing as a Developer
### 1. Start with clear communication
- Bug report: Use the issue template and include reproducible steps, the input
  file format, expected behavior, and actual behavior.
- New feature: Open an issue first so we can agree on direction before
  implementation.
- Bug fix or improvement: Open a PR with a clear problem statement and solution
  summary.

### 2. Keep the quality bar high
- Add or update unit tests when you add features or fix bugs.
- Avoid regressions on supported OSes (Linux, macOS, Windows).
- Keep CLI and shell behavior and error messages clear and consistent.
- sqly follows Clean Architecture; respect the layer boundaries enforced by
  go-arch-lint (`.go-arch-lint.yml`).

### 3. Run checks before opening a PR
```shell
make test
make lint
make build
```

`make test` runs the unit tests with coverage. `make lint` runs golangci-lint
and go-arch-lint. Aim for 80% or higher coverage with unit tests.

### 4. Run the end-to-end tests (recommended for CLI/shell changes)
sqly has an end-to-end suite of plain-YAML [atago](https://github.com/nao1215/atago)
specs (`e2e/atago/`) that exercises the real `sqly` binary. It runs in CI
(`.github/workflows/e2e_test.yml`).

```shell
# Install atago once. CI pins the same version in .github/workflows/e2e_test.yml,
# and TestContributing_PinsTheAtagoVersionCIInstalls fails when the two drift.
go install github.com/nao1215/atago@v0.21.0

# Build the binary and run the suite
make test-e2e
```

atago is not part of `make tools`, because the suite runs the shipped binary
rather than building against a library; install it with the command above.

There is also a pure-Go binary smoke harness that runs the same way on Linux,
macOS, and Windows (handy when a change affects path handling or startup):

```shell
make smoke
```

### 5. Wiring the application together
`di/di.go` is sqly's composition root, written by hand: it calls each
constructor in dependency order and returns the shell together with the function
that closes the databases it opened. There is no code generation behind it, so a
new constructor is added by editing `di.NewShell` — and, if it opens something,
by releasing it on the failure paths as well as in the returned cleanup.

`make generate` is unrelated to that. It regenerates the gomock doubles declared
by the `//go:generate mockgen` lines in `domain/repository/` and `usecase/`, so
run it after changing one of those interfaces:

```shell
make generate
```

### 6. Install developer tools
```shell
make tools
```

### 7. Refresh demo GIFs when you change a tape
The README demo GIFs under `doc/img/` are rendered from `doc/vhs/*.tape`. After
changing a tape or adding a demo, rerun `make demo` (it needs vhs, ttyd, and
ffmpeg) and commit the regenerated GIF with the tape change:

```shell
make demo
```

CI does not render GIFs; the `TestDemoAssetsInSync` test instead fails when a tape
and its GIF, or the README and a GIF, fall out of sync.

## Documentation
`README.md` (English) introduces sqly and links out; the site under
`website/content/` is where the detail lives. When you add or change a feature,
update the page it belongs to — `reference.md` for flags and exit codes,
`formats.md` for what sqly reads and writes, `shell.md` for dot-commands — and
the README only if the change is one a first-time reader needs.

The site is Hugo. `make website` builds it into `website/public/`, and a push to
`main` that touches `website/**` deploys it.

Avoid bold and emoji in documentation. Localized READMEs have been removed;
please do not add new ones.

## Releasing
Maintainers cut releases by pushing a `v*` tag. The process is documented in
[doc/RELEASE.md](./doc/RELEASE.md).

## Need help?
See [SUPPORT.md](./.github/SUPPORT.md) for where to ask questions and report
problems.

## Contributing Outside of Coding
You can still make a huge impact even if you are not writing code:

- Give sqly a GitHub Star
- Share sqly with your team and community
- Open issues with clear reproduction steps
- Sponsor the project
