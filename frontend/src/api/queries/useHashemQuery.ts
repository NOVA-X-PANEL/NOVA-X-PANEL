import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

export interface HashemStatus {
  installed: boolean;
  running: boolean;
  role: 'foreign' | 'iran' | 'none';
  carrier: string;
  activeCarrier: string;
  candidates: string[];
  localPubIp: string;
  remotePubIp: string;
  localGreIp: string;
  remoteGreIp: string;
  frpStatus: string;
  frpPort: number;
  pingMs: number;
  ports: number[];
  watchdogEnabled: boolean;
  bundle: string;
  setupCommand: string;
}

export const DEFAULT_HASHEM_STATUS: HashemStatus = {
  installed: false,
  running: false,
  role: 'none',
  carrier: 'direct',
  activeCarrier: 'direct',
  candidates: ['direct', 'fou:443', 'wss:8443'],
  localPubIp: '',
  remotePubIp: '',
  localGreIp: '',
  remoteGreIp: '',
  frpStatus: 'inactive',
  frpPort: 0,
  pingMs: -1,
  ports: [],
  watchdogEnabled: false,
  bundle: '',
  setupCommand: '',
};

async function fetchHashemStatus(): Promise<HashemStatus> {
  const msg = await HttpUtil.get<HashemStatus>('/panel/api/hashem/status', undefined, {
    silent: true,
  });
  if (!msg?.success || !msg.obj) return DEFAULT_HASHEM_STATUS;
  return { ...DEFAULT_HASHEM_STATUS, ...msg.obj };
}

export function useHashemQuery() {
  const query = useQuery({
    queryKey: keys.hashem.status(),
    queryFn: fetchHashemStatus,
    refetchInterval: 5000,
  });

  return {
    status: query.data ?? DEFAULT_HASHEM_STATUS,
    loading: query.isLoading,
    refetch: query.refetch,
  };
}
