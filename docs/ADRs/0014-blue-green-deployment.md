# ADR 0014: Blue-Green Deployment

**Status:** Accepted
**Date:** 2026-01-03

**Related ADRs:**
- [0021](0021-postgresql-only-runtime-and-extension-schemas.md): PostgreSQL-only runtime and extension-owned schemas
- [0013](0013-server-security-hardening.md): Server Security Hardening

## Context

Move Big Rocks deploys with blue-green switching so application releases do not require
an operator-visible outage window.

Constraints:

- single server architecture
- shared PostgreSQL database accessed by both slots via `DATABASE_DSN`
- minimal infrastructure changes
- fit with the existing Caddy reverse proxy

## Decision

Implement blue-green deployment using dual systemd services and Caddy
health-based load balancing.

### Architecture

```
                    ┌─────────────────┐
                    │     Caddy       │
                    │  (Load Balancer)│
                    └────────┬────────┘
                             │
              health-based routing
                             │
         ┌───────────────────┴───────────────────┐
         │                                       │
         ▼                                       ▼
┌─────────────────┐                   ┌─────────────────┐
│  mbr-blue    │                   │  mbr-green   │
│   :8080         │                   │   :8081         │
└────────┬────────┘                   └────────┬────────┘
         │                                     │
         └───────────────┬─────────────────────┘
                         │
                         ▼
              ┌─────────────────┐
              │ PostgreSQL      │
              │ (shared DSN)    │
              └─────────────────┘
```

### Components

**1. Dual Systemd Services**
- `mbr-blue.service` → port 8080
- `mbr-green.service` → port 8081

**2. Caddy Health-Based Routing**

```caddyfile
app.example.com {
    reverse_proxy localhost:8080 localhost:8081 {
        lb_policy first
        health_uri /health
        health_interval 2s
        health_timeout 5s
        fail_duration 10s
    }
}
```

**3. Active Slot Tracking**

File `/opt/mbr/.active-slot` contains `blue` or `green`.

### Deployment Flow

1. Read active slot from marker file
2. Deploy new binary to inactive slot
3. Start inactive slot service
4. Wait for health check to pass
5. Caddy routes to the healthy instance
6. Stop old slot gracefully
7. Update active slot marker

### Migration Safety Rules

All migrations must be deploy-window safe:

| Safe | Unsafe |
|------|--------|
| Add column with default | Rename column |
| Add table | Drop column |
| Add index | Change column type |

## Implementation Notes

### Port Configuration

The `PORT` environment variable is not set in `/opt/mbr/.env`. Each service
controls its own port via systemd `Environment=PORT=` directives.

### Binary Deployment

Deploy binaries to a temporary directory first to avoid `Text file busy`
errors:

```bash
tar -xzf archive.tar.gz -C /tmp/mbr-extract/
cp /tmp/mbr-extract/mbr-server /opt/mbr/mbr-{slot}
```

### Server Provisioning

The `mbr` user is created with:

1. Home directory: `/opt/mbr`
2. Shell: `/bin/bash`
3. SSH directory: `/opt/mbr/.ssh` with proper permissions

### Single-Service Replacement

The deploy workflow removes the single-slot `mbr.service` unit entirely to
prevent accidental auto-restart outside the blue-green pair.

## Consequences

**Positive:**
- zero-downtime deployments
- instant rollback capability
- simple implementation using existing tools
- no additional infrastructure required

**Negative:**
- requires deploy-window-safe migrations
- brief period of two processes running
- requires active-slot tracking

## References

- `MoveBigRocks/instance-template/deploy/mbr-blue.service` - Blue slot service
- `MoveBigRocks/instance-template/deploy/mbr-green.service` - Green slot service
- `MoveBigRocks/instance-template/deploy/Caddyfile.example` - Health-based routing config
- `MoveBigRocks/instance-template/.github/workflows/_deploy.yml` - Blue-green deployment workflow
