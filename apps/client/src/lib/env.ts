/**
 * Build-time configuration. Vite reads PUBLIC_* variables from the repository root .env
 * (see vite.config.ts). Every value has a safe default so the app runs with no .env at all.
 */

const FALSE_VALUES = new Set(['false', '0', 'no', 'off']);

/** Fake data is on unless PUBLIC_USE_FAKE_DATA is explicitly set to a false value. */
export const USE_FAKE_DATA: boolean = (() => {
  const raw = import.meta.env.PUBLIC_USE_FAKE_DATA;
  if (raw === undefined || raw === '') return true;
  return !FALSE_VALUES.has(raw.trim().toLowerCase());
})();

export const DEFAULT_API_URL: string = import.meta.env.PUBLIC_API_URL || 'http://localhost:8080';

/** True in `vite dev` and false in a production build. Gates developer-only controls. */
export const IS_DEV_BUILD: boolean = import.meta.env.DEV;

/** The demo canvas that `/` redirects to. */
export const DEMO_CANVAS_PATH = '/o/acme/w/platform/p/storefront/e/production';
