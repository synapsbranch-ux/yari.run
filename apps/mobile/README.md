# yari.run mobile (Capacitor)

A thin native shell around the shared UI in `apps/client`. The app starts on the mobile routes
(`/m`) through `server.appStartPath` (available since Capacitor 7.3.0), keeps the default
`androidScheme: 'https'` and uses no `server.url`.

## Layout

- `capacitor.config.ts`: app id `run.yari.app`, `webDir: '../client/build'`, start path `/m`.
- `android/`: the generated Android project (placeholder launcher icon and splash, backup disabled,
  monochrome notification icon `ic_stat_yari`).
- `src/push.ts`: push notification interface only, `TODO(shell)`.
- `PLATFORM.md`: ready to paste TypeScript for `apps/client/src/lib/platform.ts`.

Plugins: `@capacitor/local-notifications` and `@aparajita/capacitor-secure-storage`. Tokens must
only ever go through the secure store (Android Keystore, iOS Keychain), never `Preferences`,
`localStorage` or IndexedDB.

## Workflow

```sh
pnpm install
pnpm --filter client build
pnpm --filter mobile exec cap sync android
pnpm --filter mobile exec cap open android     # Android Studio
cd apps/mobile/android && ./gradlew assembleDebug
```

Building needs a JDK 21, the Android SDK (platform 36) and `ANDROID_HOME`. Syncing does not.

iOS only works on macOS with Xcode and CocoaPods or SwiftPM:

```sh
pnpm --filter mobile add @capacitor/ios@8.6.0
pnpm --filter mobile exec cap add ios
pnpm --filter mobile exec cap sync ios
```

## API URL

On a device `localhost` is the phone, and Android blocks cleartext HTTP. Build the client with a
public `https` `PUBLIC_API_URL` for mobile builds. The API must allow the app origin
`https://localhost` (Android) and `capacitor://localhost` (iOS) in its CORS settings.
