# ADR 0020: GeoIP Location Service

**Status:** accepted
**Date:** 2026-03-05
**Deciders:** @adrianmcphee

## Context

Move Big Rocks geolocation uses IP-derived lookup rather than `Accept-Language`
heuristics.

`Accept-Language` is unreliable for this job:

- `"en"` returns no country in many cases
- `"en-US"` reflects locale preference rather than actual visitor location
- it cannot supply region or city data
- many mobile browsers send minimal headers

Move Big Rocks already resolves client IP server-side for rate limiting and visitor ID
hashing, while keeping raw IPs out of persisted analytics data.

## Decision

Use **MaxMind GeoLite2** as a local MMDB database for IP-to-location
resolution, exposed as a **shared internal service** (`internal/shared/geoip/`)
usable by any Move Big Rocks module or extension.

Key design choices:

1. **Shared service, not analytics-specific.** The `GeoIPService` interface lives in `internal/shared/geoip/` so it can serve analytics, authentication, fraud detection, and extension use cases.
2. **Interface-based with noop fallback.** If no MMDB file is configured (`GEOIP_DB_PATH` unset), a noop implementation returns empty results and the system degrades gracefully.
3. **Local database, no API calls.** The MMDB file is loaded into memory at startup. Lookups are microsecond-latency with zero network overhead. IPs never leave the server.
4. **Schema additions.** `region` and `city` columns are added to analytics `events` and `sessions`, with corresponding `REGION` and `CITY` breakdown dimensions in GraphQL.
5. **No raw IP storage.** The IP is used in-memory for lookup and then discarded. Only derived location fields are persisted.

Implementation:

- `GeoIPService` interface: `Lookup(ip string) *GeoLocation`
- `MaxMindService`: loads `.mmdb` at startup via `oschwald/maxminddb-golang`
- `NoopService`: returns empty `GeoLocation`
- configuration: `GEOIP_DB_PATH`
- DB update: operator responsibility via MaxMind account download and deployment

## Consequences

### Positive

- accurate country, region, and city data suitable for privacy-first analytics
- zero runtime dependencies on third-party APIs
- privacy-preserving operation because IPs never leave the server and are not stored
- shared geolocation service for analytics, auth checks, and extensions
- graceful degradation when MMDB is not configured

### Negative

- operator must download and maintain the GeoLite2 database file
- requires free MaxMind account registration for database download
- accuracy depends on MaxMind's data quality

### Neutral

- MaxMind GeoLite2 requires attribution in documentation
- binary size is unchanged because the MMDB file is loaded from disk, not embedded

## Compliance

- `GeoIPService` is an interface, so implementations can be swapped without changing consumers
- noop fallback ensures the system works without MMDB configured
- no raw IPs are stored in the database
- `make test-all` validates analytics queries with the new columns
- GraphQL schema includes `REGION` and `CITY` dimensions

## Related

- **Related RFCs:** RFC-0004 (Extension System)
- **Related ADRs:** 0006 (Bounded Context Structure), 0019 (Embed Static Assets)
