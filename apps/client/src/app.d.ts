// See https://svelte.dev/docs/kit/types#app.d.ts
declare global {
  namespace App {
    // interface Error {}
    // interface PageState {}
  }

  interface ImportMetaEnv {
    readonly PUBLIC_API_URL?: string;
    readonly PUBLIC_USE_FAKE_DATA?: string;
  }
}

export {};
