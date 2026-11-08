# Desktop platform code for `apps/client/src/lib/platform.ts`

Ready to paste into `tauriImpl`. It uses only the globals injected by Tauri because
`app.withGlobalTauri` is `true` in `src-tauri/tauri.conf.json`, so the client needs no
`@tauri-apps/*` npm dependency.

Global names, verified against `tauri-plugin-notification` 2.8.0 (`api-iife.js`):
`window.__TAURI__.notification` exposes `isPermissionGranted()`, `requestPermission()` and
`sendNotification(options)`. The plugin is registered in `src-tauri/src/lib.rs` and granted by
`notification:default` in `src-tauri/capabilities/default.json`.

## Notifications

```ts
interface TauriNotificationApi {
  isPermissionGranted(): Promise<boolean>;
  requestPermission(): Promise<'granted' | 'denied' | 'default'>;
  sendNotification(options: string | { title: string; body?: string }): void;
}

function tauriNotification(): TauriNotificationApi | undefined {
  const w = window as unknown as { __TAURI__?: { notification?: TauriNotificationApi } };
  return w.__TAURI__?.notification;
}

async function tauriNotify(title: string, body: string): Promise<void> {
  const api = tauriNotification();
  if (!api) return;
  let granted = await api.isPermissionGranted();
  if (!granted) granted = (await api.requestPermission()) === 'granted';
  if (granted) api.sendNotification({ title, body });
}
```

Use it as `async notify(title, body) { await tauriNotify(title, body); }`.

## Secure store

Keep the existing in-memory `Map` implementation for now.

TODO(shell): choose an OS keychain plugin (for example a keyring based Tauri plugin) and back
`secureStore` with it. Until then tokens live in memory only and are lost on restart, which is
the safe default. Do not use `localStorage`, and do not add the Stronghold plugin without an
explicit decision (it needs a password and a larger capability surface).

## Optional: in-app navigation from the tray

The tray menu navigates by dispatching a cancelable `yari:navigate` event with
`detail.path` (`/`, `/approvals` or `/m/agent`) and, if nobody calls `preventDefault()`, falls
back to a full page load. A full reload drops the in-memory token store, so wire the event once
in the root layout:

```ts
import { goto } from '$app/navigation';

window.addEventListener('yari:navigate', (e) => {
  const path = (e as CustomEvent<{ path: string }>).detail.path;
  e.preventDefault();
  void goto(path);
});
```
