# ADR 0002: Go Modular Monolith

- Status: Accepted
- Date: 2026-08-11

## Context

The expected workload is one user and a small number of concurrent workflows. The hard problem is reliable integration orchestration, not horizontal scale.

## Decision

Use a Go modular monolith with the standard HTTP server, explicit module boundaries, SQLite WAL, and a database-backed job runner.

Do not introduce microservices, Redis, Kafka, or a separate workflow engine in the MVP.

## Consequences

- Deployment is one container and one persistent volume.
- Transactions can atomically store workflow state and jobs.
- Provider adapters remain replaceable behind Go interfaces.
- Long-running work must not execute in HTTP request goroutines.
- Upgrade to PostgreSQL or separate workers only when measurements justify it.
