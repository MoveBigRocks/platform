# Extension Authoring And Validation Plan

**Document version:** 2026-03-27  
**Status:** Proposed roadmap and current-state audit

## Purpose

This document defines how Move Big Rocks should evolve extension authoring and
validation so that:

- human developers can build extensions with confidence
- agents can understand the extension lifecycle without improvising
- first-party and customer-built packs can prove they are compliant
- extension API and SDK changes can roll out without silent breakage

It also records the current validation baseline for the existing first-party
extensions.

## Product Goal

The extension development story should make this promise:

- if an extension passes the documented verification stack, it should install,
  validate, activate, appear in the expected admin surfaces, and exercise its
  primary workflow without hidden steps
- if the extension contract changes, the SDK, first-party packs, and docs
  should make the breakage obvious before production

That promise matters for both humans and agents. The lifecycle needs to be
machine-checkable, not just socially understood.

It also needs to hold for instance admins outside a workspace context. A
workspace-scoped extension should not collapse into a 404 or dead menu entry
just because the current admin session is instance-scoped.

## Current Architecture Baseline

Today, the extension system already has a solid core runtime model:

- `manifest.json` is the source of truth for runtime class, storage class,
  routes, endpoints, admin navigation, widgets, seeds, commands, and skills
- extensions move through `install -> validate -> activate -> monitor`
- `bundle` and `service_backed` runtime classes are explicit
- `shared_primitives_only` and `owned_schema` storage classes are explicit
- service-backed packs can register health endpoints, event consumers,
  scheduled jobs, and owned-schema migrations
- admin navigation and dashboard widgets resolve through platform-owned logic
  rather than each extension inventing its own menu wiring

Relevant current sources:

- `platform/internal/platform/domain/extension.go`
- `platform/internal/platform/services/extension_service.go`
- `platform/internal/platform/services/extension_runtime.go`
- `platform/internal/platform/services/extension_admin_navigation.go`
- `platform/docs/EXTENSION_SECURITY_MODEL.md`
- `platform/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`

## What Already Works Well

### Structural validation in core

The platform already validates a meaningful amount before activation:

- manifest shape and required fields
- runtime and storage combinations
- service-backed health endpoint requirements
- endpoint and asset consistency
- admin navigation and dashboard widget endpoint references
- event consumer and scheduled job contracts
- route topology conflicts
- customization and agent-skill asset presence

### Runtime diagnostics

The CLI already exposes useful proof for installed extensions:

- `mbr extensions show --id EXTENSION_ID`
- `mbr extensions monitor --id EXTENSION_ID`

Those surfaces already show:

- declared endpoints
- declared admin navigation
- declared dashboard widgets
- declared commands and skills
- validation state
- health state
- runtime endpoint, consumer, and job diagnostics

### Internal regression coverage

The platform repo already contains strong internal tests for:

- manifest validation
- install and activate behavior
- route resolution
- admin navigation resolution
- schema migrator behavior
- bundle verification
- selected first-party packs

Key current tests include:

- `platform/internal/platform/domain/extension_test.go`
- `platform/internal/platform/services/extension_service_test.go`
- `platform/internal/platform/services/extension_runtime_test.go`
- `platform/internal/platform/services/extension_admin_navigation_test.go`
- `platform/internal/platform/services/first_party_extension_packages_test.go`
- `platform/internal/infrastructure/stores/sql/extension_schema_migrator_test.go`

## Current Gaps

The main problem is not that nothing exists. The main problem is that the best
validation surfaces are still internal and inconsistent.

### Gap 1: No public SDK-owned contract layer

External extension repos cannot currently reuse the strongest helpers because
they live under `platform/internal/...`.

That means customer-built extensions still depend on:

- local repo tests
- `install -> validate -> activate -> monitor`
- manual clicking

This is not enough for a reliable authoring contract.

### Gap 2: No public proof of resolved admin navigation

Status: addressed in the first implementation pass on March 27, 2026.

