import type { Edge, Service } from '#lib/data/types';

export const COLUMN_WIDTH = 380;
export const ROW_HEIGHT = 160;

/**
 * Positions for services that have none: consumers on the left, dependencies to the right.
 * A service sits one column right of the furthest consumer that depends on it.
 */
export function autoLayout(
  services: Service[],
  edges: Edge[]
): Map<string, { x: number; y: number }> {
  const depth = new Map<string, number>();
  const ids = new Set(services.map((s) => s.id));
  const visit = (id: string, seen: Set<string>): number => {
    const known = depth.get(id);
    if (known !== undefined) return known;
    if (seen.has(id)) return 0;
    seen.add(id);
    const consumers = edges.filter((e) => e.to === id && ids.has(e.from));
    const d =
      consumers.length === 0 ? 0 : Math.max(...consumers.map((e) => visit(e.from, seen))) + 1;
    depth.set(id, d);
    return d;
  };
  for (const s of services) visit(s.id, new Set());

  const rows = new Map<number, number>();
  const out = new Map<string, { x: number; y: number }>();
  for (const s of services) {
    const col = depth.get(s.id) ?? 0;
    const row = rows.get(col) ?? 0;
    rows.set(col, row + 1);
    out.set(s.id, { x: col * COLUMN_WIDTH, y: row * ROW_HEIGHT });
  }
  return out;
}
