/**
 * Default limits applied to every client created from the panel.
 *
 * The panel has no server-side setting for this, so the values live in
 * localStorage per browser profile and are read when a creation form opens. Both
 * numbers follow the client model's own convention: `0` means "no limit", which is
 * what the forms used before, so the feature is inert until someone sets a value.
 */

const STORAGE_KEY = 'nova.clientDefaults';

export interface ClientDefaults {
  /** Max simultaneous IPs for a new client. 0 = unlimited. */
  limitIp: number;
  /** Max registered devices (HWID) for a new client. 0 = unlimited. */
  limitHwid: number;
}

export const EMPTY_CLIENT_DEFAULTS: ClientDefaults = { limitIp: 0, limitHwid: 0 };

/** Coerce anything to a non-negative integer; anything unusable becomes 0. */
function toCount(value: unknown): number {
  const n = Math.trunc(Number(value));
  if (!Number.isFinite(n) || n < 0) return 0;
  return n;
}

export function readClientDefaults(): ClientDefaults {
  if (typeof window === 'undefined') return { ...EMPTY_CLIENT_DEFAULTS };
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { ...EMPTY_CLIENT_DEFAULTS };
    const parsed = JSON.parse(raw) as Partial<ClientDefaults> | null;
    if (!parsed || typeof parsed !== 'object') return { ...EMPTY_CLIENT_DEFAULTS };
    return { limitIp: toCount(parsed.limitIp), limitHwid: toCount(parsed.limitHwid) };
  } catch {
    return { ...EMPTY_CLIENT_DEFAULTS };
  }
}

export function writeClientDefaults(next: Partial<ClientDefaults>): ClientDefaults {
  const value: ClientDefaults = {
    limitIp: toCount(next.limitIp),
    limitHwid: toCount(next.limitHwid),
  };
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
  } catch {
    /* private mode or full quota: keep going, the caller still gets the value */
  }
  return value;
}

export function hasClientDefaults(d: ClientDefaults = readClientDefaults()): boolean {
  return d.limitIp > 0 || d.limitHwid > 0;
}
