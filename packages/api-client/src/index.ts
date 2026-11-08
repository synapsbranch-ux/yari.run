import { createClient as createConnectClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { AccessService } from "./gen/yari/v1/access_pb.js";
import { AgentService } from "./gen/yari/v1/agent_pb.js";
import { AuditService } from "./gen/yari/v1/audit_pb.js";
import { CatalogService } from "./gen/yari/v1/catalog_pb.js";
import { HierarchyService } from "./gen/yari/v1/hierarchy_pb.js";
import { IdentityService } from "./gen/yari/v1/identity_pb.js";
import { PlanService } from "./gen/yari/v1/plan_pb.js";

export * from "./gen/yari/v1/access_pb.js";
export * from "./gen/yari/v1/agent_pb.js";
export * from "./gen/yari/v1/audit_pb.js";
export * from "./gen/yari/v1/catalog_pb.js";
export * from "./gen/yari/v1/common_pb.js";
export * from "./gen/yari/v1/hierarchy_pb.js";
export * from "./gen/yari/v1/identity_pb.js";
export * from "./gen/yari/v1/plan_pb.js";

/** Returns the headers to send with every request, for example Authorization. */
export type AuthHeaders = () => Record<string, string> | Promise<Record<string, string>>;

/** One typed client per Connect service of the yari.run API. */
export interface YariClient {
  identity: ReturnType<typeof createConnectClient<typeof IdentityService>>;
  hierarchy: ReturnType<typeof createConnectClient<typeof HierarchyService>>;
  access: ReturnType<typeof createConnectClient<typeof AccessService>>;
  plan: ReturnType<typeof createConnectClient<typeof PlanService>>;
  audit: ReturnType<typeof createConnectClient<typeof AuditService>>;
  catalog: ReturnType<typeof createConnectClient<typeof CatalogService>>;
  agent: ReturnType<typeof createConnectClient<typeof AgentService>>;
}

/**
 * Creates the API client on a fetch transport. Every client of yari.run
 * (web, desktop, mobile) goes through this one function.
 *
 * Tokens must come from getAuthHeaders at call time. Do not persist them in localStorage.
 */
export function createClient(baseUrl: string, getAuthHeaders: AuthHeaders = () => ({})): YariClient {
  const auth: Interceptor = (next) => async (req) => {
    const headers = await getAuthHeaders();
    for (const [name, value] of Object.entries(headers)) {
      req.header.set(name, value);
    }
    return next(req);
  };
  const transport = createConnectTransport({ baseUrl, interceptors: [auth] });
  return {
    identity: createConnectClient(IdentityService, transport),
    hierarchy: createConnectClient(HierarchyService, transport),
    access: createConnectClient(AccessService, transport),
    plan: createConnectClient(PlanService, transport),
    audit: createConnectClient(AuditService, transport),
    catalog: createConnectClient(CatalogService, transport),
    agent: createConnectClient(AgentService, transport),
  };
}

export { AccessService, AgentService, AuditService, CatalogService, HierarchyService, IdentityService, PlanService };
