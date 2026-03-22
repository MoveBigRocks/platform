# ADR 0013: Server Security Hardening

**Status:** accepted
**Date:** 2026-01-03
**Deciders:** @adrianmcphee

## Context

Production servers are exposed to constant automated attacks:
- SSH brute force attempts (thousands per day)
- Web vulnerability scanners probing for WordPress, phpMyAdmin, .env files, etc.
- Bots searching for common exploit paths

Without proper defense-in-depth, these attacks can:
- Consume server resources and fill logs
- Eventually succeed through persistence
- Compromise the system if any vulnerability exists

## Decision

Implement layered server security with:

### 1. UFW Firewall (Default Deny)
```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp   # SSH
ufw allow 80/tcp   # HTTP
ufw allow 443/tcp  # HTTPS
```

Internal services (Prometheus :9090, Grafana :3000) bind to localhost only.

### 2. fail2ban with Three Jails

**SSH Protection:**
- 3 failed attempts → 30 day ban
- Monitors `/var/log/auth.log`

**Web Attack Protection (mbr-web):**
- 5 exploit probe attempts → 30 day ban
- Monitors systemd journal for application 404s
- Catches WordPress probes, .env access, admin paths, PHP scripts, etc.

**Recidive (Repeat Offenders):**
- 2 bans within 30 days → permanent ban
- Monitors `/var/log/fail2ban.log`

### 3. Journald Log Limits
```ini
[Journal]
SystemMaxUse=200M
MaxRetentionSec=7d
```

Prevents log-based disk exhaustion attacks.

### 4. Web Attack Filter Patterns

The web filter catches requests to:
- WordPress paths (wp-admin, wp-login, xmlrpc)
- Database tools (phpmyadmin, adminer, mysql)
- Config files (.env, .git, .htaccess, database.yml)
- Backup files (.sql, .bak, .backup, .old)
- Admin paths (/admin, /administrator, /console)
- Server-side scripts (.php, .asp, .jsp, .cgi)
- Shell exploits (cgi-bin, shell, cmd, eval)
- Vendor paths (/vendor, /node_modules, composer.json)

## Consequences

### Positive

- Blocks persistent attackers for 30 days (or permanently if recidive)
- Reduces log noise from blocked IPs
- Prevents resource exhaustion from attack traffic
- Defense in depth - multiple layers protect against different threats

### Negative

- Legitimate users from shared IPs could be blocked if someone else attacks
- Configuration must be maintained across server rebuilds (handled by IaC)
- Regex patterns may need updates as attack patterns evolve

### Neutral

- Blocked IPs consume minimal bandwidth at the firewall level
- Ban lists persist across fail2ban restarts

## Compliance

- Instance bootstrap IaC includes all security configuration
- `fail2ban-client status` shows all three jails active
- `ufw status` confirms only ports 22, 80, 443 open

## Related

- **Related ADRs:** 0002-security-architecture, 0021-postgresql-only-runtime-and-extension-schemas
