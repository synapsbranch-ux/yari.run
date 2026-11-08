const NOW = Date.now();

/** ISO timestamp for a moment `minutes` before the module was loaded. */
export function ago(minutes: number): string {
  return new Date(NOW - minutes * 60_000).toISOString();
}
