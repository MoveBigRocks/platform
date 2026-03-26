# Extension Isolation Plan

This note records the remaining work required to reach the intended architecture:

- `platform` owns generic extension hosting, routing, manifests, lifecycle, and trust.
- first-party extensions own their own domain code, handlers, stores, background jobs, and admin surfaces.
- `platform` does not carry source trees for particular extensions.

## Current state

Completed:

- `web-analytics` runtime code now lives in `MoveBigRocks/extensions`.
- `platform/internal/analytics` and the matching analytics SQL/runtime code were removed from `platform`.
- the generic host/runtime bridge now supports `unix_socket_http` without platform-owned analytics bootstrap logic.

Still to extract:

- `error-tracking` domain/services/handlers still exist in `platform/internal/observability`.
- `platform` still exposes error-tracking-specific admin handlers and workspace APIs.
- GraphQL still includes an observability surface owned by `platform`.
- some shared contracts still refer directly to issue/project types.

## Remaining extraction order

1. Finish the `error-tracking` runtime package in `MoveBigRocks/extensions`.
   - make its local SQL/runtime helpers self-contained
   - point local handlers/services/domain imports at extension-owned packages
   - stop depending on `platform/internal/observability/...`

2. Move error-tracking HTTP/UI surfaces out of `platform`.
   - admin applications/issues pages
   - workspace issue APIs
   - case-detail linked-issue rendering should become extension-provided or generic

3. Remove error-tracking service wiring from core containers.
   - `internal/infrastructure/container/observability.go`
   - error-tracking-specific worker/container setup

4. Decide the long-term API boundary for extension-owned product APIs.
   - preferred direction: extension-owned HTTP/JSON APIs
   - avoid keeping extension-specific GraphQL types in core

5. After the above, remove `platform/internal/observability` and its remaining store/model glue.

## Guardrail

Do not re-introduce platform-owned source trees for first-party extensions. If a runtime needs new behavior, add it under `MoveBigRocks/extensions` and expose only generic host contracts from `platform`.
