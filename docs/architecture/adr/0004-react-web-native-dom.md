# ADR 0004: React Web with Native DOM

- Status: Accepted
- Date: 2026-08-11

## Context

The Web client is used for search, task inspection, and configuration on desktop and mobile browsers. It needs accessible text, keyboard interaction, dense operational layouts, and conventional browser behavior.

## Decision

Use React, TypeScript, Vite, TanStack Query, Tailwind CSS, and Lucide icons. Keep standard DOM semantics and use project-owned components rather than shipping a default component-library theme.

Do not use Compose Web/Canvas or server-side rendering in the MVP.

## Consequences

- The Web UI remains accessible, inspectable, and efficient.
- Android and Web share contract semantics, not UI code.
- The production Go service can embed the generated static assets.
- A separate Node server is not required in production.
