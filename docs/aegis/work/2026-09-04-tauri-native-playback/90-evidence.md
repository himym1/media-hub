# Evidence

- `pnpm --dir web test` 27 passed
- `pnpm --dir web lint` clean
- `pnpm --dir web build` ok
- Playwright library plays: 25 passed, 1 skipped
- `cargo check` desktop/src-tauri ok
- Host has no `mpv`; native spawn will show the install error until `brew install mpv`
