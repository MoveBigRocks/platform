# ADR 0002: Security Architecture

**Status:** Accepted

**Related ADRs:**
- [0003](0003-multi-tenant-isolation.md): Multi-Tenant Isolation
- [0007](0007-s3-attachments.md): S3 Attachments with Virus Scanning

## Context

Move Big Rocks is an agent-first operations platform handling sensitive customer support data, extension-hosted operational workloads, and business automation. AI agents are first-class principals alongside human users, making security boundaries especially critical.

**Security Requirements:**
1. Complete tenant isolation (GDPR, regulatory compliance)
2. Secure authentication without passwords
3. Role-based access control at instance and workspace levels
4. Protection against common web vulnerabilities (OWASP Top 10)
5. Security event logging for audit visibility
6. Secure handling of file attachments
7. API security for external integrations

## Decisions

### 1. Passwordless Authentication via Magic Links

- Magic links expire after 15 minutes
- Single-use tokens (marked as used immediately upon verification)
- IP address and user agent logged
- Generic error messages prevent email enumeration

### 2. Opaque Session Tokens

- Session cookies contain random, opaque 256-bit token values (32 bytes)
- Default expiry: 7 days
- Session validated against database on every request
- Idle timeout: 7 days (matches session expiry)
- Session revocation supported

Token values are never stored in cleartext. The service stores a SHA-256 hash of the token in `sessions.token_hash` and validates incoming cookies by hashing and looking up the hash.

`JWT_SECRET` remains required in production and is used as the HMAC secret for admin Grafana access signing, not for user session tokens.

### 3. Secure Session Cookies

```go
c.SetSameSite(http.SameSiteLaxMode)
c.SetCookie("mbr_session", token, maxAge, "/", cookieDomain, secure, true)
```

- `HttpOnly`: Prevents XSS token theft
- `Secure`: HTTPS-only in production
- `SameSite=Lax`: CSRF protection
- `Domain`: Configurable via `COOKIE_DOMAIN` env var for cross-subdomain auth

**Cookie Domain Configuration:**
- Production: `.app.example.com` (note leading dot) enables cookies across `admin.`, `api.`, and `app.` subdomains
- Development: empty (host-only cookies for localhost)

This enables OAuth authorization flows where users authenticate on one subdomain (admin) and authorize on another (api).

### 4. Instance Role Model

**Instance Roles (Platform Administration):**
- `super_admin`: Full instance access across all workspaces and all admin capabilities.
- `admin`: All platform operations plus user-management permissions.
- `operator`: Platform operations across all workspaces, excluding user-management.

**Workspace Roles (Tenant-Scoped):**
- `owner`: Billing, full control
- `admin`: Full workspace management
- `member`: Case management
- `viewer`: Read-only access

### 5. Multi-Tenant Isolation

Two-layer defense:

```
Layer 1: Middleware - Extract workspace_id from session
Layer 2: Handler/Store - Verify workspace_id on all queries
```

**Fail-Safe:** All queries require workspace_id - missing context returns empty results.

See [ADR-0003](0003-multi-tenant-isolation.md) for details.

### 6. Security Headers

```go
c.Header("X-Content-Type-Options", "nosniff")
c.Header("X-Frame-Options", "DENY")
c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
c.Header("Content-Security-Policy", "default-src 'self'; ...")
```

### 7. CSRF Protection

Single-use form nonces with 15-minute TTL, combined with SameSite=Lax cookies.

### 8. Rate Limiting

- **Authentication**: Magic link attempts per email
- **API**: Per-key limits with configurable windows
- **Error ingestion**: Per-project event limits
- **Rule execution**: Per-rule hourly/daily caps

### 9. Audit Logging

Auditability requirements:
- User action visibility and security telemetry are captured for operators and admins.
- Storage is retention-governed and supports operational cleanup; logs are not marketed as immutable records.

All operations are logged with:
- User ID, workspace ID
- Action, resource type, resource ID
- Old/new values (for updates)
- IP address, user agent, session ID

### 10. Attachment Security

All file attachments scanned with ClamAV before storage:

1. Validate: size, extension, content-type
2. Check blocked extensions
3. Calculate SHA-256 hash
4. Scan with ClamAV
5. If clean: upload to S3
6. If infected: reject, log security event

### 11. Input Validation

Multi-layer validation:
1. **Middleware**: Path parameter validation
2. **Handler**: Request body binding
3. **Domain**: Business rule validation
4. **Database**: Constraints and foreign keys

All queries use parameterized statements (no SQL injection).

## Consequences

**Positive:**
- Defense in depth with multiple layers
- Fail-safe defaults throughout
- Comprehensive security audit logging
- Passwordless eliminates credential vulnerabilities

**Negative:**
- Multiple security layers add complexity
- Magic link depends on email deliverability

## References

- Authentication: `internal/infrastructure/auth/`
- Session management: `internal/platform/services/session_service.go`
- Authorization: `internal/infrastructure/middleware/context_auth.go`
- Virus scanning: `internal/infrastructure/antivirus/clamav.go`
