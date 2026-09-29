// @ts-nocheck — vendored from Heimdall-Panel (upstream pg-ui); keep byte-compatible with upstream.
import { useEffect, useMemo, useState } from 'react';
import { HttpUtil } from '@/utils';
import type { AdminDetails } from '@/pg-ui/service/api';

type ApiMsg<T = unknown> = {
  success?: boolean
  msg?: string
  obj?: T
}

type CurrentAdmin = AdminDetails

function toNumber(value: unknown): number {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

function toString(value: unknown): string {
  return typeof value === 'string' ? value : value == null ? '' : String(value)
}

function normalizeCurrentAdmin(raw: any): CurrentAdmin | null {
  if (!raw || typeof raw !== 'object') return null

  const role = raw.role && typeof raw.role === 'object' ? raw.role : {}
  const permissions = role.permissions ?? raw.permissions ?? {}

  return {
    id: toNumber(raw.id),
    username: toString(raw.username),
    status: toString(raw.status),
    roleId: toNumber(raw.roleId ?? raw.role_id),
    role_id: toNumber(raw.role_id ?? raw.roleId),
    api_access: Boolean(raw.apiAccess ?? raw.api_access),
    apiAccess: Boolean(raw.apiAccess ?? raw.api_access),
    profileTitle: toString(raw.profileTitle ?? raw.profile_title),
    profile_title: toString(raw.profile_title ?? raw.profileTitle),
    permissions,
    limits: role.limits ?? raw.limits ?? {},
    features: role.features ?? raw.features ?? {},
    access: role.access ?? raw.access ?? {},
    allowedGroupIds: role.allowedGroupIds ?? role.allowed_group_ids ?? raw.allowedGroupIds ?? raw.allowed_group_ids,
    allowed_group_ids: role.allowed_group_ids ?? role.allowedGroupIds ?? raw.allowed_group_ids ?? raw.allowedGroupIds,
    role: {
      id: toNumber(role.id),
      name: toString(role.name),
      slug: toString(role.slug),
      is_builtin: Boolean(role.is_builtin ?? role.builtIn),
      builtIn: Boolean(role.builtIn ?? role.is_builtin),
      is_owner: Boolean(role.is_owner ?? role.ownerRole ?? role.owner_role),
      ownerRole: Boolean(role.ownerRole ?? role.is_owner ?? role.owner_role),
      owner_role: Boolean(role.owner_role ?? role.ownerRole ?? role.is_owner),
      permissions,
      limits: role.limits ?? raw.limits ?? {},
      features: role.features ?? raw.features ?? {},
      access: role.access ?? raw.access ?? {},
      allowedGroupIds: role.allowedGroupIds ?? role.allowed_group_ids ?? raw.allowedGroupIds ?? raw.allowed_group_ids,
      allowed_group_ids: role.allowed_group_ids ?? role.allowedGroupIds ?? raw.allowed_group_ids ?? raw.allowedGroupIds,
    },
  }
}

const SESSION_CACHE_KEY = 'nova_current_admin';

function readCachedAdmin(): CurrentAdmin | null {
  try {
    if (typeof window === 'undefined' || !window.sessionStorage) return null;
    const raw = sessionStorage.getItem(SESSION_CACHE_KEY);
    if (!raw) return null;
    return normalizeCurrentAdmin(JSON.parse(raw));
  } catch {
    return null;
  }
}

function writeCachedAdmin(value: CurrentAdmin | null) {
  try {
    if (typeof window === 'undefined' || !window.sessionStorage) return;
    if (value) {
      sessionStorage.setItem(SESSION_CACHE_KEY, JSON.stringify(value));
    } else {
      sessionStorage.removeItem(SESSION_CACHE_KEY);
    }
  } catch {}
}

let sharedAdmin: CurrentAdmin | null = readCachedAdmin();
let sharedLoading: boolean = !sharedAdmin;
let sharedError: string = '';
let fetchPromise: Promise<CurrentAdmin | null> | null = null;
const listeners = new Set<() => void>();

function emitChange() {
  listeners.forEach((listener) => {
    try {
      listener();
    } catch {}
  });
}

export function clearAdminCache() {
  sharedAdmin = null;
  sharedLoading = true;
  sharedError = '';
  writeCachedAdmin(null);
  emitChange();
}

export async function fetchCurrentAdmin(force = false): Promise<CurrentAdmin | null> {
  if (fetchPromise && !force) {
    return fetchPromise;
  }

  fetchPromise = (async () => {
    try {
      const msg = await HttpUtil.get('/panel/api/admins/current', undefined, { silent: true }) as ApiMsg<unknown>;
      if (msg?.success === false) {
        throw new Error(msg?.msg || 'Failed to load current admin');
      }

      const normalized = normalizeCurrentAdmin(msg?.obj ?? msg);
      if (!normalized) {
        throw new Error('Invalid current admin payload');
      }

      sharedAdmin = normalized;
      sharedError = '';
      writeCachedAdmin(normalized);
      return normalized;
    } catch (err) {
      if (!sharedAdmin) {
        sharedAdmin = null;
        sharedError = err instanceof Error ? err.message : 'Failed to load current admin';
        writeCachedAdmin(null);
      }
      return sharedAdmin;
    } finally {
      sharedLoading = false;
      fetchPromise = null;
      emitChange();
    }
  })();

  return fetchPromise;
}

export function useAdmin() {
  const [admin, setAdmin] = useState<CurrentAdmin | null>(() => sharedAdmin);
  const [isLoading, setIsLoading] = useState<boolean>(() => !sharedAdmin && sharedLoading);
  const [error, setError] = useState<string>(() => sharedError);

  useEffect(() => {
    const handleChange = () => {
      setAdmin(sharedAdmin);
      setIsLoading(sharedLoading);
      setError(sharedError);
    };

    listeners.add(handleChange);

    if (!sharedAdmin || (!fetchPromise && !sharedAdmin)) {
      fetchCurrentAdmin();
    }

    return () => {
      listeners.delete(handleChange);
    };
  }, []);

  return useMemo(() => ({
    admin,
    isLoading,
    loading: isLoading,
    error,
  }), [admin, isLoading, error]);
}
