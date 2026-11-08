import { redirect } from '@sveltejs/kit';
import { DEMO_CANVAS_PATH } from '#lib/env.js';
import { platform } from '#lib/platform.js';

// The Capacitor shell opens the phone layout. Everything else opens the demo canvas.
export function load() {
  redirect(307, platform === 'capacitor' ? '/m' : DEMO_CANVAS_PATH);
}
