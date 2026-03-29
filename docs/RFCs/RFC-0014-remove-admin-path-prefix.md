# RFC-0014: Remove /admin/ Path Prefix from Admin Routes

**Status:** accepted
**Author:** @adrianmcphee
**Created:** 2026-03-25

## Summary

Remove the `/admin/` path prefix from all admin-subdomain routes. The admin subdomain (`admin.example.com`) already establishes admin context, making the `/admin/` path prefix redundant.

## Problem Statement

Current admin URLs are structured as:
- `admin.mbr.demandops.com/admin/extensions/web-analytics`
- `admin.mbr.demandops.com/admin/actions/workspaces/123`
- `admin.mbr.demandops.com/admin/graphql`

The "admin" appears twice (subdomain + path), which is redundant and confusing. The subdomain is the authoritative signal for admin context.

## Proposed Solution

### Overview

Strip `/admin/` from all admin-scoped path prefixes. The resulting URLs become:

| Before | After |
|--------|-------|
| `/admin/extensions/web-analytics` | `/extensions/web-analytics` |
| `/admin/actions/workspaces/:id` | `/actions/workspaces/:id` |
| `/admin/graphql` | `/graphql` |

Routes already at root (login, dashboard, metrics, grafana) are unchanged.

### Changes Required

1. **`cmd/api/routers.go`** — Route group prefixes: `/admin/extensions` → `/extensions`, `/admin/actions` → `/actions`, `/admin/graphql` → `/graphql`
2. **`extensions/*/manifest.json`** — Extension mountPaths: `/admin/extensions/...` → `/extensions/...`
3. **`web/admin-panel/templates/`** — Template hrefs and form actions
4. **`internal/platform/services/extension_runtime.go`** — Reserved path validation and mount namespace rules
5. **Production DB** — Update `manifest_json` in `installed_extensions` rows

### What Does NOT Change

- Auth routes: `/login`, `/logout`, `/verify-magic-link` (already at root)
- Dashboard: `/dashboard` (already at root)
- Grafana proxy: `/grafana/*` (already at root)
- Metrics page: `/metrics` (already at root)

## Verification Criteria

- [ ] All admin pages load without `/admin/` prefix
- [ ] Extension admin pages resolve correctly
- [ ] GraphQL endpoint works at `/graphql`
- [ ] Template links point to correct paths
- [ ] Extension manifest validation accepts new mount paths
- [ ] Build passes, tests pass

## Implementation Checklist

- [x] RFC created
- [ ] Route groups updated in routers.go
- [ ] Extension manifests updated
- [ ] Templates updated
- [ ] Extension runtime validation updated
- [ ] Production DB manifests updated
- [ ] Tests passing
- [ ] Deployed to production
