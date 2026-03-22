# ADR 0017: Magic Login Honeypot Signal for Bot Detection

**Status:** proposed
**Date:** 2026-02-15
**Deciders:** @adrianmcphee

## Context

Magic-link login endpoints are a common automation target. The existing rate limiting and generic responses reduce enumeration risk, but they do not provide a direct bot-attribution signal.

We need a low-friction signal that:

- is transparent to legitimate users,
- does not change the user-facing UX or error responses,
- and raises bot confidence on the first suspicious submission.

## Decision

1. Add a hidden `honeypot` field to user-facing magic-link forms:
   - `web/admin-panel/templates/login.html`
2. Treat any non-empty `honeypot` value as suspicious telemetry.
3. On honeypot hits, continue returning the normal generic success response and do not issue authentication.
4. Apply additional rate limiting pressure using a source fingerprint derived from IP + User-Agent for subsequent attempts.
5. Keep behavior identical to normal enumeration-safe responses so humans do not see a different outcome on first suspicious submit.

## Consequences

### Positive

- Bot attempts are easier to detect without introducing UX friction for humans.
- Repeated honeypot fills from a source are slowed down via stricter rate limiting.
- Existing anti-enumeration behavior remains unchanged for visible responses.

### Negative

- Advanced bot traffic that avoids `POST` forms may not trigger the honeypot.
- Additional telemetry and rate-limit storage is added for suspicious submissions.

### Neutral

- A single false-positive honeypot hit does not block immediately; escalation happens through subsequent attempts.

## Compliance

- Confirm the hidden field is present in the rendered magic-login form.
- Confirm suspicious submissions use `magic_link_honeypot` tracking key and return the standard success body.
- Confirm all honeypot paths short-circuit with the same generic message as normal magic-link processing.

## Related

- **Related ADRs:** 0008-session-token-security