The platform can resolve real admin navigation entries and widget hrefs, but
that logic is only exposed through internal service methods today.

That makes it hard for an external extension author to prove:

- the extension really appears in the menu
- it appears under the correct section
- the resolved href matches the expected mount path
- the extension still appears for an instance admin without a workspace selected
- the instance-level href identifies the intended workspace install

### Gap 3: No standard way to assert seeded resources

Status: partially addressed in the first implementation pass on March 27, 2026.

Extensions can seed queues, forms, and automation rules, but the SDK does not
yet give authors a standard assertions file or verifier for proving that the
correct resources were created. Public CLI and GraphQL inspection now exist,
but a contract file and one-command verifier still need to be added.

### Gap 4: Inconsistent first-party extension coverage

Some first-party packs have strong domain tests, some have only install or
template smoke tests, and not every pack is held to the same gold standard.

### Gap 5: Docs are still fragmented

The extension lifecycle exists across:

- `extension-sdk/README.md`
- `extension-sdk/START_HERE.md`
- `extension-sdk/TESTING.md`
- `platform/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`
- `platform/docs/EXTENSION_SECURITY_MODEL.md`
- public site extension pages

The pieces are useful, but they still do not read as one contract-driven story.

### Gap 6: Internal contract types are not public authoring primitives

Important extension contract logic currently lives in internal Go packages. That
is correct for platform internals, but not sufficient for external authoring.

We should not expect external extension repos to reverse-engineer internal
validation behavior from implementation files.

## Design Principle

The right model is:

- one public extension contract
- one public verifier
- one machine-readable assertion file per extension
- one clear human and agent workflow
- first-party packs as canaries for contract drift

The public contract should be the source of truth. The platform should consume
it. The SDK should expose it. The docs should describe it. The site should
teach it.

## Target Validation Model

Use six layers.

### Layer 0: Extension-local tests

Every extension repo should own fast local tests for its own code:

- template parsing
- domain logic
- handler behavior
- seeding helpers
- data mapping
- store behavior where practical

These remain extension-specific and fast.

### Layer 1: Public offline lint

Add a public offline verifier:

- `mbr extensions lint SOURCE_DIR`

This should validate:

- manifest schema
- normalization rules
- asset existence
- route and endpoint consistency
- admin navigation references
- widget references
- health endpoint rules
- job and consumer rules
- command namespacing
- skill asset references

This command should not require a running instance.

### Layer 2: Public install-and-prove verification

Add a public installed verification flow:

- `mbr extensions verify SOURCE_DIR --workspace WORKSPACE_ID`

This should:

1. install from source
2. run platform validation
3. activate the extension
4. check runtime health
5. inspect resolved admin navigation and widgets
6. inspect seeded resources
7. assert declared commands and skills
8. optionally run one extension-defined smoke flow

This is the main bridge between the platform contract and the SDK lifecycle.

### Layer 3: Extension contract assertions file

Each extension repo should be able to declare its expected proof in a machine-
readable file, for example:

- `extension.contract.json`

Suggested contents:

- expected admin navigation section, title, and href
- expected dashboard widgets
- expected instance-admin/no-workspace visibility
- expected public paths
- expected health endpoint
- expected seeded queue slugs
- expected seeded form slugs
- expected seeded automation rule keys
- expected commands
- expected agent skills
- optional smoke workflow metadata

This file matters because agents can read it directly and reason about what the
extension should prove.

### Layer 4: Browser and HTTP smoke tests

Extensions with UI should add a real smoke layer:

- HTTP checks for public and admin routes
- browser checks for menu visibility and rendering
- one primary workflow end-to-end

This is where we prove that the pack really works in the shell, not just on
paper.

### Layer 5: Release and compatibility canaries

Every contract change should be proven against:

- the SDK template
- all first-party public packs
- at least one service-backed owned-schema pack
- at least one bundle-first shared-primitives pack

This is how we keep the extension platform safe while it evolves.

