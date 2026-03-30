# ADR 0027: Explicit Fleet Registration and Disclosed Heartbeat via MBR Fleet

**Status:** Accepted
**Date:** 2026-03-30

## Context

Move Big Rocks needs a trustworthy way to understand real deployment adoption
before paid extension pricing, managed hosting transitions, and grandfathering
commitments become operationally important.

We specifically need:

- a contactable installed base
- a verifiable registration record for grandfathering and support
- a coarse signal for active versus abandoned deployments
- extension-adoption visibility by deployed instance

At the same time, the self-hosted trust story cannot regress into hidden
activation checks or a runtime dependency on vendor availability.

The desired model therefore has to satisfy both sides:

- Move Big Rocks can operate a central control plane for its own fleet view
- customer-owned instances remain self-contained and do not discover other
  customer instances
- the core platform keeps running if vendor endpoints are disabled, blocked, or
  unavailable
- any callback is explicit, disclosed, and limited to coarse operational data

## Decision

### 1. Operate Fleet Tracking As A Vendor-Owned Control Plane Extension

Move Big Rocks will run fleet tracking through a private first-party extension,
`mbr-fleet`, installed on the DemandOps Move Big Rocks instance in a dedicated
workspace.

That extension is the only global fan-in point for:

- instance registration
- heartbeat intake
- operator-facing fleet dashboards
- support visibility and grandfathering records

Customer-owned instances do not track each other. They only know their own
instance identity and the vendor-operated fleet endpoint.

### 2. Keep Registration Explicit And Human-Visible

Registration is an explicit control-plane action, not a hidden boot-time side
effect.

The intended registration path is:

- the private instance repo carries `spec.fleet` settings in `mbr.instance.yaml`
- an operator or agent runs the manual fleet-registration workflow from the
  instance repo
- that workflow calls `mbr fleet register`
- `mbr-fleet` issues or confirms the per-instance tracking secret
- the host stores that secret locally for future updates and heartbeats

Registration is not required to run the core platform. It is the basis for
support, grandfathering, and future commercial transition handling.

### 3. Make Heartbeat Host-Owned, Coarse, And Optional

Heartbeat collection is a host-owned callback, not hidden application-runtime
logic.

The intended heartbeat path is:

- a host-local timer calls the heartbeat script on a weekly cadence with jitter
- the request is authenticated with the instance tracking secret
- the payload is limited to coarse adoption and support signals
- heartbeat can be disabled in `mbr.instance.yaml`
- disabling or blocking heartbeat does not stop the core runtime

The current coarse payload includes only:

- instance ID
- platform version
- installed extension slugs and versions
- workspace count
- a coarse 30-day activity bucket

The heartbeat must not include:

- case content
- conversation content
- end-user identities
- customer records
- uploaded attachments

### 4. Disclose Fleet Tracking In Product And Deployment Surfaces

Fleet registration and heartbeat behavior must be documented consistently in:

- the private instance repo config and setup docs
- operator deployment workflows
- platform documentation
- the public site privacy and self-host guidance

This is not something to hide inside license terms alone. The operator should
understand what is sent, why it exists, and how to disable the heartbeat.

## Consequences

### Positive

- Move Big Rocks gets a central, contactable installed-base record without
  breaking the self-hosted trust story
- grandfathering commitments become auditable from a registration record rather
  than ad hoc claims
- the central fleet view can distinguish registered-only, active, and stale
  deployments
- `mbr-fleet` remains a bounded private extension instead of leaking fleet
  tracking into unrelated platform contexts

### Negative

- the vendor-operated control plane becomes one more surface Move Big Rocks must
  maintain
- some customer instances will never register or will disable heartbeat, so the
  fleet view is intentionally incomplete
- public documentation and privacy language must stay aligned with the real
  payload and workflow behavior

### Follow-On Work

- keep `mbr-fleet` limited to registration and coarse heartbeat intake until a
  real commercial entitlements surface is needed
- update the business-plan adoption model and milestone language whenever the
  implemented tracking surfaces change
- audit related docs whenever the heartbeat payload or registration workflow is
  expanded
