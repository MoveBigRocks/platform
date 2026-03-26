# Sentry SDK Integration Tests

These tests verify the error-tracking surface inside Move Big Rocks, the AI-native service operations platform, using real Sentry SDKs.

## Quick Start

```bash
# Start the API locally before running the SDK smoke tests below.
go run ./cmd/api
```

## Full SDK Tests

Require a running server (`go run ./cmd/api`):

```bash
# Python
pip install sentry-sdk requests
export SENTRY_TEST_DSN="http://test-public-key:test-secret-key@localhost:8080/test-project-key"
python tests/sentry_sdk_integration_test.py

# Node.js
cd tests && npm install
npm run test
```

## Coverage

| Test | What it verifies |
|------|------------------|
| Python test | Real sentry-sdk: exceptions, messages, breadcrumbs, tags |
| Node.js test | Real @sentry/node: exceptions, stack traces, user context |

## Troubleshooting

- **Connection refused**: Start server first
- **401 Unauthorized**: Create project with DSN key `test-public-key`
