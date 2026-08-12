# Android App

Native Android client for Media Hub.

- Kotlin 2.4.10
- Jetpack Compose
- Miuix 0.9.3, isolated behind project-owned UI wrappers as the feature set grows
- Lucide Compose icons
- Navigation 3 stable

The client uses the shared authenticated Media Hub API for discovery, subscriptions, transfers, library access, service status, 115 operations, and migration recovery. Session tokens are encrypted with Android Keystore before private persistence; release builds reject cleartext API URLs.

GitHub Release APK does not embed a private server domain. On first launch, enter the deployment HTTPS origin; the app stores it privately and uses the same origin for API and authenticated updates.
