# ADR 0008: Session Token Security

**Status:** Accepted

## Context

Session tokens stored in plaintext create security risks:
1. **Database Compromise**: Extracted tokens are immediately usable
2. **Token Leakage**: SQL logs, backups may contain credentials
3. **Compliance**: GDPR, SOC 2 require protection of auth credentials

## Decision

**Hash session tokens using SHA-256 before storage.**

### Why SHA-256 (not bcrypt)?

| Factor | bcrypt | SHA-256 |
|--------|--------|---------|
| Purpose | Password hashing | Token hashing |
| Use case | User-chosen (weak) secrets | Cryptographically random tokens |
| Lookup speed | Slow (~100ms per compare) | Fast (indexed lookup) |
| Brute force | Resistant (intentionally slow) | Impractical (256-bit entropy) |

Session tokens use 32 bytes (256 bits) of cryptographic randomness. This high entropy makes brute-forcing infeasible, allowing fast SHA-256 lookups.

### Implementation

```go
// Session creation
token := auth.GenerateSecureToken(32)  // 256-bit random
hash := sha256.Sum256([]byte(token))
session.TokenHash = hex.EncodeToString(hash[:])
db.Create(&session)

// Return token to set in cookie (never stored)
return session, token

// Session validation
hash := sha256.Sum256([]byte(token))
tokenHash := hex.EncodeToString(hash[:])
db.First(&session, "token_hash = ?", tokenHash)
```

### Database Schema

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL,  -- SHA-256 hash
    -- plaintext token never stored
    ...
);
CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions(token_hash);
```

## Consequences

**Positive:**
- Session tokens protected at rest
- Database compromise doesn't expose usable credentials
- SQL logs and backups are safe
- Consistent with API key security (also hashed)

**Negative:**
- Additional SHA-256 computation per request (negligible)

## References

- Session service: `internal/platform/services/session_service.go`
- User store: `internal/infrastructure/stores/sql/user_store.go`