## Public Interfaces To Add

To make the lifecycle reusable, the following interfaces should become public
SDK or public API surfaces rather than staying internal-only.

### 1. Public contract package

Extract or duplicate the extension contract types into a public package owned by
the SDK or a shared public module.

That package should define:

- manifest types
- validation rules
- normalization rules
- assertion-file types
- bundle source loading helpers

The platform should consume the same public contract definitions where possible.

### 2. Public resolved-navigation query

Expose resolved admin navigation and widgets through public GraphQL and CLI
surfaces.

Status: implemented.

Suggested additions:

- GraphQL query for workspace-resolved extension admin navigation
- GraphQL query for workspace-resolved extension dashboard widgets
- GraphQL query for instance-resolved extension admin navigation
- GraphQL query for instance-resolved extension dashboard widgets
- CLI command such as `mbr extensions nav --workspace WORKSPACE_ID`
- CLI command `mbr extensions widgets --workspace WORKSPACE_ID`
- CLI command `mbr extensions nav --instance`
- CLI command `mbr extensions widgets --instance`
- or `mbr extensions show --resolved --workspace WORKSPACE_ID`

This is mandatory if we want authors to prove menu placement without importing
internal Go code.

### 3. Public seeded-resource inspection

Expose seeded queues, forms, and automation rules in a pack-oriented way.

Suggested additions:

- GraphQL query returning extension-owned seeds for an installed extension
- CLI command such as `mbr extensions resources --id EXTENSION_ID`

### 4. Public verifier harness

Ship a verifier harness in the SDK that can:

- load a source tree
- produce install inputs
- install into a workspace
- fetch runtime proofs
- compare them with `extension.contract.json`

This should replace the current "copy internal helper ideas manually" pattern.

## First-Party Gold Standard

Every first-party extension should eventually meet this standard.

### For bundle-first shared-primitives packs

Required proof:

- manifest lint passes
- install, validate, and activate pass
- resolved admin navigation is correct
- seeded queues, forms, and rules are correct
- one primary workflow succeeds
- declared skills and commands are inspectable

### For service-backed owned-schema packs

Required proof:

- all of the above
- health endpoint passes
- runtime diagnostics show registered endpoints
- migrations apply cleanly
- event consumers and scheduled jobs register correctly
- one primary service-backed workflow succeeds

## Current Audit Of First-Party Extensions

Audit date: 2026-03-27

Current proof run executed:

- `go test ./...` in `extensions` -> passed
- `go test ./internal/platform/services -run 'TestFirstParty|TestExtensionService_ListWorkspaceAdminNavigationAndWidgets|TestExtensionService_ResolvePublicServiceRoute|TestExtensionService_ResolveAdminServiceRouteByWorkspace|TestExtensionService_CheckExtensionHealth'` in `platform` -> passed

### Coverage matrix

| Extension | Runtime shape | Current proof | Main gaps |
| --- | --- | --- | --- |
| `ats` | `bundle` + `shared_primitives_only` | Domain tests for ATS vocabulary and catalog; install/activate/workflow proof in `first_party_extension_packages_test.go`; shared-primitives seeding and case-tagging behavior is verified; checked-in `extension.contract.json`; instance-admin navigation proof | No public/admin route HTTP smoke; no standard asset/template parse check |
| `web-analytics` | `service_backed` + `owned_schema` | Domain, handler, resolver, and service tests in the extension repo; admin navigation resolution test via reference install; public route resolution tests; schema migrator tests; checked-in `extension.contract.json`; instance-admin navigation proof | No explicit first-party package install/activate/nav test in the pack suite; no UI/template parse smoke; no end-to-end create-property plus ingest workflow proof |
| `error-tracking` | `service_backed` + `owned_schema` | Domain and service tests in the extension repo; admin navigation resolution test via reference install; public ingest route resolution tests; schema migrator tests; checked-in `extension.contract.json`; instance-admin navigation proof | No explicit first-party package install/activate/nav test in the pack suite; no runtime UI/template parse smoke; no end-to-end ingest-to-issue workflow proof |
| `sales-pipeline` | `service_backed` + `owned_schema` | Install/activate/seed/nav proof in `first_party_extension_packages_test.go`; template embed parse test exists; checked-in `extension.contract.json`; instance-admin navigation proof | No runtime domain/store/handler tests; no end-to-end create-deal and move-stage proof; no explicit runtime registration and health proof |
| `community-feature-requests` | `service_backed` + `owned_schema` | Install/activate/seed/nav proof in `first_party_extension_packages_test.go`; template embed parse test exists; checked-in `extension.contract.json`; instance-admin navigation proof | No runtime domain/store/handler tests; no end-to-end submit/vote/admin-update proof; no explicit runtime registration and health proof |

