/**
 * WebSocket wire protocol version this bundle understands. Must match
 * collector.ProtocolVersion in the backend.
 */
export const PROTOCOL_VERSION = 2;

const RELOAD_KEY = 'dg_protocol_reload';

/** What to do with a message carrying a given protocol version. */
export type ProtocolAction = 'accept' | 'reload' | 'ignore';

/**
 * Decides how to handle a message whose envelope carries `version`.
 *
 * A matching version is accepted. A different one means this bundle is stale
 * (or the server is), so the page reloads once to fetch a matching bundle.
 * If a reload for that server version already happened this session and the
 * bundle still disagrees, the message is ignored rather than looping reloads
 * or rendering a payload this bundle cannot interpret.
 *
 * The reload marker is what stops a loop, so this fails closed: when session
 * storage is unavailable or throws, a mismatch is ignored instead of
 * reloading. It never throws.
 */
export function protocolAction(version: number, storage: Storage | null = safeSessionStorage()): ProtocolAction {
  if (version === PROTOCOL_VERSION) {
    try {
      storage?.removeItem(RELOAD_KEY);
    } catch {
      // A stale marker only costs one skipped reload later.
    }
    return 'accept';
  }
  if (!storage) return 'ignore';
  try {
    if (storage.getItem(RELOAD_KEY) === String(version)) return 'ignore';
    storage.setItem(RELOAD_KEY, String(version));
    // Read back: a write that silently didn't stick would loop too.
    if (storage.getItem(RELOAD_KEY) !== String(version)) return 'ignore';
  } catch {
    return 'ignore';
  }
  return 'reload';
}

function safeSessionStorage(): Storage | null {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}
