import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

export interface CarrierMetric {
  id: string;
  name: string;
  type: string;
  port: number;
  avgRttMs: number;
  minRttMs: number;
  maxRttMs: number;
  packetLoss: number;
  jitterMs: number;
  score: number;
  status: 'healthy' | 'good' | 'warning' | 'critical' | 'down';
  isActive: boolean;
  isRecommended: boolean;
  errorDetail?: string;
}

export interface AutoPilotStatus {
  enabled: boolean;
  thresholdLoss?: number;
  lastTriggered?: string;
  triggerCount?: number;
}

export interface BenchmarkReport {
  timestamp: string;
  durationSec: number;
  peerUrl?: string;
  peerInternalIp?: string;
  activeCarrier: string;
  bestCarrier: string;
  autoPilot: AutoPilotStatus;
  metrics: CarrierMetric[];
}

export interface HashemStatus {
  installed: boolean;
  running: boolean;
  role: 'foreign' | 'iran' | 'none';
  engine?: 'frp' | 'backhaul' | 'gre-backhaul';
  transport?: 'tcpmux' | 'wssmux' | 'tcpo';
  backhaulPort?: number;
  snappy?: boolean;
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
  autoPilot?: boolean;
  benchmark?: BenchmarkReport;
  bundle: string;
  setupCommand: string;
}

export const DEFAULT_HASHEM_STATUS: HashemStatus = {
  installed: false,
  running: false,
  role: 'none',
  engine: 'frp',
  transport: 'tcpmux',
  backhaulPort: 3080,
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
  autoPilot: false,
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
