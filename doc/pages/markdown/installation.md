### Supported OS & go version

The following OS and Go versions are supported. The sqly is likely to work on BSD as well. However, due to the time-consuming nature of running tests on BSD environments in GitHub Actions, it is not officially supported.

- Windows
- macOS
- Linux
- go1.25.0 or later

### Use "go install"

```shell
go install github.com/nao1215/sqly@latest
```

### Use homebrew

```shell
brew install nao1215/tap/sqly
```

### Use pre-built binaries

The following binaries are distributed on the release page.

- MacOS (darwin_amd64.tar.gz)
- MacOS (darwin_arm64.tar.gz)
- Linux (linux_amd64.tar.gz)
- Linux (linux_arm64.tar.gz)
- Linux (linux_amd64.deb)
- Linux (linux_arm64.deb)
- Linux (linux_amd64.rpm)
- Linux (linux_arm64.rpm)
- Windows (windows_amd64.zip)
- Windows (windows_arm64.zip)

### Verifying release integrity

Every release ships supply-chain metadata so you can verify what you download:

- Signed checksums: `checksums.txt` is signed with [cosign](https://github.com/sigstore/cosign) (keyless), producing `checksums.txt.sigstore.json`.
- SBOM: an SPDX Software Bill of Materials is attached to each release archive.
- Build provenance: SLSA build provenance is attested via GitHub OIDC.

Verify the signed checksums (then check your archive against `checksums.txt`):

```shell
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/nao1215/sqly/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Verify the build provenance of a downloaded artifact with the GitHub CLI:

```shell
gh attestation verify sqly_<version>_<os>_<arch>.tar.gz --repo nao1215/sqly
```