### Immediate conclusions from the audit

- ATS has the strongest proof of shared-primitives workflow behavior.
- Web analytics and error tracking have the strongest service-backed runtime and
  owned-schema coverage, but the first-party pack audit story is still indirect.
- Sales pipeline and community feature requests now install correctly, but they
  need deeper runtime tests before they match the maturity of the older packs.
- Every current first-party pack now has a checked-in `extension.contract.json`,
  but deeper runtime and browser proof is still uneven.

## Recommended Implementation Phases

### Phase 1: Make the current lifecycle legible

Goal:

- make the existing validation story easy to find for humans and agents

Work:

- keep `extension-sdk/README.md`, `START_HERE.md`, and `TESTING.md` aligned
- add one platform-level roadmap doc for validation and authoring
- update public docs and site pages to point at the testing strategy
- make the first-party extension audit visible in repo docs

Expected outcome:

- authors understand the current lifecycle without reading internal code

### Phase 2: Publish the contract

Goal:

- stop relying on internal platform types as the authoring truth

Work:

- define a public manifest contract package
- define `extension.contract.json` and ship it in the SDK plus first-party packs
- publish JSON schema or equivalent machine-readable definitions
- move normalization and validation logic behind public APIs where possible

Expected outcome:

- external extension repos can reason about the same contract as the platform

### Phase 3: Add public proof surfaces

Goal:

- let extension authors prove menu placement and seeded resources

Work:

- add resolved admin navigation queries
- add resolved dashboard widget queries
- add seeded resource inspection queries
- expose them in the CLI

Expected outcome:

- authors can prove "this appears in the menu" without internal helpers

### Phase 4: Ship SDK verifier tooling

Goal:

- replace ad hoc verification with a standard command

Work:

- implement `mbr extensions lint`
- implement `mbr extensions verify`
- add SDK verifier harness helpers
- add source-tree assertion loading from `extension.contract.json`

Expected outcome:

- every extension repo can run the same contract verification flow

### Phase 5: Bring first-party packs to gold standard

Goal:

- make first-party packs the proof that the contract works

Work:

- add explicit contract files for ATS, web analytics, error tracking,
  sales pipeline, and community feature requests
- add missing install/activate/nav proofs where indirect coverage exists today
- add missing runtime tests for the newer service-backed packs
- add smoke flows for the primary workflow of every first-party extension

Expected outcome:

- first-party packs become compatibility canaries and reference implementations

### Phase 6: Add browser-level proof

Goal:

- prove the extension shell integration really renders

Work:

- add browser smoke for menu presence and main page render
- add at least one critical-path UI flow per major extension

Expected outcome:

- UI regressions become detectable before release

## First-Party Backlog By Extension

### ATS

Priority work:

- add an explicit contract file
- add public page HTTP smoke for careers index and apply path
- add admin page render smoke
- add asset/template verification for bundled HTML surfaces

### Web analytics

Priority work:

- add explicit first-party package install/activate/nav assertions
- add contract file
- add runtime UI/template smoke
- add end-to-end smoke for property creation, script verification, and ingest
- assert scheduled-job registration through verifier output

### Error tracking

Priority work:

