import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// One static SPA build for the browser, the Tauri 2 shell and the Capacitor shell.
// - SSR is off (see src/routes/+layout.ts) and index.html is the fallback for every route.
// - Public variables (PUBLIC_API_URL, PUBLIC_USE_FAKE_DATA) come from the repository root .env.
//   When it is missing, safe defaults apply: fake data on (see src/lib/env.ts).
export default defineConfig({
  envDir: '../..',
  envPrefix: 'PUBLIC_',
  plugins: [
    tailwindcss(),
    sveltekit({
      compilerOptions: {
        // Runes mode for our code, not for dependencies.
        runes: ({ filename }) =>
          filename.split(/[/\\]/).includes('node_modules') ? undefined : true
      },
      adapter: adapter({
        pages: 'build',
        assets: 'build',
        fallback: 'index.html',
        precompress: false,
        strict: false
      }),
      paths: {
        // SPA fallback pages always use root-relative paths. Tauri and Capacitor serve the
        // build from the root of a custom origin (tauri://localhost, capacitor://localhost).
        relative: false
      }
    })
  ]
});
