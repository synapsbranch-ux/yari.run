/**
 * Push notifications: interface only.
 *
 * TODO(shell): not implemented. When it is, pick one transport (FCM on Android via
 * `@capacitor/push-notifications`, APNs on iOS) and implement `PushService` behind this
 * interface. Rules for the implementation:
 * - The device token is sent to the yari.run API by the web layer, never stored in plain text.
 * - Payloads carry ids only (for example an approval id). Titles and bodies are fetched with
 *   the user's session after the app opens, so no sensitive plan data transits a push provider.
 * - Nothing in this file may call a cloud provider or hold provider credentials.
 */

export type PushPermission = 'granted' | 'denied' | 'prompt';

export interface PushRegistration {
  /** Opaque transport token (FCM registration token or APNs device token). */
  token: string;
  platform: 'android' | 'ios';
}

/** The minimal payload shape. Ids only, see the rules above. */
export interface PushMessage {
  kind: 'approval_requested' | 'plan_finished' | 'agent_session_event';
  /** Id of the entity the message is about, resolved through the API after opening. */
  entityId: string;
}

export interface PushService {
  checkPermission(): Promise<PushPermission>;
  requestPermission(): Promise<PushPermission>;
  /** Registers this device with the transport and resolves with its token. */
  register(): Promise<PushRegistration>;
  unregister(): Promise<void>;
  /** Fires when a message arrives in the foreground. Returns an unsubscribe function. */
  onMessage(handler: (message: PushMessage) => void): () => void;
  /** Fires when the user taps a notification. Returns an unsubscribe function. */
  onOpen(handler: (message: PushMessage) => void): () => void;
}
