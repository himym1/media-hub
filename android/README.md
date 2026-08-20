# Android App

Native Android client for Media Hub.

- Kotlin 2.4.10
- Jetpack Compose
- Miuix 0.9.3 for HyperOS chrome (Scaffold, cards, preference rows), isolated behind project-owned UI wrappers
- Lucide Compose icons
- Navigation 3 stable

The client uses the shared authenticated Media Hub API for discovery, subscriptions, transfers, library access, service status, 115 operations, and migration recovery. Session tokens are encrypted with Android Keystore before private persistence; release builds reject cleartext API URLs.

The signed release APK defaults to `https://media.himym.us.ci`, so a fresh install opens the login flow without asking for a server address. A server URL saved by the user takes precedence, and the Services screen retains the explicit server-change flow for migration or recovery deployments.
