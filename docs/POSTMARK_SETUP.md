# Postmark Setup

This is the practical setup guide for using Postmark with Move Big Rocks for both:

- outbound mail
- inbound support email via webhooks

It is written so an operator or agent can execute it with minimal guesswork.

## What Move Big Rocks Expects

For outbound email:

- `EMAIL_BACKEND=postmark`
- `POSTMARK_SERVER_TOKEN`
- `FROM_EMAIL`
- `FROM_NAME`

For inbound email:

- `POSTMARK_WEBHOOK_SECRET`
- the Postmark inbound webhook configured to call Move Big Rocks
- a recipient address format that Move Big Rocks can map to a workspace

The live webhook endpoints are:

- `/v1/webhooks/postmark/{secret}/inbound`
- `/v1/webhooks/postmark/{secret}/bounce`
- `/v1/webhooks/postmark/{secret}/delivery`

See the handler wiring in [internal/infrastructure/routes/v1/router.go](https://github.com/movebigrocks/platform/blob/main/internal/infrastructure/routes/v1/router.go) and [internal/service/handlers/postmark_webhooks.go](https://github.com/movebigrocks/platform/blob/main/internal/service/handlers/postmark_webhooks.go).

## Recommended Production Shape

Use:

- app: `https://app.example.com`
- admin: `https://admin.example.com`
- api: `https://api.example.com`
- support mail domain: `support.app.example.com`

Recommended recipient format:

- `<workspace-id>@support.app.example.com`

Supported alternate format:

- `support@<workspace-slug>.app.example.com`

The simplest operational path is the first one, because it avoids ambiguity and
does not depend on slug conventions.

## Postmark Outbound Setup

1. Create or choose a Postmark server for production mail.
2. Verify the sender signature or domain you will use for `FROM_EMAIL`.
3. Put the server token into the instance repo secret:
   - `POSTMARK_SERVER_TOKEN`
4. Set these runtime values:
   - `EMAIL_BACKEND=postmark`
   - `FROM_EMAIL=support@your-domain`
   - `FROM_NAME=Your Support Team`

Move Big Rocks sends outbound mail through the Postmark adapter in [internal/service/services/email_providers.go](https://github.com/movebigrocks/platform/blob/main/internal/service/services/email_providers.go).

## Postmark Inbound Setup

1. Generate a strong random webhook secret.
2. Store it as:
   - `POSTMARK_WEBHOOK_SECRET`
3. Configure Postmark inbound webhooks to point to:

```text
https://api.your-app-domain/v1/webhooks/postmark/<POSTMARK_WEBHOOK_SECRET>/inbound
https://api.your-app-domain/v1/webhooks/postmark/<POSTMARK_WEBHOOK_SECRET>/bounce
https://api.your-app-domain/v1/webhooks/postmark/<POSTMARK_WEBHOOK_SECRET>/delivery
```

4. Configure the inbound message stream or routing so mail for your chosen
   support address format reaches Postmark.
5. Send a test message to a real workspace address and verify:
   - the webhook returns `200`
   - the inbound email record is created
   - the email event is published
   - the resulting case/threading behavior is correct

## Workspace Routing Rules

Move Big Rocks currently extracts the target workspace from the recipient address.

Supported patterns:

- `<workspace-id>@support.app.example.com`
- `support@<workspace-slug>.app.example.com`
- `support@<workspace-slug>.support.app.example.com`

If you want the least surprising path, use `<workspace-id>@support...`.

## What The Agent Should Do

If an agent is setting this up, it should:

1. verify the instance repo declares Postmark as the outbound provider
2. verify the required secrets exist
3. print the exact webhook URLs using the configured API base URL and webhook secret
4. tell the user exactly what to paste into Postmark
5. verify outbound mail with a live test
6. verify inbound mail with a live test
7. verify bounce and delivery webhooks are reachable
8. report any mismatch between the configured support mailbox format and the
   workspace-routing rules

## Current Limits

- inbound routing is address-pattern based, not a full helpdesk mailbox router
- Move Big Rocks is not a full mail server
- Postmark is the recommended path for Milestone 1 because it keeps both
  outbound and inbound on a simple webhook-based model

## Recommended Next Check

After configuring Postmark, run:

- an outbound test email
- an inbound test email
- a bounce test if available
- a delivery confirmation test if available

Do not treat the integration as finished until all four pass.
