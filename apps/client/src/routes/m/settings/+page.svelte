<script lang="ts">
  import Screen from '#lib/components/mobile/Screen.svelte';
  import { Button } from '#lib/components/ui/button/index.js';
  import { Input } from '#lib/components/ui/input/index.js';
  import { USE_FAKE_DATA } from '#lib/env.js';
  import { platform } from '#lib/platform.js';
  import { session } from '#lib/session.svelte.js';

  let signedOut = $state(false);

  async function signOut() {
    await session.signOut();
    signedOut = true;
  }
</script>

<svelte:head><title>Settings · yari.run</title></svelte:head>

<Screen title="Settings">
  <div class="space-y-5">
    <section class="space-y-4 rounded-xl border border-border bg-card p-4">
      <div class="space-y-1.5">
        <label for="api-url" class="text-xs font-medium">API URL</label>
        <Input
          id="api-url"
          bind:value={session.apiUrl}
          disabled={USE_FAKE_DATA}
          inputmode="url"
          autocapitalize="off"
          autocorrect="off"
          spellcheck="false"
          class="h-9 font-mono text-[13px]"
        />
        <p class="text-xs text-muted-foreground">
          {USE_FAKE_DATA
            ? 'Demo data is on, so no API calls are made.'
            : 'The Connect API this app talks to.'}
        </p>
      </div>

      {#if session.devControls}
        <div class="space-y-1.5">
          <label for="dev-subject" class="text-xs font-medium">Dev subject</label>
          <Input
            id="dev-subject"
            bind:value={session.devSubject}
            placeholder="user id sent as X-Yari-Dev-Subject"
            autocapitalize="off"
            autocorrect="off"
            spellcheck="false"
            class="h-9 font-mono text-[13px]"
          />
          <p class="text-xs text-muted-foreground">
            Development builds only. Kept in memory and never saved.
          </p>
        </div>
      {/if}
    </section>

    <section class="rounded-xl border border-border bg-card p-4 text-xs">
      <dl class="grid grid-cols-[110px_1fr] gap-y-2">
        <dt class="text-muted-foreground">Data</dt>
        <dd>{USE_FAKE_DATA ? 'Demo data' : 'Live API'}</dd>
        <dt class="text-muted-foreground">Platform</dt>
        <dd class="capitalize">{platform}</dd>
      </dl>
    </section>

    <Button variant="outline" class="w-full" onclick={signOut}>Sign out</Button>
    {#if signedOut}
      <p class="text-center text-xs text-muted-foreground">
        Signed out. Credentials were cleared from memory.
      </p>
    {/if}
  </div>
</Screen>
