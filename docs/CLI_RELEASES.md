# CLI Releases

This document is the in-repo contract for cross-platform `mbr` CLI releases.

## Artifact Set

Every tagged CLI release publishes:

- macOS archives:
  - `mbr_VERSION_darwin_amd64.tar.gz`
  - `mbr_VERSION_darwin_arm64.tar.gz`
- Linux archives:
  - `mbr_VERSION_linux_amd64.tar.gz`
  - `mbr_VERSION_linux_arm64.tar.gz`
- Windows archives:
  - `mbr_VERSION_windows_amd64.zip`
  - `mbr_VERSION_windows_arm64.zip`
- `checksums.txt`
- `release-manifest.json`
- Sigstore materials for every published archive and metadata file:
  - `FILE.sig`
  - `FILE.pem`

## Build Paths

- Local packaging:
  - `make cli-release-local VERSION=v1.2.3`
  - or `bash scripts/build-cli-release.sh --version v1.2.3 --out dist/cli-release`
- GitHub release automation:
  - [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml)

The local script builds the full archive matrix, writes `checksums.txt`, and
emits `release-manifest.json`. The GitHub workflow adds Sigstore signatures and
publishes release attachments on tag pushes.

## Verification

Checksum verification:

```bash
cd dist/cli-release
shasum -a 256 -c checksums.txt
```

Signature verification for a tagged GitHub release:

```bash
cosign verify-blob \
  --certificate mbr_v1.2.3_linux_amd64.tar.gz.pem \
  --signature mbr_v1.2.3_linux_amd64.tar.gz.sig \
  --certificate-identity-regexp 'https://github.com/MoveBigRocks/platform/.github/workflows/cli-release.yml@refs/tags/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  mbr_v1.2.3_linux_amd64.tar.gz
```

## Why This Exists

Milestone 1 claims a real operator and agent CLI surface. That claim needs
artifact proof, not only commands and tests. This document, the build script,
and the GitHub workflow are the repo-local evidence chain for that release
story.
