import { useMutation, useQueryClient } from '@tanstack/react-query';
import { message } from 'antd';
import type { MessageInstance } from 'antd/es/message/interface';
import { useTranslation } from 'react-i18next';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import type { BenchmarkReport } from './useHashemQuery';

export interface HashemSetupPayload {
  role?: string;
  engine?: 'tunnel-panel' | 'frp' | 'backhaul' | 'gre-backhaul';
  transport?: 'tcpmux' | 'wssmux' | 'tcpo';
  backhaulPort?: number;
  snappy?: boolean;
  localPub?: string;
  remotePub: string;
  frpPort?: number;
  token?: string;
  ports?: string;
  carrier?: string;
  bundle?: string;
  autoCreateInbound?: boolean;
  inboundHost?: string;
}

export interface HashemSSHSetupPayload {
  engine?: 'tunnel-panel' | 'frp' | 'backhaul' | 'gre-backhaul';
  transport?: 'tcpmux' | 'wssmux' | 'tcpo';
  backhaulPort?: number;
  snappy?: boolean;
  iranIp: string;
  sshPort?: number;
  sshUser?: string;
  sshPassword: string;
  ports?: string;
  carrier?: string;
  autoCreateInbound?: boolean;
  inboundHost?: string;
}

export interface HashemSSHSetupResult {
  success: boolean;
  message: string;
  iranIp: string;
  foreignIp: string;
  ports: string;
  log: string;
}

export interface HashemOneLinerPayload {
  engine?: 'tunnel-panel' | 'frp' | 'backhaul' | 'gre-backhaul';
  transport?: 'tcpmux' | 'wssmux' | 'tcpo';
  backhaulPort?: number;
  snappy?: boolean;
  iranIp: string;
  ports?: string;
  carrier?: string;
  autoCreateInbound?: boolean;
  inboundHost?: string;
}

export interface HashemOneLinerResult {
  oneLinerCommand: string;
  foreignIp: string;
  iranIp: string;
  ports: string;
  engine?: string;
  transport?: string;
  frpPort: number;
  backhaulPort?: number;
  token: string;
}

export interface HashemPortActionResult {
  port: number;
  action: 'add' | 'remove';
  ports: number[];
  oneLinerCommand: string;
  message: string;
}

