# Configure OIDC

Use this skill when an operator wants to enable enterprise SSO with the `enterprise-access` pack.

## Goals

- confirm the pack is installed and active
- collect the OIDC issuer, discovery URL, client ID, redirect URL, and desired claim mappings
- preserve Move Big Rocks break-glass login
- validate the provider before turning on enforcement

## Safe workflow

1. Confirm the pack is installed:
   - `mbr extensions list --instance`
   - `mbr extensions show --id EXTENSION_ID`
2. Open the settings surface:
   - `/admin/extensions/enterprise-access`
3. Configure one OIDC provider in draft mode first.
4. Test the callback flow.
5. Verify at least one break-glass admin can still log in without the provider.
6. Only then enable enforcement.

## Required inputs

- issuer or discovery URL
- client ID
- client secret reference
- redirect URL
- default scopes
- claim mapping for email and name

## Guardrails

- never disable break-glass magic-link login during initial setup
- never turn on enforcement before a successful login test
- keep provider status `draft` until validation passes
