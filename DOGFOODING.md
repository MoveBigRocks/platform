# Dogfooding: How We Use Move Big Rocks

Move Big Rocks is run operationally for the DemandOps portfolio. This
document shows what we actually use, what is still manual, and what a
new adopter can safely imitate.

## What the portfolio looks like

DemandOps operates multiple products across brands and countries under
Move Big Rocks BV, with handelsnamen including DemandOps, Simple is
Advanced, and TuinPlan. The core operating need was: do not duplicate
SaaS subscriptions, licenses, integrations, identities, and hidden
workflows for each brand.

## What runs on Move Big Rocks today

| Surface | Status | Notes |
|---------|--------|-------|
| Customer support intake | Live | Forms, queues, conversations, cases |
| Error tracking | Live | Sentry-compatible ingest via the error tracking extension |
| Web analytics | Live | Cookie-free analytics via the web analytics extension |
| ATS / recruiting | Live | Careers site and candidate workflows via the ATS extension |
| Sales pipeline | Live | Opportunity tracking via the sales pipeline extension |
| Knowledge management | Live | RFCs, templates, runbooks, strategic context |
| Automation | Live | Event-driven rules for routing and notifications |
| Community feature requests | Available | Installed, not yet promoted publicly |

## Which extensions are in active daily use

1. **Error tracking** — core to the daily stack. Every product reports here.
2. **Web analytics** — core to the daily stack. All public sites tracked.
3. **ATS** — active for recruiting across the portfolio.
4. **Sales pipeline** — active for opportunity management.

## What is still manual

- Instance upgrades are triggered manually (deploy script, not auto-update)
- Backup verification is manual spot-check (Litestream runs continuously)
- Extension installs are CLI-driven, not self-service UI yet
- No public sandbox environment yet (planned Q2 2026)

## What is still experimental

- Community feature requests extension is installed but not promoted
- Enterprise Access (SSO/OIDC) is available but not yet needed internally
- Marketplace infrastructure does not exist yet
- Third-party extensions do not exist yet

## What external tools still remain

- GitHub (source control, CI/CD, issue tracking)
- Postmark (transactional email delivery)
- DigitalOcean (VPS hosting, object storage)
- Stripe (not yet live — pending BV registration)
- LinkedIn (marketing, not operational)

These are infrastructure and distribution tools, not operational SaaS that
MBR replaces. The point is not zero external tools. The point is that the
operational stack — support, analytics, error tracking, recruiting, CRM,
knowledge, automation — runs on one owned system.

## What decisions were driven by real operations

- **Per-instance pricing, not per-user** — because the portfolio has
  varying team sizes per brand and per-seat pricing penalises growth
- **Extension model** — because each brand needs slightly different
  capabilities but must share identity, audit, and routing
- **Self-hosted** — because vendor dependency across brands and countries
  creates compounding license, compliance, and integration costs
- **Source-available, not open source** — because the platform is a
  commercial asset that funds the portfolio

## What a new adopter should copy first

1. Deploy one instance with core only (forms, queues, conversations, cases,
   knowledge)
2. Install error tracking or web analytics as the first extension — they
   deliver immediate visible value with minimal configuration
3. Add support intake forms and route to a team queue
4. Use knowledge resources for team runbooks and templates
5. Add more extensions only when you hit a real operational need

Do not start with all extensions. Start with the workflow that is most
fragmented today and replace that first.
