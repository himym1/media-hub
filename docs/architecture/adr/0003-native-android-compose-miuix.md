# ADR 0003: Native Android with Compose and Miuix

- Status: Superseded by [ADR 0004](0004-material3-expressive.md)
- Date: 2026-08-11

## Context

The real mobile use case is Android. Cross-platform support would add build, design, and testing cost without a current user. The product needs polished native interaction, notifications, deep links, background status refresh, and reliable Android lifecycle handling.

Miuix provides a distinctive Compose component language but explicitly describes its API as experimental.

## Decision

Build an Android-only native app with:

- Kotlin 2.4.10;
- Android Gradle Plugin 9.3.1;
- Gradle 9.5;
- Jetpack Compose;
- Navigation 3 stable;
- Miuix 0.9.3 pinned exactly;
- minSdk 26 and compile/target SDK 37.

Miuix is a presentation dependency only. Project-owned `MediaHub*` wrappers isolate Miuix imports from feature screens.

The app follows unidirectional data flow with ViewModel, StateFlow, repositories, and explicit loading/empty/error/content states.

## Rejected Alternatives

- Flutter: unnecessary cross-platform layer and duplicated Web semantics.
- Capacitor: weaker native lifecycle and background integration for an Android-only product.
- Compose Multiplatform project structure: no current non-Android target.
- Direct Miuix use in every screen: unacceptable API-change blast radius.

## Consequences

- Android receives a first-class native experience.
- Web and Android do not share UI code.
- Miuix upgrades require wrapper-level migration and visual regression checks.
- SDK Platform 37 is a required development prerequisite.
