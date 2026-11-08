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

const tauriImpl: PlatformImpl = {
  async notify(title, body) {
    // TODO(shell): implement with the official notification plugin, loaded lazily:
    //   const { sendNotification } = await import('@tauri-apps/plugin-notification');
    //   sendNotification({ title, body });
    void title;
    void body;
  },
  secureStore: {
    // TODO(shell): back this with a secure storage plugin (OS keychain), loaded lazily:
    //   const { Stronghold } = await import('@tauri-apps/plugin-stronghold');
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

// Capacitor mobile shell

const capacitorImpl: PlatformImpl = {
  async notify(title, body) {
    // TODO(shell): implement with the official local notifications plugin, loaded lazily:
    //   const { LocalNotifications } = await import('@capacitor/local-notifications');
    //   await LocalNotifications.schedule({ notifications: [{ id: Date.now(), title, body }] });
    void title;
    void body;
  },
  secureStore: {
    // TODO(shell): back this with a secure storage plugin (Keychain and Keystore), loaded lazily:
    //   const { SecureStoragePlugin } = await import('capacitor-secure-storage-plugin');
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
