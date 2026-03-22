# ADR 0019: Embed Static Assets and Schemas in Binary

**Status:** accepted
**Date:** 2026-03-05
**Deciders:** @adrianmcphee

## Context

The Move Big Rocks binary loads several categories of files at runtime using relative path resolution:

- SQL migration trees under `migrations/` and extension bundle `migrations/`
- Admin panel HTML templates (`web/admin-panel/templates/**/*.html`)
- Static assets: CSS, JS, images (`web/static/**`)
- Analytics tracking script (`web/static/js/analytics.js`)
- GraphQL schema (fixed in prior commit via `//go:embed`)

These files are resolved relative to the process working directory. This works because the systemd unit sets `WorkingDirectory=/opt/mbr` and the CI deploy pipeline rsyncs migration files, templates, and static assets to that directory.

This architecture has several problems:

1. **Silent fallback bugs.** The GraphQL schema had a 1000-line hardcoded fallback string that was never updated when analytics types were added. Production silently used the stale fallback for weeks, causing `Cannot query field "createAnalyticsProperty"` errors.

2. **Fragile coupling between binary and deploy pipeline.** The binary assumes specific files exist at specific relative paths. If the deploy pipeline changes (e.g., different rsync target, packaging change, new host), the binary breaks with no compile-time warning.

3. **Path search anti-pattern.** Multiple `findSchemaFile()` functions try 5-6 candidate paths including `../`, `../../`, `../../../` variations. This is a code smell indicating the real problem: runtime file resolution is unreliable.

4. **Binary-asset version skew.** During blue-green deploys, the rsync of templates/migrations/static files happens before the new binary starts. If the deploy fails partway, the old binary could be running with new assets or vice versa.

## Decision

Use Go's `//go:embed` directive to compile all static assets into the binary at build time.

**What gets embedded:**

| Category | Embed Location | Go Package |
|----------|---------------|------------|
| GraphQL schema | `internal/graphql/schema/` | `schema` (already done) |
| SQL migrations | `migrations/` | `migrations` (new embed package) |
| Admin templates | `web/admin-panel/templates/` | `web` (new embed package) |
| Static files (CSS, JS, images) | `web/static/` | `web` (same package) |

**What gets removed:**

- All `findSchemaFile()` / `findAnalyticsSchemaFile()` path-searching functions
- The rsync steps for migrations, templates, and static files in `_deploy.yml`
- The `router.Static()` calls replaced with `http.FS()` serving from embedded FS
- The dead `router.Static("/assets", "./web/admin-panel/assets")` route (directory doesn't exist)

**Implementation approach:**

- Create `migrations/embed.go` exporting `embed.FS` for SQL files
- Create `web/embed.go` exporting `embed.FS` for templates and static assets
- Update `db.go` and `analytics_db.go` to read schemas from embedded FS
- Update `admin.go` to parse templates from embedded FS via `template.ParseFS()`
- Update static file serving to use `http.FS()` with the embedded FS
- Update `analytics_script_handler.go` to read from embedded FS
- Simplify deploy pipeline: only ship the binary, service files, and `.env`

## Consequences

### Positive

- **Single binary deployment.** The binary is fully self-contained — no file syncing, no path resolution, no WorkingDirectory dependency.
- **Compile-time guarantees.** If a template or migration file is missing, `go build` fails. No runtime surprises.
- **Atomic deploys.** Binary and its assets are always in sync. No version skew during blue-green transitions.
- **Simpler deploy pipeline.** Remove rsync steps for migrations, templates, and static files. Deploy is just: copy binary, restart service.
- **Eliminates an entire class of bugs.** No more "works in dev, breaks in prod" path resolution issues.

### Negative

- **Template changes require rebuild.** Can't hot-edit templates on the server. This is acceptable because we already rebuild on every deploy and don't hot-edit in production.
- **Larger binary size.** Embedded assets add to binary size. The embedded asset set is ~500KB total (templates + CSS + JS + images + SQL), which is negligible.
- **Development workflow change.** Template changes during local dev require rebuilding or using a flag to load from filesystem. Standard practice is to use `go build` with `-tags dev` for filesystem override during development.

### Neutral

- Aligns with standard Go practices (Hugo, CockroachDB, Gitea, Caddy all embed assets)
- `embed.FS` is read-only, which matches our intent — these files should never be modified at runtime

## Compliance

- `go build` fails if any embedded file is missing (compile-time check)
- CI lint and test steps validate the build succeeds
- Deploy pipeline can be simplified to remove rsync steps after migration
- Verify no remaining `os.ReadFile`, `filepath.Glob`, or `router.Static` calls for embedded asset categories

## Related

- **Related ADRs:** 0014 (Blue-Green Deployment), 0021 (PostgreSQL-Only Runtime and Extension-Owned Schemas)
- **Related RFCs:** RFC-0003 (Web Analytics — triggered discovery of this issue)
