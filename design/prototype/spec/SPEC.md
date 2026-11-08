# yari.run prototype: shared build spec

Everything a builder needs so ~48 artboards look like ONE product. Read fully. The client brief is `BRIEF.md` (same folder). The `.dc.html` format rules are `FORMAT.md` (same folder). The gold reference artboard is `project/Canvas.dc.html` (top bar, rail, graph, node cards). COPY its chrome markup verbatim; do not redesign it.

## 0. Hard format rules (each one fails silently if broken)

- File skeleton exactly like `project/Canvas.dc.html`: `<!doctype html>`, `<html lang="en">`, `<head>` with `<meta charset="utf-8">`, a short `<title>`, and EXACTLY `<script src="./support.js"></script>`. Then `<body><x-dc><helmet>…</helmet> ROOT </x-dc><script type="text/x-dc" data-dc-script data-props='…'> class Component extends DCLogic {…} </script></body></html>`.
- Close every non-void element, quote every attribute. SVG children (`<path/>`, `<circle/>`, `<rect/>`) may self-close.
- `{{hole}}` = dotted lookup ONLY (`{{tab}}`, `{{item.name}}`, `{{$index}}`). NEVER `{{a===b}}`, `{{!x}}`, `{{fn()}}`, ternaries. Compute in `renderVals()` and return a named value.
- Events: `onClick="{{ openPanel }}"` whole-value only, handler returned from `renderVals()`. Per-item handlers: build them in `renderVals()` (`items: xs.map(x => ({...x, pick: () => this.setState({sel: x.id})}))`).
- Conditionals `<sc-if value="{{ showX }}" hint-placeholder-val="{{ true }}">…</sc-if>`; there is NO else: use two sc-ifs with two booleans. Loops `<sc-for list="{{ rows }}" as="r" hint-placeholder-count="4">…</sc-for>`.
- State-dependent styles: precompute the whole style string or piece in renderVals (e.g. `tabs: [{label, style:'…'}]`) and bind `style="{{ t.style }}"`. Keep static theme colors INLINE as literal hex in markup.
- Clickables are real `<button type="button">` or `<a href>`. Never onClick on div/span, never role=button. Icon-only buttons get `aria-label` and `title` (the tooltip, include the shortcut: `title="Zoom in (+)"`).
- Navigation to another artboard: `<a href="PlanReview.dc.html" style="…button look…">Deploy</a>`. Never put a `<button>` inside an `<a>`.
- No emoji. No `innerHTML`, `appendChild`, global keydown handlers, iframes, imports. No network except the Google Fonts link in helmet.
- Inline `style="…"` for everything visual. `<helmet><style>` only for the shared basics block below.
- `data-props` is single-quoted JSON and must include `"$preview":{"width":W,"height":H}` matching the board size. Tweaks: at most 1–3 meaningful levers (e.g. a state enum to preview "applying/applied"), never copy.
- In logic, `setTimeout` in `componentDidMount` is fine for progress animations; clear it in `componentWillUnmount`.
- Run the lint after writing: `python3 -I <scratchpad>/tools/lint.py <root>/project <File.dc.html>`. "link target missing" is OK ONLY for files in the board list (§9) that other builders are still writing.

## 1. Tokens

Dark (default):
| token | hex | use |
|---|---|---|
| bg | #0B0C10 | app background, canvas |
| surface | #121319 | cards, panels, drawers |
| surface-2 | #181A22 | hover, nested surfaces, inputs |
| border | #24262F | 1px borders |
| border-strong | #2F323D | dashed edges, input borders on hover |
| text | #E8E9EE | primary text |
| text-muted | #8C8F9C | secondary text, labels (5.7:1 on surface, AA) |
| accent | #7C8CFF | primary button bg, focus, selection, links |
| accent-tint | rgba(124,140,255,0.12) | selected rows, accent chip bg |
| on-accent | #0B0C10 | text on accent / on status-colored buttons |
| online | #3DD68C | tint rgba(61,214,140,0.12) |
| deploying | #E9B949 | tint rgba(233,185,73,0.12); also pending approval |
| failed | #F26D6D | tint rgba(242,109,109,0.12); destructive |
| sleeping | #6B6F7D | ICON/DOT ONLY (fails AA as text). Pair with a #8C8F9C label |

Light (only the three light hero boards): bg #F6F6F8, surface #FFFFFF, surface-2 #F0F1F4, border #E2E3E8, border-strong #CDCFD6, text #15161B, text-muted #5F6270, accent #4F5FE0 (white text on it), accent-tint rgba(79,95,224,0.10), online #187A4A, deploying #8F6400, failed #C93C3C, sleeping #6B6F7D, dotted grid dots #D9DAE0.

Type: Inter (UI) 13px base, line-height 1.5; JetBrains Mono for code, logs, variable names, IDs, commit SHAs, domains in mono contexts. Sizes: 11 (meta, chips, kbd), 12 (secondary), 13 (body), 14 (card titles, emphasized), 16 (drawer titles), 20 (page titles, weight 600), 28 (rare hero prompt, weight 500). Weights 400/500/600 only. Never uppercase shouting; small section labels may be 11px, weight 500, letter-spacing .02em, text-muted, sentence case.

Space: 4/8 grid. Radius: 8px controls, 10px cards/drawers, 6px chips, 4px kbd. Borders 1px. Shadow ONLY on floating elements (drawers, palette, popovers, toasts): `box-shadow: 0 12px 40px rgba(0,0,0,0.45)` (light theme: `0 12px 40px rgba(20,22,30,0.12)`).

