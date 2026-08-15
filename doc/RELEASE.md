# Release process

This describes how a sqly release is cut. It is for maintainers.

## Overview
Releases are driven by Git tags. Pushing a tag that matches `v*` triggers the
[release workflow](../.github/workflows/release.yml), which runs
[GoReleaser](https://goreleaser.com/) using [.goreleaser.yml](../.goreleaser.yml).
There is no manual upload step.

## Versioning
- sqly follows [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.
- Release notes are generated from commit messages, so use
  [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`,
  `perf:`, `docs:`, and `!` for breaking changes). `chore:`, `ci:`, `style:`,
  and `test:` commits are excluded from the notes.

## Before tagging
- Make sure `main` is green (build, unit tests, e2e, lint, gitleaks).
- Locally you can dry-run the build with `goreleaser release --snapshot --clean`.

## Cut a release
```shell
git switch main
git pull --ff-only
git tag vX.Y.Z
git push origin vX.Y.Z
```
The release workflow then:
- builds binaries for linux, macOS, and Windows (amd64 and arm64);
- publishes archives, `deb`/`rpm`/`apk` packages, and `checksums.txt`;
- signs the checksums with cosign (keyless) and attaches an SBOM;
- attests build provenance via GitHub OIDC;
- updates the Homebrew tap (`nao1215/homebrew-tap`).
- pushes the winget manifests to `nao1215/winget-pkgs` and opens the pull
  request against [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs).
  Release-candidate tags are skipped; the community repository takes stable
  versions only.

## Required secrets
- `GITHUB_TOKEN`: provided automatically; used to create the GitHub Release.
- `TAP_GITHUB_TOKEN`: a token with write access to `nao1215/homebrew-tap`,
  used to push the updated formula.
- `WINGET_GITHUB_TOKEN`: a **classic** personal access token with the `public_repo` scope. GoReleaser v2.16.0 uses this one token for both writes: committing the manifests to a branch on `nao1215/winget-pkgs`, the fork of the community repository, and opening the pull request against microsoft/winget-pkgs. A fine-grained token cannot do the second — it can only be scoped to repositories you own, and microsoft/winget-pkgs is not one of them, so the push succeeds and the pull request fails with 403. A failure is logged without failing the release, so a rejected or delayed pull request never blocks a version. If a release finishes but no pull request appears, `dist/winget/manifests/n/nao1215/sqly/<version>/` still holds the three files and they can be submitted by hand from a `sqly-<version>` branch on the fork.

## After releasing
- Check the [Releases page](https://github.com/nao1215/sqly/releases) for the
  generated notes and artifacts.
- Verify a downloaded artifact as described in
  [Verifying release integrity](../README.md#verifying-release-integrity).
- Confirm `brew upgrade sqly` picks up the new version.

## If a release fails
- Re-run the failed job from the Actions tab once the cause is fixed.
- If the tag itself is wrong, delete it locally and remotely, then tag again:
  ```shell
  git tag -d vX.Y.Z
  git push origin :refs/tags/vX.Y.Z
  ```
