# ADR 0004: Material 3 for Android presentation

- Status: Accepted
- Date: 2026-08-25
- Supersedes: [ADR 0003](0003-native-android-compose-miuix.md)

## Context

Miuix gave the Android client a HyperOS settings look. That language fought Media Hub's media-first screens, stayed experimental, and split the product from the already-dark Web client.

Material 3 already powered list-detail and the player slider. Its 1.4.0 stable artifact exposes SearchBar, SegmentedButton, FilterChip, NavigationBar, and AlertDialog. The `MaterialExpressiveTheme` / `MotionScheme` APIs exist in that jar but are still `internal`; public Expressive theme entry points remain on the 1.5.0 alpha line.

## Decision

- Drop Miuix.
- Theme the app with public `MaterialTheme` using a dark, large-radius, high-contrast scheme that follows Material 3 Expressive tactics (shape variety, emphasized titles, clearer hierarchy).
- Keep business screens on project-owned `MediaHub*` wrappers.
- Stay on stable `androidx.compose.material3:1.4.0`. Do not take 1.5.0-alpha only to call `MaterialExpressiveTheme`.
- Keep `material3-adaptive` for two-pane library/search.

## Consequences

- Android chrome matches a single Material 3 language instead of HyperOS + custom tokens.
- Wrapper-level migration is required when Material 3 components change.
- Morphing button shapes and the official motion scheme wait until those APIs are public and stable.