- add explicit first-party package install/activate/nav assertions
- add contract file
- add runtime UI/template smoke
- add end-to-end smoke for envelope ingest to issue visibility
- assert consumer registration through verifier output

### Sales pipeline

Priority work:

- add runtime domain tests
- add handler and store tests
- add contract file
- add end-to-end smoke for deal creation and stage movement
- add runtime health and registered-endpoint assertions

### Community feature requests

Priority work:

- add runtime domain tests
- add handler and store tests
- add contract file
- add end-to-end smoke for submit, vote, and admin status update
- add runtime health and registered-endpoint assertions

## CI Model

The steady-state CI story should look like this.

### Extension repo CI

Every extension repo should run:

1. language-level tests such as `go test ./...`
2. `mbr extensions lint .`
3. `mbr extensions verify . --workspace ws_preview`
4. extension-specific smoke tests
5. browser smoke tests for UI-heavy packs

### Platform repo CI

The platform repo should run:

1. core contract tests
2. first-party canary pack tests
3. compatibility tests against the SDK template
4. migration and runtime registration tests for service-backed packs

### Release gating

First-party public bundle publication should require:

- pack repo tests pass
- SDK verifier pass
- platform compatibility canaries pass
- docs and migration notes are updated for contract changes

## Agent Experience

The extension authoring lifecycle needs to be explicit for agents as well as
humans.

The SDK should therefore contain:

- one `START_HERE.md`
- one testing strategy
- one machine-readable assertion file
- one verifier command
- one clear "done means proven" checklist

Agents should never have to infer:

- how to prove menu placement
- how to prove seed creation
- how to prove runtime health
- how to know when an extension is ready for production

## Versioning And Compatibility

The extension platform needs an explicit compatibility model.

### Recommended versioned surfaces

Keep separate versions for:

- manifest schema version
- assertion-file schema version
- SDK verifier version
- public contract package version

### Additive changes

For additive contract changes:

- keep the existing manifest schema version if the change is optional
- update the public contract package
- update the verifier
- update first-party contract files where relevant
- update docs

### Breaking changes

For breaking contract changes:

1. introduce a new manifest or assertion-file schema version
2. update the SDK verifier first
3. update first-party packs as canaries
4. keep at least one previous contract version supported during migration
5. publish a migration guide before expecting external repos to move

### Compatibility promise

The goal should be:

- first-party packs are always green on the current platform main branch
- the SDK template is always green on the current platform main branch
- external authors get a clear verifier error instead of a mysterious runtime
  failure when the contract changes

## Docs And Public Site Changes To Plan

### SDK docs

Update:

- `extension-sdk/README.md`
- `extension-sdk/START_HERE.md`
- `extension-sdk/TESTING.md`

Needed additions:

- clear validation stack
- contract file explanation
- verifier command explanation
- agent-oriented "how to prove ready" checklist

### Platform docs

Update:

- `platform/docs/INSTANCE_AND_EXTENSION_LIFECYCLE.md`
- `platform/docs/EXTENSION_SECURITY_MODEL.md`
- `platform/docs/AGENT_CLI.md`

Needed additions:

- resolved-navigation public interface
- verifier commands
- compatibility and migration policy
- first-party canary expectations

### Public site

Update:

- `/build-extensions`
- `/extensions`
- CLI docs page

Needed additions:

- current validation story
- SDK testing strategy link
- explanation of how custom extensions are proven before production
- explanation that first-party packs are the reference canaries

## Recommended Next Sequence

If we want the highest leverage order, do this next:

1. publish a public SDK harness that mirrors `lint` and `verify`
2. add HTTP smoke helpers for public and admin routes
3. add browser-level checks for menu visibility and critical UI flows
4. wire the first-party packs into the stricter CI path continuously
5. keep public docs and site pages aligned with the verifier-first lifecycle
6. version and publish the contract package for external repos

That order gives us the smallest path from today's partially internal system to
a real public extension authoring contract.
