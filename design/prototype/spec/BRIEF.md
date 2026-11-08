# Design prompt: yari.run clickable prototype (the client's brief, verbatim)

Design a high-fidelity, clickable prototype of **yari.run**, a web app that lets teams deploy and operate software on the clouds they already pay for, from one calm visual canvas, with an AI agent that works for them. Use realistic fake data everywhere. Deliver desktop frames at 1440 px wide and mobile frames at 390 px wide, plus a small component library.

## 1. The product in one paragraph

yari.run is a control plane, not a host. Users connect their own provider accounts (Railway, Vercel, Supabase, Backblaze B2, Resend and many more), draw their architecture on a canvas, and press Deploy. Every change becomes a **Plan** (a reviewable diff with cost and risk) before anything is applied, and production changes need an approval. Each workspace has an **AI agent** that can operate infrastructure (Ops mode) or write and ship code for the user (Code mode), always inside its own isolated sandbox and always through the same Plan and approval flow as humans. Governance (organizations, teams, roles, per-environment permissions, audit log) is a first-class part of the product, not hidden in settings.

Tagline: **"Ship software peacefully, on any cloud."**

## 2. Personality and visual direction

- **Calm, precise, quiet confidence.** The opposite of a noisy cloud console. Think of a well-organized workshop at night: dark, soft light, everything in its place.
- Inspiration for the feel: Railway's canvas (dark, dotted grid, sober cards, command palette, side panels that open without leaving the canvas). Inspiration for the structure: an Organization-first app where Projects, Connections, Teams and Roles are main navigation.
- Name: always lowercase **yari.run**. "yari" is a Japanese spear: use a simple, slim mark. Keep it minimal.

Color (dark default): bg #0B0C10, surface #121319, surface-2 #181A22, border #24262F, text #E8E9EE, text-muted #8C8F9C, accent #7C8CFF (one accent only: primary buttons, focus, selection, links), online #3DD68C, deploying #E9B949 (also pending approval), failed #F26D6D (also destructive), sleeping #6B6F7D.
Also provide a light theme variant of the three hero screens (canvas, Plan review, Code mode).

Type: Inter for UI, JetBrains Mono for code, logs, variable names and IDs. Base size 13 to 14 px, generous line height, headings small and medium weight, never shouty.
Shape and space: 4/8 px grid, 8 to 10 px radius, 1 px borders, almost no shadows (only a soft one on floating elements), lots of empty space. Canvas uses a faint dotted grid.
Icons: thin line icons (Lucide style). Service kinds have their own icon (web, worker, cron, database, bucket, email, queue, domain).
Provider badges: small neutral text chips ("Railway", "Vercel", "Supabase", "Backblaze B2", "Resend"). No third-party logos.

## 2b. Visual references (summary of the attached screenshots)

Railway (feel): dotted dark canvas, sober service cards (icon, name, domain, status dot), a volume shown as a strip attached under its database card, thin icon rail on the left, zoom and undo controls bottom-left, "Add" top-right, breadcrumb with dropdowns. Dashed edges, free placement, a "Sleeping" status with its own icon (moon). Command palette "What would you like to create?" centered over a dimmed canvas, search on top, short list with chevrons for sub-steps. Side panel sliding over the canvas from the right without leaving it, top tabs, secondary sub-tabs, a calm checklist ("Deployment online", "Required variables"). Projects list: cards with a mini row of service icons and "production · 4/4 services online", search with ⌘K hint, grid/list toggle, sidebar with workspace switcher on top.
yari.run adds: Organization and Workspace levels in the breadcrumb, a provider chip on every card, a lock on protected environments, each edge labeled with the injected variable name, a second palette step "Where?" with providers and "Let yari.run choose", panel tabs Deployments, Variables, Logs, Metrics, Access, Settings; sidebar with Connections, Members & Teams, Roles, Audit log as first-class items.

Devopness (structure only, not the look): Organization as the foundation (a personal organization exists from the first second); a one-paragraph explanation above short create forms; a subtle success toast after creation; governance objects visible on the org home page with counts and quick add actions (neutral style, no multicolor tiles); every empty state explains what the object is for and has one clear action (small line icon, no big illustrations); roles described in one human sentence each; short forms with one sentence explaining the concept; Credentials and Teams visible at environment level (= per-environment permissions); NEVER show grids of infrastructure primitives (subnets, SSH keys, cron grids, network rules); destructive actions tucked behind a settings menu; typed-name confirmation for destructive actions with a clear explanation of consequences, as a calm bordered warning with the failed color only on the button; steppers that end with a Review step (onboarding, connecting a provider). yari.run never asks users to build VPCs.

## 3. Global layout

