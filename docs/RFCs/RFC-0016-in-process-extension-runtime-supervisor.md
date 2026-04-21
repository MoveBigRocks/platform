# RFC-0016: In-Process Extension Runtime Supervisor

**Status:** accepted
**Author:** @adrianmcphee
**Created:** 2026-04-21

## Summary

Move extension runtime process lifecycle from systemd (one unit per extension,
per-extension sudoers rules, per-extension env files) into the core mbr
service. The core binary reads `runtime-manifest.json` at startup, spawns each
extension runtime as a supervised child process, pipes its stdout/stderr into
the core's structured logger, restarts it on crash with backoff, and SIGTERMs
it on shutdown. The deploy step that managed extension runtime units
(`mbr-extension-runtime@*.service`) is removed; so are the sudoers rules for
those units. Blue-green isolation becomes natural: each slot owns its own
child runtimes bound to slot-scoped socket paths.

## Problem Statement

The current model treats each extension runtime as an independent systemd
unit template (`mbr-extension-runtime@.service`) with a per-extension sudoers
allowlist and a deploy loop that stops/starts each unit. Two concrete failure
modes already bit production:

1. **Sudoers drift.** `deploy/mbr-sudoers` is installed once at bootstrap by
   `setup.sh` and never re-synced by deploy. Adding a new extension requires
   six new sudoers lines (`start/stop/restart/status/enable/disable`), and
   when those lines don't make it to `/etc/sudoers.d/mbr`, the deploy's
   `sudo /bin/systemctl restart mbr-extension-runtime@...` silently fails
   (sudo returns a password prompt over a non-tty ssh heredoc, exit non-zero,
   the `2>/dev/null || true` patterns and `set -e` interactions with for-loops
   swallow it). The deploy reports green while the old binary keeps serving.
   This happened on 2026-04-21 with web-analytics v0.8.25 and was only caught
   because a curl against the new agent surface returned 404.

2. **Adding a new extension is a documented ops config change.** Six sudoers
   lines, a new env file, a systemd unit start, a service file edit. Easy to
   miss; no compile-time or CI check. The deploy workflow does not fail until
   an operator hits the missing surface.

The runtimes themselves are not genuinely independent services. They share
the core's release cycle, they bind to the same binary distribution, and
they cannot outlive the core meaningfully (the core's socket dispatcher
must be alive to reach them). The per-service systemd model is ceremony that
buys nothing in exchange for brittleness.

## Proposed Solution

### Overview

Introduce `internal/extensionhost/supervisor` in the platform repo. One
`Supervisor` instance owned by the core (`cmd/api`) is responsible for the
lifecycle of all extension runtime processes for the running slot:

- **Source of truth**: the `runtime-manifest.json` produced by
  `reconcile-extensions` (already copied into `/opt/mbr/extensions/` during
  deploy).
- **Spawn**: for each entry, `exec.CommandContext(ctx, "/opt/mbr/extensions-runtime/bin/"+entry.Binary)`
  with environment containing `MBR_EXTENSION_RUNTIME_SOCKET_PATH` pointing at
  the slot-scoped socket, and inheriting the core's DB DSN / secret env.
- **Slot scoping**: socket root becomes
  `${EXTENSION_RUNTIME_DIR}/${MBR_SLOT}/...` when `MBR_SLOT` is set. Blue and
  green bind to disjoint directories so both can run during a cutover without
  colliding. `cmd/api`'s own socket dispatcher reads the same rule.
- **Supervision**: per-child goroutine waits for exit. If the context is not
  cancelled (shutdown), the supervisor re-spawns with exponential backoff
  starting at 2s and capped at 60s, logging each attempt.
- **Shutdown**: on context cancel, the supervisor sends SIGTERM to every
  child, waits up to a configurable grace (default 10s), then SIGKILL the
  stragglers and closes its internal wait group.
- **Process death on parent exit**: children are spawned with
  `SysProcAttr{Pdeathsig: syscall.SIGTERM}` (Linux) so a core crash reaps them
  automatically — belt-and-braces with the explicit SIGTERM on the shutdown
  path.
- **Logging**: stdout and stderr of each child are scanned line-by-line and
  emitted through the core's logger with a `source=extension_runtime slug=...`
  tag. Extension logs end up in the same `journalctl -u mbr-blue` stream as
  the core.

### Deploy simplification (mbr-prod repo)

- Remove `deploy/mbr-extension-runtime@.service` from the instance repo and
  from `/etc/systemd/system/` on the server (one-shot idempotent cleanup in
  the deploy workflow's first run after this change: `systemctl disable`,
  `systemctl stop`, `rm` the unit file, `daemon-reload`).
