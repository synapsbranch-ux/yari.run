# yari.run desktop (Tauri 2)

A thin native shell around the shared UI in `apps/client`. Rust is limited to the window, the
system tray and one native notification. There is no business logic, no credentials and no provider
call in Rust: the web code talks to the API like it does in the browser.

## Layout

- `src-tauri/tauri.conf.json`: window, CSP, bundle settings. `withGlobalTauri` is on, so the web
  code reaches `window.__TAURI__` and no `@tauri-apps/*` package is bundled into the client.
- `src-tauri/src/lib.rs`: tray (open, pending approvals, agent sessions, quit) and the startup
  notification. The counters are fake; see the `TODO(shell)` comments.
- `src-tauri/capabilities/default.json`: `core:default` and `notification:default`, nothing else.
- `PLATFORM.md`: ready to paste TypeScript for `apps/client/src/lib/platform.ts`.

## Prerequisites

Rust (stable), Node 22, pnpm, and the Tauri system libraries.

Debian and Ubuntu:

```sh
sudo apt-get update
sudo apt-get install -y libwebkit2gtk-4.1-dev build-essential curl wget file libxdo-dev \
  libssl-dev libayatana-appindicator3-dev librsvg2-dev
```

macOS needs Xcode command line tools. Windows needs the Microsoft C++ Build Tools and WebView2.

## Develop and build

```sh
pnpm install
pnpm --filter client dev                    # terminal 1, serves http://localhost:5173
pnpm --filter desktop tauri dev             # terminal 2
pnpm --filter desktop tauri build           # runs `pnpm --filter client build` first
pnpm --filter desktop tauri build --debug --bundles deb   # quick Linux check
```

## Pointing the app at an API

The CSP allows network access only to `'self'`, Tauri IPC and one API origin. The default is
`http://localhost:8080`. For production, change the API origin in **both** places, which must match:

1. The client build variable `PUBLIC_API_URL` (repository `.env`), for example
   `https://api.yari.run`.
2. `connect-src` in `app.security.csp` (and `devCsp`) of `src-tauri/tauri.conf.json`: replace
   `http://localhost:8080` with the same origin, for example `https://api.yari.run`.

Use `https` for anything but local development.

## Content Security Policy

```
default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;
font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none';
connect-src 'self' ipc: http://ipc.localhost http://localhost:8080
```

Tauri adds hashes for the one inline bootstrap script of the SvelteKit static build at compile
time, so `script-src` needs no `'unsafe-inline'`. `style-src` keeps `'unsafe-inline'` because the
SvelteKit shell uses a `style="display: contents"` attribute. `ipc:` and `http://ipc.localhost` are
required by Tauri 2 for command calls.

## Icons

`pnpm --filter desktop icons` regenerates the neutral placeholder (`scripts/make-icon.mjs`) and
derives the set with `tauri icon`. Replace `src-tauri/app-icon.png` with real artwork later and
run `pnpm --filter desktop exec tauri icon src-tauri/app-icon.png`, then delete the generated
`icons/android` and `icons/ios` folders (unused here).

## CI notes

Install the apt packages above on the runner, build the client first, then run
`pnpm --filter desktop tauri build`. `cargo check` in `src-tauri` needs the same system libraries.
