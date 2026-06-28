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
sqly has a [ShellSpec](https://github.com/shellspec/shellspec) suite that
exercises the real `sqly` binary. It runs in CI
(`.github/workflows/e2e_test.yml`).

```shell
# Install ShellSpec once (see https://github.com/shellspec/shellspec#installation)
curl -fsSL https://git.io/shellspec | sh -s 0.28.1 --yes

# Build the binary and run the suite
make test-e2e
```

### 5. Regenerate code when you touch DI or templates
sqly uses Google Wire for dependency injection. After changing `di/wire.go` or
anything covered by `go:generate`, regenerate and verify:

```shell
make generate
```

### 6. Install developer tools
```shell
make tools
```

## Documentation
`README.md` (English) is the source of truth for user-facing documentation. When
you add or change a feature, also update the GitHub Pages docs under
`doc/pages/markdown/`. Avoid bold and emoji in documentation. Localized READMEs
have been removed; please do not add new ones.

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
