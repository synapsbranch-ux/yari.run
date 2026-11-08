import { DEFAULT_API_URL, IS_DEV_BUILD } from './env';
import { secureStore } from './platform';

const TOKEN_KEY = 'access_token';

/**
 * Connection settings and credentials for the real API path.
 *
 * Everything here lives in memory. The access token goes through `secureStore`, which is in
 * memory on the web. Nothing is written to localStorage, sessionStorage or IndexedDB.
 */
class Session {
  apiUrl = $state(DEFAULT_API_URL);
  /** Developer-only identity sent as X-Yari-Dev-Subject. Honoured in dev builds only. */
  devSubject = $state('');
  token = $state<string | null>(null);

  readonly devControls = IS_DEV_BUILD;

  async hydrate() {
    this.token = await secureStore.get(TOKEN_KEY);
  }

  async setToken(token: string) {
    this.token = token;
    await secureStore.set(TOKEN_KEY, token);
  }

  async signOut() {
    this.token = null;
    this.devSubject = '';
    await secureStore.delete(TOKEN_KEY);
  }

  headers = (): Record<string, string> => {
    const out: Record<string, string> = {};
    if (this.token) out['Authorization'] = `Bearer ${this.token}`;
    if (IS_DEV_BUILD && this.devSubject.trim()) {
      out['X-Yari-Dev-Subject'] = this.devSubject.trim();
    }
    return out;
  };
}

export const session = new Session();
