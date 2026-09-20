import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Form, Input, Modal, Select, Switch } from 'antd';

import type { AdminAccount, AdminRoleDoc } from '@/api/queries/useAdmins';

export interface AdminFormValues {
  username: string;
  password?: string;
  roleId: number;
  status: boolean;
}

interface AdminFormModalProps {
  open: boolean;
  mode: 'create' | 'edit';
  admin: AdminAccount | null;
  roles: AdminRoleDoc[];
  saving?: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: AdminFormValues) => void | Promise<void>;
}

export default function AdminFormModal({
  open,
  mode,
  admin,
  roles,
  saving,
  onOpenChange,
  onSubmit,
}: AdminFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<AdminFormValues>();

  // The owner account keeps its role and cannot be disabled; a non-owner can be
  // moved between roles freely.
  const lockedRole = mode === 'edit' && !!admin?.ownerRole;

  const roleOptions = useMemo(
    () =>
      roles.filter((role) => !role.ownerRole).map((role) => ({ value: role.id, label: role.name })),
    [roles],
  );

  useEffect(() => {
    if (!open) return;
    if (mode === 'edit' && admin) {
      form.setFieldsValue({
        username: admin.username,
        password: '',
        roleId: admin.roleId,
        status: admin.status === 'active',
      });
    } else {
      form.setFieldsValue({
        username: '',
        password: '',
        roleId: roleOptions.length ? roleOptions[0].value : undefined,
        status: true,
      });
    }
  }, [open, mode, admin, roleOptions, form]);

  const handleOk = async () => {
    const values = await form.validateFields();
    await onSubmit(values);
  };

  return (
    <Modal
      open={open}
      title={mode === 'create' ? t('pages.admins.create') : t('pages.admins.edit')}
      width={520}
      okText={t('save')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      onOk={handleOk}
      onCancel={() => onOpenChange(false)}
      destroyOnHidden
      maskClosable={false}
    >
      <Form form={form} layout="vertical" autoComplete="off">
        {mode === 'edit' && admin?.roleName && (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message={`${t('pages.admins.columns.role')}: ${admin.roleName}`}
            description={admin.ownerRole ? t('pages.admins.ownerHint') : undefined}
          />
        )}

        <Form.Item
          name="username"
          label={t('username')}
          rules={[{ required: true, message: t('pages.admins.usernameRequired') }]}
        >
          <Input autoComplete="off" />
        </Form.Item>

        <Form.Item
          name="password"
          label={t('password')}
          rules={
            mode === 'create'
              ? [{ required: true, message: t('pages.admins.passwordRequired') }]
              : []
          }
          extra={mode === 'edit' ? t('pages.admins.passwordKeepHint') : undefined}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>

        <Form.Item
          name="roleId"
          label={t('pages.admins.columns.role')}
          rules={[{ required: true, message: t('pages.admins.roleRequired') }]}
        >
          <Select options={roleOptions} disabled={lockedRole} />
        </Form.Item>

        <Form.Item
          name="status"
          label={t('pages.admins.columns.status')}
          valuePropName="checked"
          tooltip={t('pages.admins.statusHint')}
        >
          <Switch disabled={mode === 'edit' && !!admin?.ownerRole} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