## 2. Helmet block (paste in every board, dark boards)

```html
<helmet>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&amp;family=JetBrains+Mono:wght@400;500&amp;display=swap">
<style>
body{margin:0;background:#0B0C10}
a{color:#7C8CFF;text-decoration:none}a:hover{color:#A9B4FF}
*:focus-visible{outline:2px solid #7C8CFF;outline-offset:2px}
button,input,select,textarea{font-family:inherit;font-size:inherit}
.hv:hover{background:#181A22 !important}
.hvb:hover{border-color:#2F323D !important}
@keyframes yr-pulse{0%,100%{opacity:1}50%{opacity:.35}}
@keyframes yr-spin{to{transform:rotate(360deg)}}
@keyframes yr-shimmer{0%{background-position:-400px 0}100%{background-position:400px 0}}
@keyframes yr-in{from{opacity:0;transform:translateY(4px)}to{opacity:1;transform:none}}
</style>
</helmet>
```
Light boards: same but body background #F6F6F8, `a{color:#4F5FE0}`, focus outline #4F5FE0, `.hv:hover{background:#F0F1F4 !important}`.
Use `class="hv"` for hover backgrounds on rows/buttons, `animation: yr-pulse 1.6s ease-in-out infinite` on deploying dots, `animation: yr-spin 1s linear infinite` on spinners.

## 3. Root shells

Desktop app board (w 1440, expand fill): the ROOT is
`<div style="width: 100%; height: 900px; overflow-x: auto; overflow-y: hidden; background: #0B0C10; color: #E8E9EE; font-family: Inter, system-ui, sans-serif; font-size: 13px; line-height: 1.5">` containing ONE inner `<div style="min-width: 1200px; height: 900px; display: flex; flex-direction: column; position: relative">`. Taller boards: change both heights to the board's h, and use `overflow-y: auto` on the scroll region, not the root.

Mobile board (390×844, NOT expand): root `<div style="width: 390px; height: 844px; position: relative; overflow: hidden; background: #0B0C10; color: #E8E9EE; font-family: Inter, system-ui, sans-serif; font-size: 14px; line-height: 1.5; display: flex; flex-direction: column">`. No fake OS status bar (no clock/battery row) except the lock-screen mock, which shows a large clock as content. Top app bar 52px; bottom tab bar 64px with 5 tabs (icon 20px + 11px label): Projects, Approvals (badge "2"), Agent, Activity, Settings; active tab in #E8E9EE with accent icon, others #8C8F9C. Touch targets ≥ 44px.