- Remove all six-per-extension sudoers rules and the associated
  `cp /opt/mbr/mbr-extension-runtime@.service ...` line from
  `deploy/mbr-sudoers`.
- Remove the two for-loops in `_deploy.yml` that stopped and restarted
  extension runtime units, and the `extensions-runtime/env/*.env` generation
  (the only content today is `MBR_EXTENSION_RUNTIME_SOCKET_PATH`, now set by
  the supervisor).
- Re-install `deploy/mbr-sudoers` to `/etc/sudoers.d/mbr` on every deploy via
  `visudo -cf` validation, so the symmetric sudoers-drift bug can't recur.
  Even though the per-extension rules are gone, the same drift pattern could
  bite future rules.

### Changes Required

1. **`platform/internal/extensionhost/supervisor/supervisor.go`** — new
   package with `Supervisor`, `Config`, `New`, `Start`, `Stop`. Tested with
   a fake runtime binary that can be scripted to crash, hang, or exit cleanly.
2. **`platform/cmd/api/main.go`** — construct `Supervisor` after DB init,
   start it before the HTTP server starts, stop it after HTTP server shutdown.
3. **`platform/pkg/extensionhost/infrastructure/config/config.go`** — new
   `MBRSlot` config (env `MBR_SLOT`, default empty). When non-empty, the
   socket dispatcher and supervisor both append the slot to
   `EXTENSION_RUNTIME_DIR`.
4. **`platform/internal/extensionhost/runtime/socket_transport.go`** — honour
   slot-scoped socket paths in `doUnixSocketRequest`.
5. **`mbr-prod/deploy/mbr-blue.service` + `mbr-green.service`** — add
   `Environment=MBR_SLOT=blue` and `MBR_SLOT=green` respectively.
6. **`mbr-prod/deploy/mbr-sudoers`** — remove 26 lines of per-extension
   sudoers + the one `cp` rule for the extension-runtime service file.
7. **`mbr-prod/deploy/mbr-extension-runtime@.service`** — delete.
8. **`mbr-prod/.github/workflows/_deploy.yml`** — remove the two for-loops
   (stop + restart-with-ready-wait), remove the service-file install block,
   remove the env-file rsync (still sync sudoers). Add a one-shot cleanup
   step that removes any legacy `mbr-extension-runtime@*.service` instance
   and template from `/etc/systemd/system/` if present.
9. **`mbr-prod/deploy/setup.sh`** — drop the initial install of the
   extension-runtime template (superseded by this RFC); sudoers rule for
   template install also removed.

### What Does NOT Change

- `runtime-manifest.json` schema or producer (`reconcile-extensions`).
- Extension manifest format or contract.
- Extension runtime binaries themselves (they continue listening on the
  socket they're told about via `MBR_EXTENSION_RUNTIME_SOCKET_PATH`).
- The core's socket dispatch code path (same `runtimeproto.SocketPath` API,
  just a different root directory when `MBR_SLOT` is set).
- Caddy config. Caddy still load-balances to localhost:8088/8089 (the core);
  the core fan-outs to its children's sockets.

## ADR Compliance

| ADR / RFC | Title | Compliance |
|-----------|-------|------------|
| ADR 0014 | Blue-Green Deployment | Both slots now truly independent — each owns its runtime processes and sockets. Cutover semantics unchanged externally. |
| ADR 0026 | Extension Host Lifecycle and Public Extension SDK Boundary | No SDK boundary change. Runtimes still talk to the platform over the documented unix_socket_http protocol. |
| RFC-0004 | Extension System | Clarifies the runtime lifecycle model implicit in RFC-0004. |

## Alternatives Considered

### Alternative 1: wildcard sudoers rule + re-sync sudoers on every deploy

Replace the 6 rules per extension with
`mbr ALL=(ALL) NOPASSWD: /bin/systemctl * mbr-extension-runtime@*` and teach
deploy to reinstall sudoers every run.

**Pros:** minimal change, ~10 lines in `_deploy.yml` and sudoers.
**Cons:** keeps the per-extension systemd unit dance forever. Adding a new
extension still requires env-file generation in the workflow, a runtime
manifest entry, and a systemd unit instance. The silent-failure class
(anything outside the wildcard) remains.
**Why rejected:** patches the symptom; leaves the architectural
mismatch ("extensions as independent services") intact.

### Alternative 2: drive per-extension sudoers lines from runtime-manifest.json

