import { useQuery } from '@tanstack/react-query';

import { useAdmin } from '@/pg-ui/hooks/use-admin';
import { canReadResourcePage } from '@/pg-ui/utils/rbac';
import { useMemo } from 'react';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { HostListSchema, type HostRecord } from '@/schemas/api/host';
import { keys } from '@/api/queryKeys';

export type { HostRecord };

async function fetchHosts(): Promise<HostRecord[]> {
  const msg = await HttpUtil.get('/panel/api/hosts/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch hosts');
  const validated = parseMsg(msg, HostListSchema, 'hosts/list');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useHostsQuery() {
  const { admin } = useAdmin();
  // null while the current account is still loading; true/false once known.
  const permitted = admin ? canReadResourcePage(admin, 'hosts') : null;

  const query = useQuery({
    queryKey: keys.hosts.list(),
    queryFn: fetchHosts,
    enabled: permitted === true,
  });

  const hosts = useMemo(() => query.data ?? [], [query.data]);

  return {
    hosts,
    loading: query.isFetching,
    // A role without hosts access has nothing to wait for, so the query counts
    // as settled instead of leaving the page spinning forever.
    fetched: permitted === false ? true : query.data !== undefined || query.isError,
    fetchError: permitted === false || !query.error ? '' : (query.error as Error).message,
    permitted: permitted !== false,
    refetch: query.refetch,
  };
}