- Top bar: yari.run mark, then the breadcrumb Organization / Workspace / Project / Environment, each segment a dropdown. Protected environments (production) show a small lock. On the right: an Approvals bell with a count, an Agent button, a primary Deploy button, and the user avatar.
- Left icon rail inside a Project: Canvas, Code, Agent sessions, Logs, Metrics, Activity, Settings, with tooltips.
- Organization level sidebar: Projects, Connections, Members & Teams, Roles, Audit log, Usage, Settings.
- Keyboard shortcuts visible in tooltips and menus (⌘K, ⌘Enter, Esc).

## 4. Fake data

- Organization: Kestrel Studio (an agency). Workspaces: Mango Market (client), Harbor Clinic (client), Internal.
- Project in Mango Market: mango-shop, environments production (protected), staging, pr-42 (preview).
- Services in mango-shop: web (Vercel, static/web, mango.shop), api (Railway, web, api.mango.shop), worker (Railway, worker), postgres (Supabase, database), media (Backblaze B2, bucket), email (Resend, email).
- Edges: web to api (API_URL), api to postgres (DATABASE_URL), worker to postgres (DATABASE_URL), api to media (BUCKET_ENDPOINT), api to email (RESEND_API_KEY, masked).
- Statuses: most Online, worker Sleeping, one failed deployment on api in staging.
- Members: Sami Joseph (owner), Lina Pierre (admin), Marc Etienne (developer), Ana Ruiz (viewer, client contact), plus the Agent as a member with a simple "spark" avatar (no robot) and a "developer" role.
- Agent models in the picker: claude-sonnet-5-5, gpt-5.5, local-llama (self-hosted).

## 5. Screens

5.1 Onboarding (wizard, 4 steps, ends with Review): 1 "Welcome to yari.run" (a personal organization is already created; "Create a team organization" as a secondary option). 2 Connect your first provider: grid grouped by category (PaaS, Clouds, Databases, Storage, Email...). Available ones selectable; others show a "Planned" chip. Selecting one opens the connect step: auth method (OAuth or scoped token), what scopes will be requested, and a calm warning when a provider only offers account-wide tokens. 3 Create a project: from a GitHub repo, a template, or "Describe what you want to build" (Code mode). 4 Review: summary of what will be created, then "Create project".

5.2 Organization home: projects grid; each card has name, short description, a mini row of service-kind icons, provider chips, footer like "production · 6/6 services online". Search, sort, grid/list toggle, "New project". (Also governance counts: Connections, Members, Teams, Roles with quick add.)

5.3 Connections (Provider Catalog): list grouped by category with search and filters. Each row: provider name, kinds it supports, auth method, status chip (Connected, Available, Planned), which workspaces use it. A connected provider shows its scope and "Last validated 2 h ago". Planned ones have a disabled "Connect" and "Notify me".

5.4 Members & Teams, Roles: Members table (name, kind Human/API key/CI/Agent, role, scope Organization/Workspace/Project/Environment, last active). Environment permissions matrix for mango-shop: rows Owner, Admin, Developer, Viewer, Agent; columns Deploy, Read secrets, Approve, Run agent; per-environment tabs (production, staging, pr-42). Production clearly stricter (developers and the agent need approval, cannot read secrets). Roles page: four built-in roles with plain-language descriptions.

5.5 Canvas (hero): dotted canvas, six services, dashed edges labeled with injected variable names. Node card: kind icon, name, domain, status dot with label, provider chip in the corner; storage attachment as a small strip under its card. api on staging shows a subtle drift badge "Changed outside yari.run"; hover explains and offers "Review drift". Bottom-left: zoom, fit, undo/redo. Top-right: "Add" (opens palette). Environment switch keeps the same graph but changes statuses and variables.

5.6 Command palette (⌘K): step 1 "What would you like to create?": GitHub Repository, Database, Bucket, Email, Queue, Search, Template, Docker Image, Function, Volume, Empty Service. Step 2 "Where?": providers that support that kind, connected first, planned ones "Coming soon", plus "Let yari.run choose" with a one-line reason ("Supabase: fits the free tier, same region as api").

5.7 Service side panel: from the right over the canvas. Header: icon, name, provider chip, status, environment. Tabs: Deployments (commit message, author human or agent, time, status, Rollback), Variables (injected values show "injected from postgres" with link icon and masked value; "Reveal" requires permission and re-authentication), Logs (live monospace stream, level filters, search), Metrics (requests, latency, memory), Access (who can deploy, read secrets, approve for this service in this environment), Settings (Danger zone; typing the service name).

5.8 Plan review drawer (hero): opened by Deploy. Summary "4 changes across 3 providers". Changes grouped by provider, action chip (Create, Update, Delete, No change) and one-line description. Estimated monthly cost delta ("+$7.00 / month") and risk chip (Low, Medium, High). On production: "Requires approval from an admin" with approvers listed. Buttons: Cancel, and Apply or Request approval. States: applying (progress per provider in dependency order "1. postgres, 2. api and worker, 3. web"), applied (calm success), failed (what failed, "Ask the agent to fix").

