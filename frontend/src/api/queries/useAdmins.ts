import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { AdminAccount, AdminRoleDoc, AdminStats, CurrentAdmin } from '@/lib/rbac';

export type { AdminAccount, AdminRoleDoc, AdminStats, CurrentAdmin };

export interface AdminPayload {
  username?: string;
  password?: string;
  roleId?: number;
  status?: string;
}

export interface AdminRolePayload {
  name?: string;
  permissions?: Record<string, unknown>;
  limits?: Record<string, unknown>;
  features?: Record<string, unknown>;
  access?: Record<string, unknown>;
}

async function fetchAdmins(): Promise<AdminAccount[]> {
  const msg = await HttpUtil.get('/panel/api/admins/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch admins');
  return Array.isArray(msg.obj) ? (msg.obj as AdminAccount[]) : [];
}

async function fetchAdminStats(): Promise<AdminStats | null> {
  const msg = await HttpUtil.get('/panel/api/admins/stats', undefined, { silent: true });
  if (!msg?.success) return null;
  return (msg.obj as AdminStats) ?? null;
}

async function fetchRoles(): Promise<AdminRoleDoc[]> {
  const msg = await HttpUtil.get('/panel/api/admin-roles/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch admin roles');
  return Array.isArray(msg.obj) ? (msg.obj as AdminRoleDoc[]) : [];
}

async function fetchCurrentAdmin(): Promise<CurrentAdmin | null> {
  const msg = await HttpUtil.get('/panel/api/admins/current', undefined, { silent: true });
  if (!msg?.success) return null;
  return (msg.obj as CurrentAdmin) ?? null;
}

export function useAdminsQuery() {
  const query = useQuery({ queryKey: keys.admins.list(), queryFn: fetchAdmins });
  return {
    admins: query.data ?? [],
    loading: query.isLoading,
    fetched: query.isFetched,
    fetchError: query.error ? String((query.error as Error).message ?? query.error) : '',
    refetch: query.refetch,
  };
}

export function useAdminStatsQuery() {
  const query = useQuery({ queryKey: keys.admins.stats(), queryFn: fetchAdminStats });
  return { stats: query.data ?? null, loading: query.isLoading };
}

export function useAdminRolesQuery() {
  const query = useQuery({ queryKey: keys.admins.roles(), queryFn: fetchRoles });
  return {
    roles: query.data ?? [],
    loading: query.isLoading,
    fetched: query.isFetched,
    fetchError: query.error ? String((query.error as Error).message ?? query.error) : '',
    refetch: query.refetch,
  };
}

export function useCurrentAdminQuery() {
  const query = useQuery({ queryKey: keys.admins.current(), queryFn: fetchCurrentAdmin });
  return { current: query.data ?? null, loading: query.isLoading };
}

export function useAdminMutations() {
  const qc = useQueryClient();
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: keys.admins.root() });
  };

  const create = useMutation({
    mutationFn: (payload: AdminPayload) =>
      HttpUtil.post('/panel/api/admins/add', payload).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  const update = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: AdminPayload }) =>
      HttpUtil.post(`/panel/api/admins/update/${id}`, payload).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  const remove = useMutation({
    mutationFn: (id: number) =>
      HttpUtil.post(`/panel/api/admins/del/${id}`, {}).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  const setStatus = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      HttpUtil.post(`/panel/api/admins/${enabled ? 'enable' : 'disable'}/${id}`, {}).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  return { create, update, remove, setStatus };
}

export function useAdminRoleMutations() {
  const qc = useQueryClient();
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: keys.admins.roles() });
  };

  const create = useMutation({
    mutationFn: (payload: AdminRolePayload) =>
      HttpUtil.post('/panel/api/admin-roles/add', payload).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  const update = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: AdminRolePayload }) =>
      HttpUtil.post(`/panel/api/admin-roles/update/${id}`, payload).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  const duplicate = useMutation({
    mutationFn: (id: number) =>
      HttpUtil.post(`/panel/api/admin-roles/duplicate/${id}`, {}).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  const remove = useMutation({
    mutationFn: (id: number) =>
      HttpUtil.post(`/panel/api/admin-roles/del/${id}`, {}).then((m) => {
        if (!m?.success) throw new Error(m?.msg || 'failed');
        return m;
      }),
    onSuccess: invalidate,
  });

  return { create, update, duplicate, remove };
}
