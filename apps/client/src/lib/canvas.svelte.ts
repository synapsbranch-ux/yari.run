import { getContext, setContext } from 'svelte';
import { SvelteSet } from 'svelte/reactivity';
import { estimateMonthlyCents, kindLabel } from '#lib/kinds.js';
import type { PlanChange, ProviderInfo, Service } from '#lib/data/types';

const KEY = Symbol('yari.canvas');

export type DockPanel = 'service' | 'agent' | null;

/** A request to show a plan in the review drawer. */
export type PlanView = { mode: 'deploy' } | { mode: 'review'; planId: string };

/** UI state of one open canvas. Provided by the canvas page and read by its children. */
export class CanvasState {
  environmentId = $state('');
  environmentProtected = $state(false);

  selectedId = $state<string | null>(null);
  dock = $state<DockPanel>(null);
  paletteOpen = $state(false);
  planView = $state<PlanView | null>(null);

  /** Draft nodes the user added and has not deployed yet. */
  drafts = $state<Service[]>([]);

  select(id: string | null) {
    this.selectedId = id;
    if (id) this.dock = 'service';
    else if (this.dock === 'service') this.dock = null;
  }

  openAgent() {
    this.selectedId = null;
    this.dock = this.dock === 'agent' ? null : 'agent';
  }

  closeDock() {
    this.dock = null;
    this.selectedId = null;
  }

  openDeploy() {
    this.planView = { mode: 'deploy' };
  }

  reviewPlan(planId: string) {
    this.planView = { mode: 'review', planId };
  }

  /**
   * Adds a draft node. `provider` is a catalog entry, or null when yari.run should choose.
   * Names stay unique within the environment.
   */
  addDraft(
    input: { nodeKind: string; label: string },
    provider: ProviderInfo | null,
    existing: Service[]
  ): Service {
    const taken = new SvelteSet([...existing, ...this.drafts].map((s) => s.name));
    const base =
      input.nodeKind === 'web' && input.label === 'Empty Service' ? 'service' : input.nodeKind;
    let name = base;
    for (let i = 2; taken.has(name); i++) name = `${base}-${i}`;
    const draft: Service = {
      id: `draft_${name}_${this.drafts.length + 1}`,
      environmentId: this.environmentId,
      name,
      kind: input.nodeKind,
      provider: provider?.id ?? '',
      status: 'draft',
      monthlyCostCents: estimateMonthlyCents(input.nodeKind)
    };
    this.drafts = [...this.drafts, draft];
    this.select(draft.id);
    return draft;
  }

  removeDraft(id: string) {
    this.drafts = this.drafts.filter((d) => d.id !== id);
    if (this.selectedId === id) this.select(null);
  }

  /** Plan changes that deploying the drafts would make. */
  draftChanges(): PlanChange[] {
    return this.drafts.map((d) => ({
      service: d.name,
      kind: d.kind,
      provider: d.provider || 'auto',
      action: 'create',
      risk: 'low',
      costDeltaCents: d.monthlyCostCents ?? 0,
      description: `Create ${kindLabel(d.kind).toLowerCase()} "${d.name}"${
        d.provider ? '' : '. yari.run picks the provider with the best fit'
      }.`
    }));
  }
}

export function setCanvasState(state: CanvasState) {
  setContext(KEY, state);
}

export function getCanvasState(): CanvasState {
  return getContext<CanvasState>(KEY);
}
