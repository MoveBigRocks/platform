# RFC-0005: Operational Health Extension

## Status

draft

## Summary

Operational health should be an optional first-party extension that can monitor endpoints, record probe results, and attach health signals to shared operational data.

## Design

- the extension defines probes and schedules
- probe results can create or update cases when configured
- `Application` remains available as a shared target identity when needed
- agents access probe state through CLI commands and GraphQL
- Move Big Rocks core keeps its own internal health checks regardless of installed extensions

## Why Extension, Not Core

- not every deployment needs it
- it shares primitives with the rest of the platform
- it should be purchasable and installable like other product packs
