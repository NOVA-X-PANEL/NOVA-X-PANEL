import { useMutation, useQueryClient } from '@tanstack/react-query';
import { message } from 'antd';
import { useTranslation } from 'react-i18next';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

export interface HashemSetupPayload {
  role?: string;
  localPub?: string;
  remotePub: string;
  frpPort?: number;
  token?: string;
  ports?: string;
  carrier?: string;
  bundle?: string;
}

export interface HashemSSHSetupPayload {
  iranIp: string;
  sshPort?: number;
  sshUser?: string;
  sshPassword: string;
  ports?: string;
  carrier?: string;
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
  iranIp: string;
  ports?: string;
  carrier?: string;
}

export interface HashemOneLinerResult {
  oneLinerCommand: string;
  foreignIp: string;
  iranIp: string;
  ports: string;
  frpPort: number;
  token: string;
}

export function useHashemMutations() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.hashem.status() });

  const setCarrierMutation = useMutation({
    mutationFn: async (carrier: string) => {
      const res = await HttpUtil.post('/panel/api/hashem/carrier', { carrier });
      if (!res?.success) throw new Error(res?.msg || 'Failed to switch carrier');
      return res;
    },
    onSuccess: () => {
      message.success(t('pages.hashem.toasts.carrierSuccess'));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const restartMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post('/panel/api/hashem/restart');
      if (!res?.success) throw new Error(res?.msg || 'Failed to restart tunnel');
      return res;
    },
    onSuccess: () => {
      message.success(t('pages.hashem.toasts.restartSuccess'));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const setWatchdogMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const res = await HttpUtil.post('/panel/api/hashem/watchdog', { enabled });
      if (!res?.success) throw new Error(res?.msg || 'Failed to update watchdog');
      return res;
    },
    onSuccess: () => {
      message.success(t('pages.hashem.toasts.watchdogSuccess'));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const syncInboundsMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post<number[]>('/panel/api/hashem/sync-inbounds');
      if (!res?.success) throw new Error(res?.msg || 'Failed to sync inbounds');
      return res.obj;
    },
    onSuccess: (ports) => {
      message.success(t('pages.hashem.toasts.syncSuccess', { count: ports?.length || 0 }));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const setupMutation = useMutation({
    mutationFn: async (payload: HashemSetupPayload) => {
      const res = await HttpUtil.post<string>('/panel/api/hashem/setup', payload);
      if (!res?.success) throw new Error(res?.msg || 'Setup failed');
      return res.obj;
    },
    onSuccess: () => {
      message.success(t('pages.hashem.toasts.setupSuccess'));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
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
      message.success(data?.message || t('pages.hashem.toasts.setupSuccess'));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
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
    onError: (err: Error) => message.error(err.message),
  });

  const installMutation = useMutation({
    mutationFn: async () => {
      const res = await HttpUtil.post<string>('/panel/api/hashem/install');
      if (!res?.success) throw new Error(res?.msg || 'Install failed');
      return res.obj;
    },
    onSuccess: () => {
      message.success(t('pages.hashem.toasts.installSuccess'));
      invalidate();
    },
    onError: (err: Error) => message.error(err.message),
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
    setup: setupMutation.mutateAsync,
    isSettingUp: setupMutation.isPending,
    setupSSH: setupSSHMutation.mutateAsync,
    isSettingUpSSH: setupSSHMutation.isPending,
    generateOneLiner: generateOneLinerMutation.mutateAsync,
    isGeneratingOneLiner: generateOneLinerMutation.isPending,
    install: installMutation.mutateAsync,
    isInstalling: installMutation.isPending,
  };
}
