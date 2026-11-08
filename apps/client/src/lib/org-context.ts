import { getContext, setContext } from 'svelte';
import type { Organization } from '#lib/data/types';

const KEY = Symbol('yari.org');

export interface OrgContext {
  /** The Organization of the URL, once the list has loaded. */
  readonly org: Organization | undefined;
  readonly slug: string;
}

export function setOrgContext(ctx: OrgContext) {
  setContext(KEY, ctx);
}

export function getOrgContext(): OrgContext {
  return getContext<OrgContext>(KEY);
}
