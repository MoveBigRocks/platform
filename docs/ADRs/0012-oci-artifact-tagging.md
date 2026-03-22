# ADR 0012: OCI Artifact Tagging and Cross-Referencing

**Status:** Accepted
**Date:** 2026-01-02

## Context

Move Big Rocks publishes Go binaries and related release assets as OCI artifacts to
GitHub Container Registry using ORAS.

Release artifacts need bidirectional cross-reference between git tags and OCI
artifacts:

- from a git tag, find the exact artifact digests
- from an OCI tag, understand which release it belongs to
- support both immutable references and human-friendly version references

## Decision

Implement bidirectional cross-referencing between git tags and OCI artifacts.

### Release Model: One Tag, Multiple Artifacts

A single release version such as `v1.2.3` encompasses all deployment artifacts
built from one commit:

| Artifact | Repository | Unique Digest | Shared Tags |
|----------|------------|---------------|-------------|
| Services binary | `mbr-services` | `sha256:aaa...` | `v1.2.3`, `:abc1234` |
| Migrations | `mbr-migrations` | `sha256:bbb...` | `v1.2.3`, `:abc1234` |
| Release manifest | `mbr-manifest` | `sha256:ccc...` | `v1.2.3`, `:abc1234` |

Key points:

- all three artifacts share the same commit SHA and semver tag
- each artifact has its own unique content digest
- the release manifest contains the digests needed to pin the core release
- git tag annotations record all three digests for auditability
- deploy scripts remain owned by the private instance repo rather than core

### 1. Git Tag Annotation Includes OCI Digests

When creating a git tag, include the artifact digests in the annotation:

```text
Release v1.2.3

Commit: abc1234def5678...
Build Date: 2026-01-02T10:30:00Z

OCI Artifacts (ghcr.io/movebigrocks/mbr-*):
  services:    @sha256:abc123...
  migrations:  @sha256:def456...
  manifest:    @sha256:789ghi...

Pull with: oras pull ghcr.io/movebigrocks/mbr-manifest:v1.2.3
```

### 2. OCI Artifacts Get Semver Tags

After git tag creation, tag OCI artifacts with the same semver:

```bash
oras tag ghcr.io/movebigrocks/mbr-services:<sha> v1.2.3
oras tag ghcr.io/movebigrocks/mbr-migrations:<sha> v1.2.3
oras tag ghcr.io/movebigrocks/mbr-manifest:<sha> v1.2.3
```

### 3. Tag Hierarchy

Each artifact ends up with multiple tags pointing to the same digest:

| Tag Type | Example | Purpose |
|----------|---------|---------|
| **SHA** | `:abc1234` | Immutable, for CI/CD pipelines |
| **Semver** | `:v1.2.3` | Human-friendly, for documentation |
| **Environment** | `:production-latest` | Deployed environment pointer |

### Workflow

```text
                    Build Job                          Version Job
                        │                                   │
    ┌───────────────────┴───────────────────┐              │
    │  Push artifacts with SHA tag          │              │
    │  ghcr.io/.../services:abc1234         │              │
    │  ghcr.io/.../manifest:abc1234         │              │
    │                                       │              │
    │  Tag with environment-latest          │              │
    │  ghcr.io/.../services:production-latest │            │
    └───────────────────────────────────────┘              │
                        │                                   │
                        ▼                                   │
                 Deploy + Verify                           │
                        │                                   │
                        ▼                                   │
              ┌─────────────────────────────────────────────┴──────┐
              │  1. Fetch manifest to get all digests              │
              │  2. Calculate semver (v1.2.3)                      │
              │  3. Create annotated git tag with digest info      │
              │  4. Tag OCI artifacts with semver                  │
              └────────────────────────────────────────────────────┘
```

## Consequences

### Positive

- full traceability from git history to exact deployed artifacts
- human-readable tags in the registry
- debugging can start from either git history or the registry
- git tag annotations provide a durable audit trail
- rollback is straightforward through semver or digest references

### Negative

- CI is slightly longer because it tags artifacts after verification
- more tags exist per artifact in the registry
- git tag creation requires registry access

## References

- `.github/workflows/production.yml` - version job implementation
- `.github/workflows/_build.yml` - build job that creates initial artifacts
- [ORAS documentation](https://oras.land/) - OCI artifact tooling
