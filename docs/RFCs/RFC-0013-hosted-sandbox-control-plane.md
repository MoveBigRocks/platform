# RFC-0013: Hosted Sandbox Control Plane

**Status:** draft
**Author:** Adrian McPhee
**Created:** 2026-03-25

## Summary

This RFC preserves the design for a future hosted sandbox path for Move Big
Rocks while keeping the current product posture self-host-first. Hosted
sandboxes are not available today. The supported path remains an owned
deployment on infrastructure the customer controls. This RFC exists so the
design, constraints, and resurrection steps are not lost and so users have a
clear place to comment if they want this path to become real.

Related feedback and demand tracking live in
[issue #1](https://github.com/MoveBigRocks/platform/issues/1).

## Current Product Position

Hosted sandboxes are deferred.

Today the intended evaluation path is:

- run Move Big Rocks locally if you are comfortable with a technical setup, or
- deploy one owned instance on a VPS you control, with agent help if useful

This keeps the trust model honest and avoids investing in a vendor-hosted path
before the self-hosted path is sharper.

## Problem Statement

There are still real reasons to keep the hosted sandbox design alive:

- some evaluators want a lower-friction path than provisioning their own VPS
- some agents benefit from a disposable environment for product inspection and
  extension trials
- some prospects want a fast proof-of-shape path before they commit to an
  owned instance

At the same time, a hosted sandbox path creates real cost and complexity:

- provisioning and teardown orchestration
- domain and TLS automation
- safety boundaries between disposable evaluation and real production
- pricing and abuse controls
- export or promotion paths into owned instances

The feature is worth preserving as a design, but not worth making the primary
public path today.

## Proposed Solution

### Overview

When resurrected, hosted sandboxes should be an optional evaluation path, not
the primary product model.

The core rules:

- production remains self-hosted and customer-controlled
- hosted sandboxes are disposable evaluation environments
- sandbox lifecycle stays CLI-first and machine-readable
- export and handoff into an owned instance are first-class
- sandbox trust remains explicitly weaker than a customer-owned deployment

### Intended User Experience

The intended future experience is:

1. install `mbr`
2. run `mbr sandboxes create ... --json`
3. receive a ready URL, admin/login path, expiry, and next steps
4. inspect the real product model in a disposable hosted environment
5. optionally extend the sandbox or export the evaluation state
6. move into an owned instance if the evaluation succeeds

The key product rule is that sandbox creation should return a usable runtime
directly rather than routing the user through an unrelated vendor workflow.

### CLI and Machine Contract

The expected command family is:

```text
mbr sandboxes create --email EMAIL [--name NAME] [--json]
mbr sandboxes show SANDBOX_ID --manage-token TOKEN [--json]
mbr sandboxes extend SANDBOX_ID --manage-token TOKEN [--json]
mbr sandboxes destroy SANDBOX_ID --manage-token TOKEN [--reason TEXT] [--json]
mbr sandboxes export SANDBOX_ID --manage-token TOKEN [--out PATH] [--json]
```

The contract should remain consistent with the rest of `mbr`:

- JSON output on every command
- explicit lifecycle states
- machine-readable next-step guidance
- no browser-only management dependency

### Domain Model

Entity: Sandbox
- `id`: public sandbox identifier
- `name`: optional human-friendly name
- `owner_email`: bootstrap contact for creation and follow-up
- `status`: lifecycle status
- `manage_token_hash`: server-side hash of management token
- `subdomain`: assigned hosted hostname
- `instance_urls`: app, api, admin, and GraphQL URLs
- `created_at`: creation timestamp
- `expires_at`: expiry timestamp
- `extended_until`: optional later expiry after extension
- `seed_mode`: blank or seeded
- `export_status`: none, requested, ready, downloaded

State transitions:

```text
REQUESTED -> PROVISIONING -> READY -> EXPIRED
                             -> DESTROYED
                             -> EXPORTED
READY -> EXTENDED -> EXPIRED
```

Business rules and invariants:

- sandboxes are disposable and time-bounded
- sandbox credentials and management tokens are distinct from production admin
  ownership
- sandbox URLs are vendor-hosted and must be clearly marked as evaluation-only
- export must not pretend to be production promotion; it is a handoff aid

## Control Plane Shape

The hosted sandbox control plane should eventually cover:

- runtime provisioning and teardown
- DNS/subdomain assignment
- TLS issuance
- lifecycle tracking and expiry enforcement
- seeded demo or blank runtime selection
- manage-token issuance for non-authenticated lifecycle operations
- observability and abuse controls

The expected runtime shape is intentionally narrower than production. A sandbox
should expose the real product model, but it does not need the same operational
depth or customer-specific deployment flexibility as an owned instance.

## Trust Model

Hosted sandboxes must remain visibly separate from production.

- they are for inspection and evaluation
- they are time-limited
- they should be treated as lower-trust than customer-owned instances
- production decisions still belong on the owned-instance path

This distinction is part of the product model, not just an infrastructure note.

## Export and Handoff

The sandbox story should include a clean handoff into self-hosting.

The minimum expected handoff path:

- export evaluation state, configuration, and relevant content
- generate machine-readable guidance for the next owned deployment step
- make it obvious which parts transfer cleanly and which do not

The sandbox should help a customer get to an owned instance, not become a trap
that delays that move indefinitely.

## Commercial Model

If resurrected, the commercial model should stay simple.

Possible shape:

- short default duration for evaluation
- optional paid extension of the sandbox lifetime
- possible inclusion of selected first-party extension trials
- eventual path into a managed offering or a self-hosted owned deployment

The hosted sandbox path should not displace the self-hosted core business
model.

## Public Site Positioning

If brought back, the public site should describe hosted sandboxes as:

- optional
- evaluation-only
- CLI-first
- separate from the production trust model

It should not let the sandbox path redefine Move Big Rocks as a conventional
vendor-managed SaaS product.

## Why Deferred Now

This path is deferred because current effort is better spent on:

- tightening the self-hosted quickstart
- making one-VPS deployment easy for humans and agents
- sharpening the extension story
- keeping the trust model and commercial model clear

The design is preserved here so it can be resurrected later without having to
reconstruct the shape from memory.

## Resurrection Checklist

- [ ] decide whether demand justifies the hosted path
- [ ] restore the sandbox CLI family and CLI contract docs
- [ ] restore public bootstrap guidance for sandbox creation
- [ ] define DNS, TLS, and provisioning automation
- [ ] define seeded versus blank sandbox modes
- [ ] define expiry, extension, and destruction rules
- [ ] define export and owned-instance handoff format
- [ ] define abuse prevention and operational guardrails
- [ ] define pricing and extension-trial rules
- [ ] update public site copy and README once the path is truly available

## Verification Criteria

### Acceptance Criteria

- [ ] a user can create a hosted sandbox from `mbr` and receive a usable URL
- [ ] a user can inspect lifecycle state, extend, destroy, and export the
  sandbox through the CLI
- [ ] the public site clearly distinguishes sandbox evaluation from owned
  production
- [ ] the handoff into an owned instance is documented and machine-readable
- [ ] the sandbox path does not weaken the self-hosted production story

## Open Questions

- [ ] should the default sandbox be blank, seeded, or selectable?
- [ ] what is the right default duration?
- [ ] should selected first-party off-the-shelf extensions be available during
  sandbox evaluation by default?
- [ ] what is the cleanest export format for handoff into an owned instance?
- [ ] when does demand justify investing in the control plane?

## Related

- **Issue:** [platform#1](https://github.com/MoveBigRocks/platform/issues/1)
- **Related docs:** [README.md](../../README.md)
- **Supersedes:** none

---

## Changelog

| Date | Author | Change |
|------|--------|--------|
| 2026-03-25 | Adrian McPhee | Initial draft |
