import { useState } from 'react';
import { Alert, Modal, Select, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

import { useGetAdminsSimple } from '@/pg-ui/service/api';
import type { AdminSimple } from '@/pg-ui/service/api';
import { isOwner } from '@/pg-ui/utils/rbac';

interface ClientTransferModalProps {
  open: boolean;
  email: string;
  currentOwnerId?: number;
  loading?: boolean;
  onCancel: () => void;
  onSubmit: (adminId: number) => Promise<void> | void;
}

export default function ClientTransferModal({
  open,
  email,
  currentOwnerId,
  loading = false,
  onCancel,
  onSubmit,
}: ClientTransferModalProps) {
  const { t } = useTranslation();
  const [targetAdminId, setTargetAdminId] = useState<number>();
  const { data, isLoading, error } = useGetAdminsSimple(
    { all: true },
    { query: { enabled: open, staleTime: 5 * 60 * 1000 } },
  );

  const admins = ((data?.admins || []) as AdminSimple[]).filter((admin) => {
    const status = String(admin.status || '').toLowerCase();
    return admin.id !== currentOwnerId && status !== 'disabled' && !isOwner(admin);
  });

  return (
    <Modal
      title={t('pages.clients.transferToAdmin')}
      open={open}
      onCancel={onCancel}
      onOk={() => {
        if (targetAdminId) void onSubmit(targetAdminId);
      }}
      okText={t('confirm')}
      cancelText={t('cancel')}
      confirmLoading={loading}
      okButtonProps={{ disabled: !targetAdminId || isLoading || !!error }}
      destroyOnHidden
    >
      <Typography.Paragraph>
        {t('pages.clients.transferAdminDescription', { email })}
      </Typography.Paragraph>
      {error && (
        <Alert type="error" showIcon message={t('pages.clients.transferAdminsLoadFailed')} />
      )}
      {!isLoading && !error && admins.length === 0 && (
        <Alert type="warning" showIcon message={t('pages.clients.transferNoAdmins')} />
      )}
      <Select
        showSearch
        optionFilterProp="label"
        loading={isLoading}
        disabled={isLoading || !!error || admins.length === 0 || loading}
        value={targetAdminId}
        onChange={setTargetAdminId}
        placeholder={t('pages.clients.transferTargetAdmin')}
        style={{ width: '100%' }}
        options={admins.map((admin) => ({
          value: admin.id,
          label: admin.username,
        }))}
      />
    </Modal>
  );
}
