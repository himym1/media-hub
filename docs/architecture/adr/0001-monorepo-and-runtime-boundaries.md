# ADR 0001: Monorepo and Runtime Boundaries

- Status: Accepted
- Date: 2026-08-11

## Context

Media Hub needs a NAS control service, a browser interface, and an Android application. They share API semantics but have different runtime and UI constraints.

## Decision

Use one Git monorepo with `backend`, `web`, `android`, and one shared OpenAPI contract.

Deploy two artifacts:

1. A NAS container containing the Go service and built Web assets.
2. A native Android APK.

## Consequences

- API changes can be reviewed with both clients in one change.
- Web and Android remain free to use platform-appropriate UI implementations.
- Build pipelines are independent and can fail separately.
- The repository must avoid coupling release cadence through shared UI code.
