import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: 'run.yari.app',
  appName: 'yari.run',
  // The shared SvelteKit static build (adapter-static). Build it first:
  //   pnpm --filter client build
  webDir: '../client/build',
  server: {
    // Keep the defaults: the app is served from https://localhost inside the WebView.
    // No `server.url` (no live remote content in production builds).
    androidScheme: 'https',
    // Capacitor >= 7.3 loads this path instead of the default /index.html, so the app
    // opens on the mobile layout. The static build serves it through the SPA fallback.
    appStartPath: '/m'
  },
  android: {
    // Release builds must not allow remote debugging of the WebView.
    webContentsDebuggingEnabled: false
  },
  plugins: {
    LocalNotifications: {
      // Android small icon: res/drawable/ic_stat_yari.xml (monochrome vector).
      smallIcon: 'ic_stat_yari'
    }
  }
};

export default config;