Generate sudoers dynamically in CI from the manifest.

**Pros:** stricter than wildcard; every rule is explicitly scoped.
**Cons:** couples the deploy script to the manifest format; adds a code path
that has to be kept in sync with the reconciler; does not remove the per-
extension systemd ceremony.
**Why rejected:** more code for the same architectural mismatch.

### Alternative 3: systemd user units under the `mbr` user

Run extension runtimes as `systemctl --user` units. No sudo needed.

**Pros:** removes sudoers surface for runtimes entirely.
**Cons:** requires `loginctl enable-linger mbr`; user unit lifecycle and
journal scope are subtly different from system units; two classes of units
on the same host confuses operators. Still one unit per extension.
**Why rejected:** solves sudoers drift but not per-extension ceremony.

### Alternative 4: cascade via `PartOf=mbr-<slot>.service`

Add `PartOf=mbr-blue.service mbr-green.service` to the template so a slot
restart cascades into the runtime units.

**Pros:** deploy simplifies; no explicit restart loop.
**Cons:** sudoers drift remains — `PartOf` only handles stop/restart, not
initial start of a new-slug template instance. Adding an extension still
requires sudoers updates for `enable` + `start`. Also, `PartOf` behaviour on
templated units across two separate parent services is subtle enough to
warrant its own bug budget.
**Why rejected:** cleaner than 1–3, still touches sudoers and keeps the
ceremony.

## Verification Criteria

### Unit Tests (supervisor package)
- [ ] Supervisor spawns every entry in a test runtime-manifest
- [ ] Supervisor restarts a child that exits with non-zero and respects
      backoff (>= 2s between first and second spawn, <= 60s cap)
- [ ] Supervisor does NOT restart children once `Stop` has been called
- [ ] `Stop` sends SIGTERM, then SIGKILL after grace, in that order
- [ ] Missing binary path surfaces an error at `Start` time rather than
      silently spin-looping

### Integration Tests
- [ ] End-to-end test in `cmd/api` that boots the core with a one-entry
      runtime-manifest pointing at a scripted fake binary, asserts the
      child is alive, issues a shutdown signal, asserts the child exits
      within grace.

### Acceptance Criteria (post-deploy)
- [ ] `/etc/systemd/system/mbr-extension-runtime@.service` is absent on
      mbr.demandops.com
- [ ] `/etc/systemd/system/mbr-extension-runtime@*-runtime.service`
      instance files absent
- [ ] `/etc/sudoers.d/mbr` contains no `mbr-extension-runtime@` rules
- [ ] `pgrep -f web-analytics-runtime` shows exactly one PID, a child of
      the active `mbr-blue` or `mbr-green` process (verified via
      `/proc/<pid>/status` `PPid:`)
- [ ] `journalctl -u mbr-green -n 50` shows lines tagged
      `source=extension_runtime slug=web-analytics` for recent analytics
      ingest events
- [ ] `curl -H "Authorization: Bearer <agent_token>" POST
      /extensions/web-analytics/api/agent/properties` still returns 201
      (the RFC-0015 surface continues to work after the architectural shift)

## Implementation Checklist

- [ ] RFC committed
- [ ] Supervisor package + unit tests
- [ ] Integration test in cmd/api
- [ ] Supervisor wired into cmd/api/main.go with proper start/stop ordering
- [ ] Slot-scoped socket path support in config + socket dispatcher
- [ ] mbr-blue.service and mbr-green.service updated with `MBR_SLOT=`
- [ ] mbr-sudoers cleaned up; sudoers re-synced on every deploy
- [ ] mbr-extension-runtime@.service removed from repo
- [ ] _deploy.yml simplified
- [ ] One-shot cleanup of legacy units on mbr.demandops.com (first
      post-change deploy)
- [ ] Post-deploy acceptance checks run green against mbr.demandops.com

## Related

- **Related RFCs:** RFC-0004 (Extension System), RFC-0015 (Agent-Callable
  Extension Endpoints)
- **Related ADRs:** ADR 0014 (Blue-Green Deployment), ADR 0026 (Extension
  Host Lifecycle and Public Extension SDK Boundary), ADR 0028 (Dual-Auth Gate)
- **Supersedes:** the per-extension systemd/sudoers pattern established in
  `setup.sh`, not formally documented in an ADR

---

## Changelog

| Date | Author | Change |
|------|--------|--------|
| 2026-04-21 | @adrianmcphee | Initial draft, written after the silent sudoers-drift failure during the web-analytics-v0.8.25 deploy. |
