# Move Big Rocks Infrastructure Guide

Production deployment on a single Ubuntu host using systemd, Caddy, and PostgreSQL.

This guide assumes the live installation is controlled from a **private instance repo**, not from the public core repo.

The fastest getting-started path is a DigitalOcean Ubuntu Droplet. The same
guide also fits any equivalent Ubuntu VPS.

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│  Ubuntu Droplet / VPS                                        │
│                                                              │
│  Caddy (TLS + host routing)                                  │
│     ├─ app.<domain>               -> Move Big Rocks blue/green    │
│     ├─ admin.<domain>         -> Move Big Rocks + /grafana/*  │
│     └─ api.<domain>           -> Move Big Rocks API           │
│                                                              │
│  Move Big Rocks systemd blue/green services                          │
│     ├─ mbr-blue.service  (port 8080)                      │
│     └─ mbr-green.service (port 8081)                      │
│                                                              │
│  Prometheus + Grafana (localhost only)                       │
│  PostgreSQL (external or managed, via DATABASE_DSN)          │
└──────────────────────────────────────────────────────────────┘
```

**Note:** Ports 8080 and 8081 are defaults for blue/green slots. On shared hosts where those ports are already in use, update the port numbers in the systemd service files, Caddyfile, and deploy workflow accordingly.

## Deployment Model

- the host runs one Move Big Rocks installation
- the installation should be driven from a private instance repo created from `MoveBigRocks/instance-template`
- the instance repo owns CI/CD, secrets wiring, and pinned release refs
- blue-green deployment remains the recommended rollout model

Canonical entry points:

- [Customer Instance Setup](https://github.com/movebigrocks/platform/blob/main/docs/CUSTOMER_INSTANCE_SETUP.md)
- [Customer FAQ](https://github.com/movebigrocks/platform/blob/main/docs/CUSTOMER_FAQ.md)

## Server Bootstrap

1. Create Ubuntu 22.04+ host.
2. Create a private instance repo from the public Move Big Rocks instance template.
3. Fill in `mbr.instance.yaml` with the pinned core artifact refs, host, domains, storage, and email choices.
4. Validate it with `scripts/read-instance-config.sh mbr.instance.yaml`.
5. Add GitHub repository or environment secrets for that instance repo.
6. Clone that instance repo on the host or let an agent use it as the deployment control plane.
7. Run `deploy/setup.sh` as root from the instance repo.
8. Add the deploy SSH key for user `mbr`.
9. Deploy the pinned Move Big Rocks release from that repo.

Canonical setup/runbook inside the deployment control plane lives in the
instance-template repo, not in this core repo.

## DNS

Point these records to the production IP:

- `app.<domain>`
- `admin.<domain>`
- `api.<domain>`

Add explicit tenant subdomains or wildcard DNS only if you expose workspace-scoped subdomain routes.

For the public Move Big Rocks site itself, manage domains, routing, and website
updates from the site instance repo rather than the core product repo.

## Operations

```bash
# Active slot
cat /opt/mbr/.active-slot

# Service status
systemctl status mbr-blue mbr-green caddy prometheus grafana-server

# Follow logs
journalctl -u mbr-blue -f
journalctl -u mbr-green -f
journalctl -u caddy -f

# Health checks
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8081/health
curl https://api.example.com/v1/health
```

## Backups

PostgreSQL backups, retention, and point-in-time recovery are owned by your
database provider or your PostgreSQL runbook. The instance repo verifies
database connectivity through `DATABASE_DSN`; it no longer installs a
database-side backup sidecar on the app host.

## Monitoring

- Grafana: `https://admin.<domain>/grafana/`
- Prometheus: `127.0.0.1:9090` (internal only)
- App metrics endpoint protected by `METRICS_TOKEN` in production.

## Troubleshooting

- SSL/TLS: verify DNS records and Caddy status.
- App boot failures: `journalctl -u mbr-blue -n 100 --no-pager` (or green).
- Database connectivity issues: verify `DATABASE_DSN` in `/opt/mbr/.env`
  and validate connectivity with `psql "$DATABASE_DSN"`.
- Backup issues: follow the PostgreSQL provider or operator runbook instead of
  app-host service logs.
