// A static single-page app: no server rendering, index.html is the fallback for every route.
// The same build is wrapped by the Tauri and Capacitor shells.
export const ssr = false;
export const prerender = false;
export const trailingSlash = 'never';