export function useHashemMutations(customMessageApi?: MessageInstance) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [internalMessageApi] = message.useMessage();
  const messageApi = customMessageApi || internalMessageApi;

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: keys.hashem.status() });
    queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
    queryClient.invalidateQueries({ queryKey: keys.clients.root() });
  };

  const setCarrierMutation = useMutation({
    mutationFn: async (carrier: string) => {
      const res = await HttpUtil.post('/panel/api/hashem/carrier', { carrier });
      if (!res?.success) throw new Error(res?.msg || 'Failed to switch carrier');
      return res;
    },
    onSuccess: () => {
      messageApi.success(t('pages.hashem.toasts.carrierSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const restartMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post('/panel/api/hashem/restart');
      if (!res?.success) throw new Error(res?.msg || 'Failed to restart tunnel');
      return res;
    },
    onSuccess: () => {
      messageApi.success(t('pages.hashem.toasts.restartSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const setWatchdogMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const res = await HttpUtil.post('/panel/api/hashem/watchdog', { enabled });
      if (!res?.success) throw new Error(res?.msg || 'Failed to update watchdog');
      return res;
    },
    onSuccess: () => {
      messageApi.success(t('pages.hashem.toasts.watchdogSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const syncInboundsMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post<number[]>('/panel/api/hashem/sync-inbounds');
      if (!res?.success) throw new Error(res?.msg || 'Failed to sync inbounds');
      return res.obj;
    },
    onSuccess: (ports) => {
      messageApi.success(t('pages.hashem.toasts.syncSuccess', { count: ports?.length || 0 }));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const editPortsMutation = useMutation({
    mutationFn: async (ports: number[]) => {
      const res = await HttpUtil.post(
        '/panel/api/hashem/edit-ports',
        { ports },
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!res?.success) throw new Error(res?.msg || 'Failed to update ports');
      return res;
    },
    onSuccess: () => {
      messageApi.success(
        t('pages.hashem.toasts.editPortsSuccess', {
          defaultValue: 'Tunnel ports updated successfully',
        }),
      );
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const setupMutation = useMutation({
    mutationFn: async (payload: HashemSetupPayload) => {
      const res = await HttpUtil.post<string>('/panel/api/hashem/setup', payload);
      if (!res?.success) throw new Error(res?.msg || 'Setup failed');
      return res.obj;
    },
    onSuccess: () => {
      messageApi.success(t('pages.hashem.toasts.setupSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const setupSSHMutation = useMutation({
    mutationFn: async (payload: HashemSSHSetupPayload) => {
      const res = await HttpUtil.post<HashemSSHSetupResult>(
        '/panel/api/hashem/setup-ssh',
        payload,
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!res?.success) throw new Error(res?.msg || 'SSH Setup failed');
      return res.obj;
    },
    onSuccess: (data) => {
      messageApi.success(data?.message || t('pages.hashem.toasts.setupSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const generateOneLinerMutation = useMutation({
    mutationFn: async (payload: HashemOneLinerPayload) => {
      const res = await HttpUtil.post<HashemOneLinerResult>(
        '/panel/api/hashem/generate-oneliner',
        payload,
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!res?.success) throw new Error(res?.msg || 'Generating one-liner failed');
      return res.obj;
    },
    onSuccess: () => {
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const installMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post<string>('/panel/api/hashem/install');
      if (!res?.success) throw new Error(res?.msg || 'Install failed');
      return res.obj;
    },
    onSuccess: () => {
      messageApi.success(t('pages.hashem.toasts.installSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const removeMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post('/panel/api/hashem/remove');
      if (!res?.success) throw new Error(res?.msg || 'Remove failed');
      return res;
    },
    onSuccess: () => {
      messageApi.success(t('pages.hashem.toasts.removeSuccess'));
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const runBenchmarkMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post<BenchmarkReport>('/panel/api/hashem/benchmark/run');
      if (!res?.success) throw new Error(res?.msg || 'Benchmark failed');
      return res.obj;
    },
    onSuccess: () => {
      messageApi.success(
        t('pages.hashem.toasts.benchmarkSuccess', {
          defaultValue: 'Carrier benchmark completed successfully',
        }),
      );
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const setAutoPilotMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const res = await HttpUtil.post('/panel/api/hashem/benchmark/autopilot', { enabled });
      if (!res?.success) throw new Error(res?.msg || 'Failed to update autopilot');
      return res;
    },
    onSuccess: () => {
      messageApi.success(
        t('pages.hashem.toasts.autoPilotSuccess', {
          defaultValue: 'Carrier auto-pilot status updated',
        }),
      );
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const autoCreateInboundMutation = useMutation({
    mutationFn: async (payload: { ports?: string; host?: string }) => {
      return HttpUtil.post('/panel/api/hashem/auto-create-inbound', payload, {
        headers: { 'Content-Type': 'application/json' },
      });
    },
    onSuccess: () => {
      messageApi.success(
        t('pages.hashem.toasts.inboundCreated', {
          defaultValue: 'Matching inbounds created successfully.',
        }),
      );
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const addPortMutation = useMutation({
    mutationFn: async (payload: { port: number; autoInbound?: boolean }) => {
      const res = await HttpUtil.post<HashemPortActionResult>(
        '/panel/api/hashem/add-port',
        payload,
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!res?.success) throw new Error(res?.msg || 'Failed to add port');
      return res.obj;
    },
    onSuccess: () => {
      messageApi.success(
        t('pages.hashem.toasts.addPortSuccess', {
          defaultValue: 'Port added to tunnel successfully',
        }),
      );
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  const removePortMutation = useMutation({
    mutationFn: async (payload: { port: number }) => {
      const res = await HttpUtil.post<HashemPortActionResult>(
        '/panel/api/hashem/remove-port',
        payload,
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!res?.success) throw new Error(res?.msg || 'Failed to remove port');
      return res.obj;
    },
    onSuccess: () => {
      messageApi.success(
        t('pages.hashem.toasts.removePortSuccess', {
          defaultValue: 'Port removed from tunnel successfully',
        }),
      );
      invalidate();
    },
    onError: (err: Error) => messageApi.error(err.message),
  });

  return {
    setCarrier: setCarrierMutation.mutateAsync,
    isSettingCarrier: setCarrierMutation.isPending,
    restart: restartMutation.mutateAsync,
    isRestarting: restartMutation.isPending,
    setWatchdog: setWatchdogMutation.mutateAsync,
    isSettingWatchdog: setWatchdogMutation.isPending,
    syncInbounds: syncInboundsMutation.mutateAsync,
    isSyncingInbounds: syncInboundsMutation.isPending,
    editPorts: editPortsMutation.mutateAsync,
    isEditingPorts: editPortsMutation.isPending,
    addPort: addPortMutation.mutateAsync,
    isAddingPort: addPortMutation.isPending,
    removePort: removePortMutation.mutateAsync,
    isRemovingPort: removePortMutation.isPending,
    setup: setupMutation.mutateAsync,
    isSettingUp: setupMutation.isPending,
    setupSSH: setupSSHMutation.mutateAsync,
    isSettingUpSSH: setupSSHMutation.isPending,
    generateOneLiner: generateOneLinerMutation.mutateAsync,
    isGeneratingOneLiner: generateOneLinerMutation.isPending,
    install: installMutation.mutateAsync,
    isInstalling: installMutation.isPending,
    removeTunnel: removeMutation.mutateAsync,
    isRemovingTunnel: removeMutation.isPending,
    runBenchmark: runBenchmarkMutation.mutateAsync,
    isRunningBenchmark: runBenchmarkMutation.isPending,
    setAutoPilot: setAutoPilotMutation.mutateAsync,
    isSettingAutoPilot: setAutoPilotMutation.isPending,
    autoCreateInbound: autoCreateInboundMutation.mutateAsync,
    isAutoCreatingInbound: autoCreateInboundMutation.isPending,
  };
}