### 3a. Project top bar (48px) — copy from Canvas.dc.html
mark + "yari.run" | breadcrumb `Kestrel Studio ▾ / Mango Market ▾ / mango-shop ▾ / [lock] production ▾` (each a `<button class="hv">` with chevron; separators "/" in #3A3D4A) | right side: Approvals bell button with count badge "2" (badge bg #E9B949, text #0B0C10, 10px, 600), "Agent" ghost button with spark icon (`title="Agent (⌘J)"`), primary "Deploy" with kbd "⌘↵", avatar "ME" (Marc Etienne, the signed-in desktop user, 28px circle bg #181A22 border #24262F, 11px 600).
The env segment is a link-styled dropdown; on Canvas boards the env menu links to Canvas.dc.html (production), CanvasStaging.dc.html, CanvasPR42.dc.html.

### 3b. Project left rail (48px wide, border-right) — copy from Canvas.dc.html
Icon buttons 32×32, radius 8, in order: Canvas, Code, Agent sessions, Logs, Metrics, Activity; Settings pinned at the bottom. Active: bg #181A22, icon #E8E9EE; inactive #8C8F9C. Each has aria-label + title tooltip with shortcut (Canvas "Canvas (G C)", Code "Code (G D)", Agent sessions "Agent sessions (G A)", Logs "Logs (G L)", Metrics "Metrics (G M)", Activity "Activity (G V)", Settings "Settings (G S)"). Canvas links to Canvas.dc.html, Code to CodeStart.dc.html, Agent sessions to AgentOps.dc.html.

### 3c. Organization shell (for OrgHome, Connections, Members, Permissions, Roles, AuditLog, Usage, empty/loading/error boards)
Top bar 48px: mark + "yari.run" | breadcrumb `Kestrel Studio ▾ / Mango Market ▾` | right: bell "2", avatar "ME" (no Deploy, no Agent on org pages).
Left sidebar 240px, bg #0B0C10, border-right #24262F, padding 12px:
- Workspace switcher button at top: 28px square monogram "KS" (bg #181A22, border), "Kestrel Studio" 13px 500 + "Organization · 4 members" 11px muted, chevron-up-down. 
- Nav (each `<a class="hv">` 32px tall, radius 8, icon 16px + label 13px; active: bg #181A22, text #E8E9EE; else #8C8F9C): Projects (box) → OrgHome.dc.html, Connections (plug) → Connections.dc.html, Members & Teams (users) → Members.dc.html, Roles (shield) → Roles.dc.html, Audit log (clipboard) → AuditLog.dc.html, Usage (gauge) → Usage.dc.html, Settings (sliders). Approvals (inbox, with count 2) also appears as a nav item right after Projects → Approvals.dc.html (approvals are never hidden).
- Section label "Workspaces" then: Mango Market (active dot), Harbor Clinic, Internal, "+ New workspace" muted.
- Bottom: "Docs" and "Keyboard shortcuts ?" muted small links.
Content area: padding 32px 40px, max-width 1120px. Page header: title 20px 600 + one calm sentence 13px muted, actions right.

## 4. Icons (Lucide paths; 24 viewBox)
Wrapper: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"> … </svg>`
- web/globe: `<circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>`
- worker/cpu: `<rect width="16" height="16" x="4" y="4" rx="2"/><rect width="6" height="6" x="9" y="9" rx="1"/><path d="M15 2v2M15 20v2M2 15h2M2 9h2M20 15h2M20 9h2M9 2v2M9 20v2"/>`
- cron/clock: `<circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>`
- database: `<ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14a9 3 0 0 0 18 0V5"/><path d="M3 12a9 3 0 0 0 18 0"/>`
- bucket/archive: `<rect width="20" height="5" x="2" y="3" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/>`
- email/mail: `<rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>`
- queue/list: `<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>`
- domain/link: `<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>`
- volume/hard-drive: `<path d="M22 12H2"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/><path d="M6 16h.01M10 16h.01"/>`
- spark (agent avatar/button): `<path d="M12 3l1.9 5.8L20 11l-6.1 2.2L12 19l-1.9-5.8L4 11l6.1-2.2z"/>` (Agent avatar = 24–28px circle, bg rgba(124,140,255,0.12), border 1px rgba(124,140,255,0.35), spark in #7C8CFF)
- lock: `<rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>`
- bell: `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>`
- plus `<path d="M5 12h14M12 5v14"/>` · minus `<path d="M5 12h14"/>` · x `<path d="M18 6 6 18M6 6l12 12"/>` · check `<path d="M20 6 9 17l-5-5"/>`
- chevron-down `<path d="m6 9 6 6 6-6"/>` · chevron-right `<path d="m9 18 6-6-6-6"/>` · chevron-left `<path d="m15 18-6-6 6-6"/>` · chevrons-up-down `<path d="m7 15 5 5 5-5M7 9l5-5 5 5"/>`
- search `<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`
- moon (sleeping) `<path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"/>`
- x-circle (failed) `<circle cx="12" cy="12" r="10"/><path d="m15 9-6 6M9 9l6 6"/>` · check-circle `<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>`
- alert-triangle `<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4M12 17h.01"/>` · info `<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>`
- loader `<path d="M21 12a9 9 0 1 1-6.22-8.56"/>` (spin it)
- git-branch `<path d="M6 3v12"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>` · git-commit `<circle cx="12" cy="12" r="3"/><path d="M3 12h6M15 12h6"/>` · git-pull-request `<circle cx="18" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><path d="M13 6h3a2 2 0 0 1 2 2v7M6 9v12"/>`
- canvas `<rect width="8" height="8" x="3" y="3" rx="2"/><path d="M7 11v4a2 2 0 0 0 2 2h4"/><rect width="8" height="8" x="13" y="13" rx="2"/>` · code `<path d="m16 18 6-6-6-6M8 6l-6 6 6 6"/>` · logs `<path d="M4 6h16M4 12h16M4 18h10"/>` · metrics `<path d="M3 3v18h18"/><path d="M18 17V9M13 17V5M8 17v-3"/>` · activity `<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>` · settings/sliders `<path d="M21 4h-7M10 4H3M21 12h-9M8 12H3M21 20h-5M12 20H3M14 2v4M8 10v4M16 18v4"/>`
- users `<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>` · user `<circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/>` · shield `<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/>` · plug `<path d="M12 22v-5M9 8V2M15 8V2M18 8v5a6 6 0 0 1-12 0V8z"/>` · box/project `<path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5M12 22V12"/>` · gauge `<path d="m12 14 4-4"/><path d="M3.34 19a10 10 0 1 1 17.32 0"/>` · clipboard `<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><rect width="8" height="4" x="8" y="2" rx="1"/><path d="M9 12h6M9 16h4"/>` · inbox `<path d="M22 12h-6l-2 3h-4l-2-3H2"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>`
- key `<circle cx="7.5" cy="15.5" r="5.5"/><path d="m21 2-9.6 9.6M15.5 7.5l3 3L22 7l-3-3"/>` · eye `<path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/>` · folder `<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>` · file `<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>` · terminal `<path d="m4 17 6-6-6-6M12 19h8"/>` · stop/square `<rect width="14" height="14" x="5" y="5" rx="2"/>` · external-link `<path d="M15 3h6v6M10 14 21 3M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>`
- undo `<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>` · redo `<path d="m15 14 5-5-5-5"/><path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13"/>` · fit/maximize `<path d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3"/>` · grid `<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/>` · rollback `<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>` · download `<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/>` · refresh `<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>` · zap/function `<path d="M4 14a1 1 0 0 1-.78-1.63l9.9-10.2a.5.5 0 0 1 .86.46l-1.92 6.02A1 1 0 0 0 13 10h7a1 1 0 0 1 .78 1.63l-9.9 10.2a.5.5 0 0 1-.86-.46l1.92-6.02A1 1 0 0 0 11 14z"/>` · template `<rect width="18" height="7" x="3" y="3" rx="1"/><rect width="9" height="7" x="3" y="14" rx="1"/><rect width="5" height="7" x="16" y="14" rx="1"/>` · empty-service `<rect width="18" height="18" x="3" y="3" rx="2" stroke-dasharray="3 3"/>` · more `<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>` · message `<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>` · arrow-up `<path d="m5 12 7-7 7 7M12 19V5"/>` · arrow-left `<path d="m12 19-7-7 7-7M19 12H5"/>` · smartphone `<rect width="14" height="20" x="5" y="2" rx="2"/><path d="M12 18h.01"/>` · monitor `<rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8M12 17v4"/>`

yari.run mark (exact; slim spear, accent): `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M4 20 14.2 9.8" stroke="#7C8CFF" stroke-width="1.75" stroke-linecap="round"/><path d="M13 11 16.6 5.6 21 3 18.4 7.4Z" fill="#7C8CFF"/></svg>` then wordmark `<span style="font-weight: 600; font-size: 14px; letter-spacing: -0.01em">yari<span style="color: #8C8F9C">.run</span></span>`.

## 5. Components (inline styles; copy exactly)

Buttons (height 32, radius 8, padding 0 12px, font 13px 500, inline-flex, align center, gap 6px, cursor pointer, border 1px):
- primary: `background: #7C8CFF; color: #0B0C10; border: 1px solid #7C8CFF`
- secondary: `background: #181A22; color: #E8E9EE; border: 1px solid #24262F` (+class hvb)
- ghost: `background: transparent; color: #8C8F9C; border: 1px solid transparent` (+class hv)
- destructive: `background: #F26D6D; color: #0B0C10; border: 1px solid #F26D6D`
- disabled: opacity .45, cursor not-allowed, `disabled` attribute.
- small: height 26, padding 0 8px, font 12px.
kbd: `<kbd style="font-family: 'JetBrains Mono', monospace; font-size: 11px; padding: 0 4px; border: 1px solid #2F323D; border-radius: 4px; color: #8C8F9C; background: transparent">⌘K</kbd>`; inside a primary button use `border-color: rgba(11,12,16,0.25); color: rgba(11,12,16,0.7)`.

Chips (height 20–22, padding 0 8px, radius 6, font 11px 500, inline-flex gap 5px):
- status: Online = tint bg rgba(61,214,140,0.12), text #3DD68C, 6px dot #3DD68C. Deploying = tint deploying, pulsing dot. Failed = x-circle 12px icon + "Failed" #F26D6D. Sleeping = moon 12px icon #6B6F7D + label #8C8F9C, bg rgba(107,111,125,0.16). Pending approval = deploying tint + clock icon + "Pending approval". ALWAYS icon/dot + text label.
- provider: `border: 1px solid #24262F; color: #8C8F9C; background: transparent; font-weight: 500` text only ("Railway", "Vercel", "Supabase", "Backblaze B2", "Resend").
- action: Create (online tint + plus icon), Update (accent tint #7C8CFF text + refresh-ish icon or "~"), Delete (failed tint + minus), No change (border only, muted).
- location: Sandbox = `background: #181A22; border: 1px solid #2F323D; color: #E8E9EE; font-family mono` + box icon, label "Sandbox"; yari = accent tint, accent text, tiny mark, label "yari".
- risk: Low (online tint), Medium (deploying tint), High (failed tint), always text "Low risk" etc.
- env: `production` with lock 12px; mono 11px.

Status dot alone: 8px circle; deploying pulses; failed = 10px x-circle icon; sleeping = moon icon. Never color only: next to a label or with aria-label.

Node card (canvas): `<a href|button>` absolute, width 240, bg #121319, border 1px #24262F, radius 10, padding 14px; row1: kind icon 16px #8C8F9C in a 28px square (bg #181A22 radius 8) + name 14px 500 + provider chip pushed right; row2: domain 12px muted (omit if none; worker shows "No public domain" muted italic? no — show nothing); row3 (margin-top 14px): status. Selected: border #7C8CFF + 0 0 0 3px rgba(124,140,255,0.18). Attachment strip under a card: a sibling div directly below, same width, `border: 1px solid #24262F; border-top: none; border-radius: 0 0 10px 10px; background: #0F1015; padding: 8px 14px; font-size: 12px; color: #8C8F9C` with hard-drive icon + "postgres-data · 8 GB of 20 GB"; the card above then has radius 10 10 0 0.
Edge label: `position: absolute; transform: translate(-50%,-50%); background: #0B0C10; border: 1px solid #24262F; border-radius: 6px; padding: 1px 6px; font-family: 'JetBrains Mono', monospace; font-size: 11px; color: #8C8F9C`. Masked secret label: `RESEND_API_KEY ••••` with a tiny lock.
Edges: ONE aria-hidden absolutely-positioned `<svg>` behind the nodes, paths `stroke="#3A3D4A" stroke-width="1.25" stroke-dasharray="4 4" fill="none"` with small arrowheads (marker). See Canvas.dc.html.

Tabs: row with border-bottom #24262F, each tab a `<button>` 13px, padding 10px 2px, margin-right 20px; active: color #E8E9EE + 2px bottom border #7C8CFF (use `box-shadow: inset 0 -2px 0 #7C8CFF`); inactive #8C8F9C. Sub-tabs: pill buttons height 26, active bg #181A22.

Drawer / side panel: absolute right 0 top 0 bottom 0 inside the area below the top bar, width 560 (plan review), 640 (service panel), 480 (agent panel); bg #121319; border-left 1px #24262F; soft floating shadow; header padding 20px 24px with title 16px 600 and a close ghost icon button (`aria-label="Close (Esc)"`, title "Close (Esc)"); footer sticky bottom with border-top, padding 16px 24px, buttons right-aligned. Canvas behind stays visible (no dim) for side panels; palette and confirmation dialogs dim with `background: rgba(11,12,16,0.6)` overlay.

Command palette: centered 560px wide, top 120px, bg #121319, border #24262F, radius 12, shadow; search input row 48px with search icon + placeholder + "Esc" kbd; list items 40px with icon in 28px square, label, muted hint right, chevron-right for sub-steps; selected item bg #181A22; footer row 32px muted: "↑↓ to navigate · ↵ to select · Esc to close".

Timeline items (agent): vertical list, gap 14px, each with a 24px left gutter icon:
- message (agent): spark avatar + text 13px #E8E9EE.
- user message: right-aligned bubble bg #181A22 radius 10 padding 10px 12px.
- action: 1px border card, mono 12px command/summary + location chip (Sandbox / yari) + duration "2.1 s" muted + check icon when done / spinner when running.
- observation: collapsible (`<button>` "Show output" chevron) mono 12px block bg #0B0C10 border, max ~8 lines.
- guardrail: bordered card border-color rgba(242,109,109,0.35), shield icon #F26D6D, title "Blocked: agents cannot delete databases" + one muted line why and what it did instead.
- plan card: border #2F323D card: title, "1 change on Railway · No cost change · Low risk", env chip, buttons "Review plan" (primary small) / "Dismiss".

Plan change row: provider group header (provider chip + "2 changes"), rows: action chip, service name 13px 500 (mono? no: Inter), one-line description muted, cost delta right in mono ("+$5.00"), expandable "Details" chevron.
Budget meter: label "$0.84 of $2.00" 12px + 4px bar (track #24262F, fill #7C8CFF; >90% fill #E9B949; 100% #F26D6D) + "42%".
Table: header row 32px 11px 500 muted, rows 44px, border-bottom #24262F, hover hv.
Toast: bottom-center or bottom-right, bg #181A22, border #24262F, radius 10, shadow, padding 10px 14px, check-circle in #3DD68C + text + optional "View" link. Subtle; never bright.
Empty state: centered column max 360px: 40px rounded square bg #181A22 border with a 20px line icon, title 14px 500, one or two calm sentences muted, one primary button, optional "Learn more" link.
Confirmation dialog (typed name): 440px centered card over dim; title; bordered calm warning box (border #24262F, bg #181A22, alert-triangle muted icon) listing consequences in 2–3 bullets; label "Type <mono>api</mono> to confirm"; input; buttons Cancel (secondary) + destructive action (disabled until the typed value matches; implement with state).
Inputs: height 32, bg #0B0C10 (or #181A22 inside surfaces), border 1px #24262F, radius 8, padding 0 10px, color #E8E9EE, placeholder #6B6F7D... (placeholder must still be readable: use #8C8F9C). Always a `<label>`.
Avatars: 24–28px circle, initials 11px 600, bg #181A22 border #24262F. Sami Joseph "SJ", Lina Pierre "LP", Marc Etienne "ME", Ana Ruiz "AR"; Agent = spark avatar (§4); CI = git-commit icon avatar; API key = key icon avatar.

## 6. The story & data (use EXACTLY; consistency across boards matters)

Signed-in desktop user: Marc Etienne (developer, "ME"). Mobile user (approver): Lina Pierre (admin, "LP"). Date: Sunday 8 Nov 2026, afternoon. Times like "15:40", "4 min ago".
Org Kestrel Studio · workspaces Mango Market (client), Harbor Clinic (client), Internal.
Projects: Mango Market → mango-shop ("Storefront, API and order worker for Mango Market", 6 services, production · 6/6 online... note worker is Sleeping: footer "production · 5/6 online · 1 sleeping"), mango-admin ("Back-office for the Mango team", 2 services: web Vercel, api Railway; "production · 2/2 services online"). Harbor Clinic → harbor-portal ("Patient booking portal", 4 services: web Vercel, api Railway, postgres Supabase, email Resend; "production · 4/4 services online"), harbor-reminders ("SMS and email reminders", cron Railway + email Resend, "production · 2/2 online"). Internal → kestrel-site ("Studio website", web Vercel, "1/1 online"), playground ("Experiments", 2 services, staging only, "staging · 1/2 online · 1 failed"? keep calm: "staging · 2/2 sleeping").

mango-shop services (all envs share the graph):
| service | kind | provider | domain prod | domain staging | domain pr-42 |
|---|---|---|---|---|---|
| web | web (static) | Vercel | mango.shop | staging.mango.shop | mango-shop-pr-42.vercel.app |
| api | web | Railway | api.mango.shop | api.staging.mango.shop | api-pr-42.up.railway.app |
| worker | worker | Railway | — | — | — |
| postgres | database | Supabase | — (strip: "postgres-data · 8 GB of 20 GB") | strip "2 GB of 8 GB" | (none in pr-42 until the agent adds it) |
| media | bucket | Backblaze B2 | media.mango.shop | — ("mango-media-staging") | — |
| email | email | Resend | mail.mango.shop (sending domain) | same | same |
Statuses — production: all Online except worker Sleeping. staging: api Failed (latest deploy failed at 15:12, previous version still serving) + drift badge, worker Sleeping, others Online. pr-42: web, api Online, worker Sleeping, media/email Online; postgres added by the agent ("New").
Edges: web→api API_URL; api→postgres DATABASE_URL; worker→postgres DATABASE_URL; api→media BUCKET_ENDPOINT; api→email RESEND_API_KEY (masked "RESEND_API_KEY ••••").
Regions: api/worker Railway eu-west (Amsterdam); postgres Supabase eu-central-1 (Frankfurt); web Vercel global edge; media B2 eu-central-003.

Commits: 8f2c1a4 "Add order export endpoint" (Marc Etienne, 20 min ago) — the one being deployed to production in flow 1. Previous prod deploys: 3b91e07 "Fix VAT rounding on invoices" (Lina Pierre, yesterday), a54c2d8 "Cache product images at the edge" (Agent, 2 days ago), 77de310 "Upgrade to Node 22" (Marc, 4 days ago).

FLOW 1 plan (production): id plan_7Q2M · "4 changes across 3 providers. About +$7.00 per month." · Medium risk (includes a database migration) · approvers Lina Pierre (admin), Sami Joseph (owner) · requested by Marc Etienne.
- Supabase · postgres · Update · "Run migration 0042_order_exports (adds 1 table, no data rewritten)" · +$2.00 (storage +2 GB)
- Railway · api · Update · "Deploy 8f2c1a4 “Add order export endpoint”" · $0.00
- Railway · worker · Update · "Deploy 8f2c1a4 and wake from sleep to process exports" · +$5.00
- Vercel · web · Update · "Deploy 8f2c1a4 with EXPORTS_ENABLED=true" · $0.00
- No change: media (Backblaze B2), email (Resend) — shown collapsed "2 services unchanged".
Apply order: "1. postgres · 2. api and worker · 3. web". Applying durations: postgres 18 s, api 52 s, worker 41 s, web 24 s. Applied: "Deployed to production in 2 min 14 s. 6 of 6 services healthy." After apply worker becomes Online.
Failed variant (PlanFailed): step 2 worker fails: "Health check failed after 90 s. worker exited with code 1: missing variable QUEUE_CONCURRENCY." postgres migration applied (kept), api deployed OK, web skipped (not started). "Nothing is broken for customers: web and api keep serving." Actions: "Ask the agent to fix" (primary → AgentOps.dc.html), "Retry", "View logs".

FLOW 2 (staging fix): api build on staging failed at 15:12: commit c41e9a2 "Bump sharp to 0.34" (Marc). Log: `npm ERR! engine Unsupported engine: wanted: {"node":">=22"} (current: {"node":"20.11.1"})` and `Build failed: npm ci exited with code 1`. Cause: railway build config pins Node 20 (`nixpacks.toml`: `NIXPACKS_NODE_VERSION = "20"`). Agent session ses_2c91 (Ops mode, claude-sonnet-5-5, budget "$0.84 of $2.00", started by Marc). Steps: (yari) read build logs for api on staging → observation log excerpt → message explaining cause → (Sandbox) clone kestrel-studio/mango-shop at c41e9a2 → (Sandbox) edit nixpacks.toml: 20 → 22 (diff) → (Sandbox) `npm ci && npm run build` ✓ 38 s → (yari) push branch yari/fix-node-22 → Plan card plan_5B8X "Rebuild api on staging with Node 22" · 1 change on Railway · No cost change · Low risk · staging (no approval needed). Apply → api Online on staging, deploy 9d02b6f "Use Node 22 for builds" by Agent.
Guardrail example (AgentWaiting or AgentOps): agent wanted to delete old unused database `postgres-old` on staging → "Blocked: agents cannot delete databases. I listed it in the plan notes so a human can decide."

Ops session ending in approval (AgentWaiting): ses_b410, goal "Wake worker and raise concurrency to 4 on production before Monday's sale", plan_9KD1 · 1 change on Railway · +$5.00/month · Low risk · production → "Waiting for approval on production" → link Approvals.dc.html. Requested 12 min ago.

Approvals pending (count 2 everywhere): plan_7Q2M (Marc Etienne, mango-shop production, 4 changes, +$7.00/mo, Medium, 4 min ago) and plan_9KD1 (Agent · ses_b410, mango-shop production, 1 change, +$5.00/mo, Low, 12 min ago). Recently decided: plan_4HCX harbor-portal production by CI github-actions, approved by Sami Joseph 1 h ago; plan_2WQA kestrel-site production by Lina, approved by Sami yesterday.

Code mode (FLOW 3): session ses_7f3a, mode Code, model claude-sonnet-5-5, budget "$1.36 of $3.00", repo kestrel-studio/mango-shop, branch yari/session-7f3a, preview env pr-42, preview URL mango-shop-7f3a.preview.yari.run (Private). Prompt: "Add product reviews. Customers who bought a product can rate it and leave a short review. Email the shop owner when a new review arrives." Files: new `apps/api/src/routes/reviews.ts`, new `apps/api/db/migrations/0043_reviews.sql`, new `apps/api/src/email/review-notification.ts`, new `apps/web/src/components/ReviewForm.tsx`, modified `apps/web/src/pages/product/[slug].tsx`, modified `apps/api/src/server.ts`. Terminal: `pnpm install` (done 11.2 s), `pnpm --filter api test` (14 passed), `pnpm --filter web build` (ok), `pnpm dev` (web on :3000, api on :4000). Plan card plan_P42D: "Add postgres on Supabase and connect it to api" on pr-42: Create · Supabase · postgres (Free tier, eu-central-1, same region as api) $0.00; Update · Railway · api "Inject DATABASE_URL" $0.00 → "2 changes across 2 providers. No cost change." Low risk, no approval (preview). Promote to production → plan_C3PR "Promote yari/session-7f3a to production": Supabase postgres Update "Run migration 0043_reviews (adds 1 table)", Railway api Update "Deploy 5e7a0b1 “Add product reviews”", Vercel web Update "Deploy 5e7a0b1" → "3 changes across 3 providers. About +$0.00 per month." Medium risk, requires approval (Lina Pierre, Sami Joseph).

Members (Members board): Sami Joseph · sami@kestrel.studio · Human · Owner · Organization · active 2 min ago; Lina Pierre · lina@kestrel.studio · Human · Admin · Organization · 14 min ago; Marc Etienne · marc@kestrel.studio · Human · Developer · Workspace: Mango Market · now; Ana Ruiz · ana@mangomarket.co · Human · Viewer (client contact) · Project: mango-shop · 3 days ago; Agent · Agent · Developer · Workspace: Mango Market · 12 min ago; github-actions · CI · Developer · Environment: mango-shop / staging · 1 h ago; deploy-bot · API key (key_…9f2c) · Developer · Project: mango-shop · yesterday. Teams: "Mango team" (Lina, Marc, Agent; Mango Market), "Harbor team" (Sami, Lina; Harbor Clinic), "Clients" (Ana; viewers).
Permission matrix (rows Owner, Admin, Developer, Viewer, Agent; columns Deploy, Read secrets, Approve, Run agent):
production: Owner ✓ ✓ ✓ ✓ · Admin ✓ ✓ ✓ ✓ · Developer "With approval" ✗ ✗ ✓ · Viewer ✗ ✗ ✗ ✗ · Agent "With approval" "Never" ✗ "—".
staging: Owner/Admin all ✓ · Developer ✓ ✓ ✗ ✓ · Viewer ✗ ✗ ✗ ✗ · Agent ✓ "Never" ✗ "—".
pr-42: Owner/Admin ✓ ✓ "Not needed" ✓ · Developer ✓ ✓ "Not needed" ✓ · Viewer ✗ ✗ ✗ ✗ · Agent ✓ "Never" ✗ "—".
Roles: Owner "Everything, including billing, deleting the organization and transferring ownership." · Admin "Manages people, connections and protected environments, and approves production changes." · Developer "Builds and deploys. Production changes wait for an admin's approval." · Viewer "Sees projects, canvases and logs. Cannot change anything or read secrets."

Connections (catalog): Connected: Railway (PaaS; kinds web, worker, cron, database; OAuth; scope "Workspace kestrel-studio · 3 projects"; used by Mango Market, Internal; last validated 2 h ago), Vercel (PaaS; web, static, function; scoped token; scope "Team kestrel · 2 projects"; Mango Market, Harbor Clinic, Internal; 2 h ago), Supabase (Databases; database; OAuth; scope "Organization kestrel · 2 projects"; Mango Market, Harbor Clinic; 1 h ago), Backblaze B2 (Storage; bucket; scoped application key; "Buckets mango-media, mango-media-staging"; Mango Market; 3 h ago), Resend (Email; email; API key, account-wide only — calm note; Mango Market, Harbor Clinic; 2 h ago). Planned: Render, Fly.io, Netlify (PaaS); AWS, Google Cloud, Hetzner (Clouds); Neon, PlanetScale (Databases); Cloudflare R2 (Storage); Postmark (Email); Upstash (Queues); Meilisearch Cloud (Search).

Usage (Nov 1–8): per workspace Mango Market $38.40, Harbor Clinic $12.10, Internal $4.75 (total $55.25 of $150 monthly limit). Per model: claude-sonnet-5-5 $41.20, gpt-5.5 $14.05, local-llama $0.00 (self-hosted, 3.1 h). Sessions: ses_7f3a Code "Add product reviews" 46 min $1.36 Running; ses_b410 Ops "Wake worker for Monday's sale" 6 min $0.31 Waiting for approval; ses_2c91 Ops "Why is api failing on staging?" 9 min $0.84 Fixed; ses_91ee Ops "Add a staging copy of postgres" 4 min $0.22 Applied; ses_60ab Code "Harbor: booking reminders page" 1 h 12 min $2.00 Budget reached; ses_33c0 Ops "Explain last night's latency spike" 3 min $0.09 Answered.

Audit log rows (today unless noted): 15:44 Agent (ses_b410) Requested approval plan_9KD1 · worker · production; 15:40 Marc Etienne Requested approval plan_7Q2M · mango-shop · production; 15:31 Agent (ses_2c91) Applied plan plan_5B8X · api · staging; 15:27 Agent (ses_2c91) Blocked by guardrail "delete database" · postgres-old · staging; 15:12 Marc Etienne Deployed c41e9a2 (failed) · api · staging; 15:02 github-actions (CI) Deployed 7b11f3e · api · pr-42; 14:48 Lina Pierre Revealed secret STRIPE_SECRET_KEY (re-authenticated) · api · production; 14:10 deploy-bot (API key) Created environment pr-42 · mango-shop; 13:55 Sami Joseph Changed role of Ana Ruiz to Viewer · mango-shop; 12:30 Lina Pierre Connected provider Resend · organization; 11:02 yari.run (system) Detected drift on api · staging; yesterday 18:20 Sami Joseph Approved plan_2WQA · kestrel-site · production.

Drift (api on staging): detected 11:02 by yari.run: "Changed outside yari.run" — in the Railway dashboard: replicas 1 → 2, variable LOG_LEVEL added (= debug). Options: "Keep these changes" (adopt into yari.run, creates a plan with no infra change) or "Restore yari.run version" (plan: set replicas 1, remove LOG_LEVEL, −$5.00/month).

## 7. Copy tone
Short, calm, specific: what will happen and what it costs. Never alarming, never cute, no exclamation marks, no marketing. Use these where they fit verbatim: "4 changes across 3 providers. About +$7.00 per month." · "Production is protected. An admin needs to approve this." · "The agent works in its own sandbox. It cannot see your provider keys." · "Nothing waiting. Everything is calm." · "This session reached its $2.00 budget. Raise it or start a new session."

## 8. Canvas geometry (production; reused by all canvas-backed boards)
Canvas area = everything right of the 48px rail and below the 48px top bar, dotted bg: `background-color: #0B0C10; background-image: radial-gradient(#24262F 1px, transparent 1px); background-size: 20px 20px`. Graph group positioned at left 120px top 90px inside it (coordinates below are inside the group). Node width 240, height ~108.
web (80,260) · api (480,260) · worker (480,520) · media (900,60) · email (900,248) · postgres (900,420) + strip below (to ~y 560).
Edge paths: web→api `M320 314 H480`, label API_URL at (400,314) · api→media `M720 290 H800 V114 H900` label BUCKET_ENDPOINT at (800,200) · api→email `M720 302 H900` label RESEND_API_KEY •••• at (810,302) · api→postgres `M720 338 H840 V474 H900` label DATABASE_URL at (840,406) · worker→postgres `M720 574 H780 V500 H900` label DATABASE_URL at (840,500).
Bottom-left controls (left 16, bottom 16 of canvas area): vertical stack of bordered 32px icon buttons: zoom in, zoom out, fit, then a gap, undo, redo; plus "100%" zoom readout. Top-right of canvas: "Add" secondary button with plus + kbd ⌘K. Boards that put a drawer over the canvas may simply let the drawer cover the right part of the graph.

## 9. Board list (file → page → size → links). All desktop boards: w 1440, expand fill. Mobile: 390×844.
start: Main (index), Tokens (1440×1400, static), ComponentsBase (1440×1900, static), ComponentsProduct (1440×2300, static)
onboarding: Onboarding1 → Onboarding2 → Onboarding3 → Onboarding4 → Canvas
org: OrgHome, Connections, Members, Permissions, Roles, AuditLog (1440×1000), Usage (1440×1100)
canvas: Canvas (prod), CanvasStaging, DriftReview, Palette, ServicePanel, PlanReview, PlanApply, PlanFailed
approvals: Approvals, ApprovalsEmpty
agent: AgentOps, AgentWaiting, AgentBudget
code: CodeStart, CodeWorking, CanvasPR42, CodePromote
light: CanvasLight, PlanReviewLight, CodeLight
states: EmptyProjects, EmptyConnections, ProviderPlanned, Loading, ErrorState
mobile: MobileProjects, MobileProject, MobileNotification, MobileApprovals, MobileApproval, MobileAgent, MobileAgentSession (390×844), Tray (480×420)

Flow links (hrefs, exact file names + `.dc.html`):
- F1: Canvas "Deploy" → PlanReview; PlanReview "Request approval" → in-board requested state with a link "Continue on Lina's phone" → MobileNotification; MobileNotification tap first notification → MobileApproval; MobileApproval Approve → confirm sheet → approved state with link "Back to desktop" → PlanApply; PlanApply (auto progresses applying → applied) "Back to canvas" → Canvas.
- F2: CanvasStaging api node "Ask the agent" → AgentOps (steps reveal progressively; plan card → in-board staging plan drawer → Apply → applying → applied → "api is online on staging") → "Back to canvas" → CanvasStaging (note: the board still shows the failed snapshot; acceptable) or Canvas.
- F3: CodeStart (type/submit or chip) → CodeWorking (stages; Preview toggle; plan card Accept → toast "postgres added to pr-42" with "View on canvas" → CanvasPR42; "Deploy to preview (pr-42)" → in-board deploying→live; "Promote to production" → CodePromote) → CodePromote "Request approval" → requested state with link → Approvals.
- F4: Onboarding1 → Onboarding2 (pick Railway → connect step → Connected) → Onboarding3 (GitHub repo kestrel-studio/mango-shop) → Onboarding4 Review "Create project" → Canvas.
- Shared nav: org sidebar links (§3c), project rail links (§3b), env dropdown on canvases, bell → Approvals, "Agent" button → AgentOps, PlanFailed "Ask the agent to fix" → AgentOps, CanvasStaging drift "Review drift" → DriftReview, Canvas "Add" → Palette, Canvas node click → ServicePanel, mobile bottom tabs → MobileProjects / MobileApprovals / MobileAgent (Activity and Settings tabs may stay on the current board), Tray "Open yari.run" → Canvas, "Pending approvals: 2" → Approvals.
