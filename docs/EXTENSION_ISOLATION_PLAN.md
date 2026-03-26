# Extension Isolation Plan

This note records the architecture that the repo should continue to enforce:

- `platform` owns generic extension hosting, routing, manifests, lifecycle, and trust.
- first-party extensions own their own domain code, handlers, stores, background jobs, and admin surfaces.
- `platform` does not carry source trees for particular extensions.

## Current state

The first-party extraction work is complete for the currently shipped product extensions:

- `web-analytics` runtime code lives in `MoveBigRocks/extensions`.
- `error-tracking` runtime code lives in `MoveBigRocks/extensions`.
- `ats` product-specific domain code lives in `MoveBigRocks/extensions`.
- `platform/internal/analytics`, `platform/internal/observability`, and the matching extension-specific SQL/runtime glue were removed from `platform`.
- the generic host/runtime bridge supports `unix_socket_http` without platform-owned first-party bootstrap logic.

## Guardrail

Do not re-introduce platform-owned source trees for first-party extensions. If a runtime needs new behavior, add it under `MoveBigRocks/extensions` and expose only generic host contracts from `platform`.
