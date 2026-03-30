# Architecture Decision Records

ADRs capture durable architectural decisions for Move Big Rocks, the AI-native service
operations platform.

## ADR Index

### Process

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-use-architecture-decision-records.md) | Use Architecture Decision Records | Accepted |

### Security

| ADR | Title | Status |
|-----|-------|--------|
| [0002](0002-security-architecture.md) | Security Architecture | Accepted |
| [0003](0003-multi-tenant-isolation.md) | Multi-Tenant Isolation | Accepted |
| [0008](0008-session-token-security.md) | Session Token Security | Accepted |
| [0017](0017-magic-login-honeypot-telemetry.md) | Magic Login Honeypot Telemetry | Proposed |

### Infrastructure

| ADR | Title | Status |
|-----|-------|--------|
| [0005](0005-event-driven-architecture.md) | Event-Driven Architecture | Accepted |
| [0007](0007-s3-attachments.md) | S3 Attachments with Virus Scanning | Accepted |
| [0012](0012-oci-artifact-tagging.md) | OCI Artifact Tagging and Cross-Referencing | Accepted |
| [0013](0013-server-security-hardening.md) | Server Security Hardening | Accepted |
| [0014](0014-blue-green-deployment.md) | Blue-Green Deployment | Accepted |
| [0019](0019-embed-static-assets.md) | Embed Static Assets and Schemas in Binary | Accepted |
| [0020](0020-geoip-location-service.md) | GeoIP Location Service | Accepted |
| [0021](0021-postgresql-only-runtime-and-extension-schemas.md) | PostgreSQL-Only Runtime and Extension-Owned Schemas | Accepted |
| [0022](0022-postgresql-native-uuidv7-row-ids-and-public-identifiers.md) | PostgreSQL Native UUIDv7 Row IDs and Public Identifiers | Accepted |
| [0023](0023-core-postgresql-bounded-context-schemas.md) | Core PostgreSQL Bounded-Context Schemas | Accepted |
| [0024](0024-postgresql-migration-ledgers-and-identifier-ownership.md) | PostgreSQL Migration Ledgers and Identifier Ownership | Accepted |
| [0025](0025-pre-production-postgresql-baseline-reset.md) | Pre-Production PostgreSQL Baseline Reset | Accepted |

### Code Architecture

| ADR | Title | Status |
|-----|-------|--------|
| [0006](0006-bounded-context-structure.md) | Bounded Context Package Structure | Accepted |
| [0009](0009-code-architecture.md) | Code Architecture Patterns | Accepted |

### API and Extensions

| ADR | Title | Status |
|-----|-------|--------|
| [0010](0010-agent-api-and-graphql-architecture.md) | Agent API and GraphQL Architecture | Accepted |
| [0015](0015-workspace-scoped-agent-access.md) | Workspace-Scoped Agent Access | Accepted |
| [0016](0016-cli-and-agent-authentication.md) | CLI and Agent Authentication Guidelines | Accepted |
| [0018](0018-api-interface-versioning.md) | API Interface and Versioning Strategy | Accepted |
| [0026](0026-extension-host-lifecycle-and-public-extension-sdk-boundary.md) | Extension Host Lifecycle and Public Extension SDK Boundary | Proposed |
