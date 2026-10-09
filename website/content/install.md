---
title: Install
description: Install sqly with go install, Homebrew, the AUR, aqua, mise, or a prebuilt binary, and verify the release signature.
weight: 10
---

Runs on Linux, macOS, and Windows. Building from source needs Go 1.26.9 or later.

## go install

```shell
go install github.com/nao1215/sqly@latest
```

## Homebrew

```shell
brew install nao1215/tap/sqly
```

## Arch Linux (AUR)

```shell
yay -S sqly-bin
```

Without an AUR helper:

```shell
git clone https://aur.archlinux.org/sqly-bin.git
cd sqly-bin
makepkg -si
```

## aqua

sqly is in the [aqua](https://aquaproj.github.io/) standard registry:

```shell
aqua g -i nao1215/sqly
```

## mise

[mise](https://mise.jdx.dev/) installs it through the same registry with the aqua backend:

```shell
mise use aqua:nao1215/sqly
```

## Prebuilt binaries

The [release page](https://github.com/nao1215/sqly/releases) has archives for Linux, macOS, and Windows.

## Verify a release

Every release ships supply-chain metadata:

- `checksums.txt` is signed with [cosign](https://github.com/sigstore/cosign) (keyless), producing `checksums.txt.sigstore.json`.
- An SPDX SBOM is attached to each release archive.
- SLSA build provenance is attested via GitHub OIDC and attached to the release as `multiple.intoto.jsonl`.

Verify the signed checksums, then your archive against them:

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/sqly/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Verify build provenance with the GitHub CLI:

```shell
gh attestation verify sqly_<version>_<os>_<arch>.tar.gz --repo nao1215/sqly
```

Or download the `multiple.intoto.jsonl` release asset and verify the archive against it with [slsa-verifier](https://github.com/slsa-framework/slsa-verifier), which checks the provenance signature against the Sigstore transparency log:

```shell
slsa-verifier verify-artifact sqly_<version>_<os>_<arch>.tar.gz \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/nao1215/sqly \
  --source-tag v<version>
```
