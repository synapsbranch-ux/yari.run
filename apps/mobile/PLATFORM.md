# Mobile platform code for `apps/client/src/lib/platform.ts`

Ready to paste into `capacitorImpl`. It uses only `window.Capacitor.Plugins`, which the native
shell injects at startup for every installed plugin (`JSExport.getPluginJS` on Android), so the
client needs no `@capacitor/*` npm dependency. Plugin ids, verified against the plugin sources:

- `@capacitor/local-notifications` 8.3.1: `Capacitor.Plugins.LocalNotifications`
  (`checkPermissions`, `requestPermissions`, `schedule`).
- `@aparajita/capacitor-secure-storage` 8.0.1: `Capacitor.Plugins.SecureStorage`.

The secure storage plugin's friendly `get`/`set`/`remove` live in its JS wrapper, which is not
bundled here. Only the native methods exist on the global, so the snippet calls those directly:
`internalGetItem`, `internalSetItem` and `internalRemoveItem`, taking a `prefixedKey` (the wrapper
normally adds the prefix, so we add our own). On Android values are AES-GCM encrypted with a key in
the Android Keystore; on iOS they go to the Keychain (`access: 0` is `whenUnlocked`, `sync: false`
keeps items out of iCloud). `android:allowBackup` is `false` so ciphertext is never restored
without its key.

## Notifications

```ts
interface LocalNotificationsApi {
  checkPermissions(): Promise<{ display: string }>;
  requestPermissions(): Promise<{ display: string }>;
  schedule(options: {
    notifications: { id: number; title: string; body: string }[];
  }): Promise<unknown>;
}

function capPlugin<T>(name: string): T | undefined {
  const w = window as unknown as { Capacitor?: { Plugins?: Record<string, unknown> } };
  return w.Capacitor?.Plugins?.[name] as T | undefined;
}

async function capNotify(title: string, body: string): Promise<void> {
  const api = capPlugin<LocalNotificationsApi>('LocalNotifications');
  if (!api) return;
  let { display } = await api.checkPermissions();
  if (display === 'prompt' || display === 'prompt-with-rationale') {
    ({ display } = await api.requestPermissions());
  }
  if (display !== 'granted') return;
  // Ids are 32-bit ints on Android, so Date.now() cannot be used directly.
  const id = Date.now() % 2_147_483_647;
  await api.schedule({ notifications: [{ id, title, body }] });
}
```

Use it as `async notify(title, body) { await capNotify(title, body); }`.

## Secure store

```ts
interface SecureStorageNative {
  internalGetItem(o: { prefixedKey: string; sync: boolean }): Promise<{ data: string | null }>;
  internalSetItem(o: {
    prefixedKey: string;
    data: string;
    sync: boolean;
    access: number;
  }): Promise<void>;
  internalRemoveItem(o: { prefixedKey: string; sync: boolean }): Promise<{ success: boolean }>;
}

const SECURE_PREFIX = 'yari_';

function secureNative(): SecureStorageNative {
  const api = capPlugin<SecureStorageNative>('SecureStorage');
  if (!api) throw new Error('SecureStorage plugin is not available');
  return api;
}

const capSecureStore: SecureStore = {
  async get(key) {
    const { data } = await secureNative().internalGetItem({
      prefixedKey: SECURE_PREFIX + key,
      sync: false
    });
    return data;
  },
  async set(key, value) {
    await secureNative().internalSetItem({
      prefixedKey: SECURE_PREFIX + key,
      data: value,
      sync: false,
      access: 0 // KeychainAccess.whenUnlocked (iOS), ignored on Android
    });
  },
  async delete(key) {
    await secureNative().internalRemoveItem({ prefixedKey: SECURE_PREFIX + key, sync: false });
  }
};
```

Use it as `secureStore: capSecureStore`. `capPlugin` is defined in the notifications snippet.
Errors reject with the plugin's `StorageError`; let them propagate so a failed token write is not
silently treated as success.