5.9 Approvals: inbox of pending plans: environment with lock, requester (human or Agent badge), changes count, cost, risk, time. Detail mirrors Plan review, Approve and Reject behind a confirmation. Empty: "Nothing waiting. Everything is calm."

5.10 Agent panel (Ops mode): right drawer from the Agent button. Header: model picker, budget meter "$0.84 of $2.00", sandbox status "Isolated sandbox · 2 vCPU · 2 GB · expires in 42 min · network: model gateway and yari only". Goal input with suggestions ("Why is api failing on staging?", "Add a staging copy of postgres"). Timeline: agent messages, actions with a location chip (Sandbox or yari), collapsible observations (logs, command output), guardrail refusals ("Blocked: agents cannot delete databases"), Plan cards that open the Plan review drawer. Follow-up box and Stop. Final state "Waiting for approval on production" with a link to Approvals.

5.11 Code mode (hero): Project → Code tab. Start state: large calm prompt "Describe what you want to build", example chips ("A shop with Stripe checkout", "An API with Postgres and email"), repo selector. Working state, three panes: left file tree + Changes section; center read-only viewer with syntax highlighting, switching to diff view for changed files; right agent chat/timeline with model, budget, sandbox status; bottom drawer Terminal output (read-only mono); a toggle opens a Preview pane: browser frame at mango-shop-7f3a.preview.yari.run with a "Private" lock chip. Tab top bar: repo and branch (yari/session-7f3a), session status, actions Open PR, Deploy to preview (pr-42), Promote to production (opens Plan review with approval). A moment where the agent proposes a Plan card "Add postgres on Supabase and connect it to api", which, once accepted, makes a new node appear on the canvas.

5.12 Audit log: filterable table: time, actor (human, API key, CI, Agent, with icons), action, target, environment, linked Plan or Agent Session. Agent rows link to the session timeline. Export button.

5.13 Usage: agent credit spend per workspace and per model (simple bar chart), recent Agent Sessions with mode (Ops or Code), duration, cost, outcome.

5.14 Mobile (390 px): bottom tabs Projects, Approvals, Agent, Activity, Settings. Projects: list with status summaries; tapping a project shows environments and a plain list of services (no canvas). Approvals: inbox and a detail sheet with Approve / Reject (confirmation sheet). Agent: sessions list and a live timeline with Stop and a reply box. A lock-screen style notification mock: "Agent is waiting for approval on mango-shop production".

5.15 Desktop tray (small mock): menu bar dropdown "Open yari.run", "Pending approvals: 2", "Agent sessions running: 1", "Quit".

## 6. Flows to make clickable
1. Deploy to production with approval: Canvas → Deploy → Plan review (production, Request approval) → mobile notification → mobile approval → back on desktop, applying → applied, node statuses update.
2. Agent fixes a failed build: failed api node on staging → "Ask the agent" → Agent panel timeline (reads logs in yari, edits build config in Sandbox, reruns) → Plan card → apply on staging (no approval) → Online.
3. Build with Code mode: start → describe → agent works (files appear, terminal output, preview opens) → agent proposes adding postgres (Plan) → Deploy to preview → Promote to production → Plan review with approval.
4. Onboarding: sign up → connect Railway → create project from a repo → Review → canvas.

## 7. States
Empty states (no projects, no connections, nothing to approve), loading skeletons, an error state with a clear next step, drift, agent budget exhausted ("This session reached its $2.00 budget. Raise it or start a new session."), and a "provider planned, not supported yet" state with a docs link.

## 8. Microcopy tone
Short, calm and specific. Say what will happen and what it costs. Never alarming, never cute. Examples: "4 changes across 3 providers. About +$7.00 per month." / "Production is protected. An admin needs to approve this." / "The agent works in its own sandbox. It cannot see your provider keys." / "Nothing waiting. Everything is calm."

## 9. Accessibility
WCAG AA contrast on dark and light, visible focus rings in the accent color, status never shown by color alone (always a label or icon), keyboard reachable palette, drawers and tabs.

## 10. Do not
No grids of raw infrastructure primitives. No bright multicolor styling, gradients everywhere or heavy shadows. No real company logos (neutral text chips). Do not hide teams, roles or approvals in settings. No marketing copy inside the app.

## 11. Deliverables
Desktop frames for every screen in 5 and every state in 7; mobile frames for 5.14; clickable flows of 6; a component library (Button primary/secondary/ghost/destructive, Chip status/provider/action/location, Status dot, Node card, Edge label, Breadcrumb with dropdowns, Tabs, Drawer, Side panel, Command palette, Timeline item message/action/observation/guardrail/plan card, Plan change row, Budget meter, Table, Toast, Empty state, Confirmation dialog with typed name); color and type tokens dark and light.
