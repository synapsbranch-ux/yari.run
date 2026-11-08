/**
 * Platform seam. The same build runs in a browser, in the Tauri 2 desktop shell and in the
 * Capacitor mobile shell. Everything that differs lives here.
 *
 * Rules:
 * - No Node-only APIs.
 * - Tokens are never written to localStorage, sessionStorage or IndexedDB. The secure store is
 *   in memory on the web and a native secure store in the shells.
 */

export type Platform = 'web' | 'tauri' | 'capacitor';

interface ShellWindow {
  __TAURI_INTERNALS__?: unknown;
  __TAURI__?: unknown;
  Capacitor?: { isNativePlatform?: () => boolean };
}

export function detectPlatform(): Platform {
  if (typeof window === 'undefined') return 'web';
  const w = window as unknown as ShellWindow;
  if (w.__TAURI_INTERNALS__ !== undefined || w.__TAURI__ !== undefined) return 'tauri';
  if (w.Capacitor?.isNativePlatform?.()) return 'capacitor';
  return 'web';
}

export const platform: Platform = detectPlatform();

export interface SecureStore {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
}

interface PlatformImpl {
  notify(title: string, body: string): Promise<void>;
  secureStore: SecureStore;
}

// Web

const memory = new Map<string, string>();

const webImpl: PlatformImpl = {
  async notify(title, body) {
    if (typeof Notification === 'undefined') return;
    if (Notification.permission !== 'granted') return;
    new Notification(title, { body });
  },
  secureStore: {
    async get(key) {
      return memory.get(key) ?? null;
    },
    async set(key, value) {
      memory.set(key, value);
    },
    async delete(key) {
      memory.delete(key);
    }
  }
};

// Tauri 2 desktop shell
//
// Uses only the globals Tauri injects (app.withGlobalTauri is true), so the client has no
// @tauri-apps npm dependency.

interface TauriNotificationApi {
  isPermissionGranted(): Promise<boolean>;
  requestPermission(): Promise<'granted' | 'denied' | 'default'>;
  sendNotification(options: string | { title: string; body?: string }): void;
}

async function tauriNotify(title: string, body: string): Promise<void> {
  const w = window as unknown as { __TAURI__?: { notification?: TauriNotificationApi } };
  const api = w.__TAURI__?.notification;
  if (!api) return;
  let granted = await api.isPermissionGranted();
  if (!granted) granted = (await api.requestPermission()) === 'granted';
  if (granted) api.sendNotification({ title, body });
}

const tauriImpl: PlatformImpl = {
  async notify(title, body) {
    await tauriNotify(title, body);
  },
  // TODO(shell): back this with an OS keychain plugin. Until then tokens live in memory only and
  // are lost on restart, which is the safe default. Never localStorage.
  secureStore: webImpl.secureStore
};

// Capacitor mobile shell
//
// Uses window.Capacitor.Plugins, which the native shell injects for every installed plugin, so the
// client has no @capacitor npm dependency. Plugins: @capacitor/local-notifications and
// @aparajita/capacitor-secure-storage (Keychain on iOS, Keystore-backed AES-GCM on Android).

interface LocalNotificationsApi {
  checkPermissions(): Promise<{ display: string }>;
  requestPermissions(): Promise<{ display: string }>;
  schedule(options: {
    notifications: { id: number; title: string; body: string }[];
  }): Promise<unknown>;
}

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

const SECURE_PREFIX = 'yari_';

function secureNative(): SecureStorageNative {
  const api = capPlugin<SecureStorageNative>('SecureStorage');
  if (!api) throw new Error('SecureStorage plugin is not available');
  return api;
}

// Errors propagate on purpose: a failed token write must not look like success.
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
      access: 0 // KeychainAccess.whenUnlocked on iOS, ignored on Android
    });
  },
  async delete(key) {
    await secureNative().internalRemoveItem({ prefixedKey: SECURE_PREFIX + key, sync: false });
  }
};

const capacitorImpl: PlatformImpl = {
  async notify(title, body) {
    await capNotify(title, body);
  },
  secureStore: capSecureStore
};

const impls: Record<Platform, PlatformImpl> = {
  web: webImpl,
  tauri: tauriImpl,
  capacitor: capacitorImpl
};

const impl = impls[platform];

/** Shows a system notification when the platform allows it. Never throws. */
export async function notify(title: string, body: string): Promise<void> {
  try {
    await impl.notify(title, body);
  } catch {
    // Notifications are best effort.
  }
}

/** Asks for notification permission on the web. A no-op elsewhere. */
export async function requestNotificationPermission(): Promise<boolean> {
  if (platform !== 'web' || typeof Notification === 'undefined') return false;
  if (Notification.permission === 'granted') return true;
  if (Notification.permission === 'denied') return false;
  return (await Notification.requestPermission()) === 'granted';
}

export const secureStore: SecureStore = impl.secureStore;
