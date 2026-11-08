//! yari.run desktop shell.
//!
//! Scope is deliberately tiny: window, system tray and one native notification.
//! No business logic, no credentials and no provider calls live in Rust. Everything
//! the user sees comes from the shared web UI in `apps/client`.

use tauri::{
    menu::{MenuBuilder, MenuItemBuilder},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Manager,
};
use tauri_plugin_notification::NotificationExt;

// TODO(shell): replace these fake counters. The web UI polls `PlanService.ListPendingApprovals`
// and the agent session list through the client's API layer and should push the numbers here
// (for example via a small Tauri command or event called from `src/lib/platform.ts`).
// Rust must never call the API itself.
const FAKE_PENDING_APPROVALS: u32 = 3;
const FAKE_AGENT_SESSIONS_RUNNING: u32 = 2;

const MENU_OPEN: &str = "open";
const MENU_APPROVALS: &str = "approvals";
const MENU_AGENTS: &str = "agents";
const MENU_QUIT: &str = "quit";

/// Client routes opened by the tray menu.
const ROUTE_HOME: &str = "/";
const ROUTE_APPROVALS: &str = "/approvals";
const ROUTE_AGENTS: &str = "/m/agent";

/// Shows, un-minimizes and focuses the main window.
fn show_main_window(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}

/// Navigates the SPA to `route` and brings the window forward.
///
/// The script first dispatches a cancelable `yari:navigate` event so the client can route
/// in-app (keeping in-memory state). If nothing calls `preventDefault()`, it falls back to a
/// plain page load of the route, which the static build serves through its SPA fallback.
fn open_route(app: &AppHandle, route: &str) {
    show_main_window(app);
    if let Some(window) = app.get_webview_window("main") {
        // `route` is one of the compile-time constants above, so interpolating is safe.
        let script = format!(
            "(function(p){{var e=new CustomEvent('yari:navigate',{{detail:{{path:p}},cancelable:true}});\
             if(window.dispatchEvent(e)){{window.location.assign(p);}}}})('{route}')"
        );
        let _ = window.eval(&script);
    }
}

fn build_tray(app: &AppHandle) -> tauri::Result<()> {
    let open = MenuItemBuilder::with_id(MENU_OPEN, "Open yari.run").build(app)?;
    let approvals = MenuItemBuilder::with_id(
        MENU_APPROVALS,
        format!("Pending approvals: {FAKE_PENDING_APPROVALS}"),
    )
    .build(app)?;
    let agents = MenuItemBuilder::with_id(
        MENU_AGENTS,
        format!("Agent sessions running: {FAKE_AGENT_SESSIONS_RUNNING}"),
    )
    .build(app)?;
    let quit = MenuItemBuilder::with_id(MENU_QUIT, "Quit").build(app)?;

    let menu = MenuBuilder::new(app)
        .item(&open)
        .separator()
        .item(&approvals)
        .item(&agents)
        .separator()
        .item(&quit)
        .build()?;

    let mut tray = TrayIconBuilder::with_id("main")
        .tooltip("yari.run")
        .menu(&menu)
        // Left click shows the window, right click opens the menu.
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id().as_ref() {
            MENU_OPEN => open_route(app, ROUTE_HOME),
            MENU_APPROVALS => open_route(app, ROUTE_APPROVALS),
            MENU_AGENTS => open_route(app, ROUTE_AGENTS),
            MENU_QUIT => app.exit(0),
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                show_main_window(tray.app_handle());
            }
        });

    if let Some(icon) = app.default_window_icon() {
        tray = tray.icon(icon.clone());
    }
    tray.build(app)?;
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_notification::init())
        .setup(|app| {
            build_tray(app.handle())?;

            // TODO(shell): drive this from the real pending approvals count.
            if FAKE_PENDING_APPROVALS > 0 {
                let _ = app
                    .notification()
                    .builder()
                    .title("yari.run")
                    .body(format!(
                        "{FAKE_PENDING_APPROVALS} approvals are waiting for you."
                    ))
                    .show();
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running yari.run desktop");
}
