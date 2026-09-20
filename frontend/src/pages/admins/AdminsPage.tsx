import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  Layout,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
import { setMessageInstance } from '@/utils/messageBus';
import {
  useAdminMutations,
  useAdminRolesQuery,
  useAdminStatsQuery,
  useAdminsQuery,
} from '@/api/queries/useAdmins';
import type { AdminAccount } from '@/api/queries/useAdmins';

interface AdminFormValues {
  username: string;
  password?: string;
  roleId: number;
  status: string;
}

export default function AdminsPage() {
  const { t } = useTranslation();
  const { isDark } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const { admins, loading, fetchError, refetch } = useAdminsQuery();
  const { stats } = useAdminStatsQuery();
  const { roles } = useAdminRolesQuery();
  const { create, update, remove, setStatus } = useAdminMutations();

  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<AdminAccount | null>(null);
  const [form] = Form.useForm<AdminFormValues>();

  const roleOptions = useMemo(
    () => roles.filter((r) => !r.ownerRole).map((r) => ({ value: r.id, label: r.name })),
    [roles],
  );

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ username: '', password: '', status: 'active' });
    setOpen(true);
  };

  const openEdit = (row: AdminAccount) => {
    setEditing(row);
    form.setFieldsValue({
      username: row.username,
      password: '',
      roleId: row.roleId,
      status: row.status,
    });
    setOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields();
    try {
      if (editing) {
        await update.mutateAsync({
          id: editing.id,
          payload: {
            username: values.username,
            password: values.password || undefined,
            roleId: values.roleId,
            status: values.status,
          },
        });
        messageApi.success(t('pages.admins.toasts.updated'));
      } else {
        await create.mutateAsync({
          username: values.username,
          password: values.password,
          roleId: values.roleId,
          status: values.status,
        });
        messageApi.success(t('pages.admins.toasts.created'));
      }
      setOpen(false);
      void refetch();
    } catch (e) {
      messageApi.error(String((e as Error).message ?? e));
    }
  };

  const columns: ColumnsType<AdminAccount> = [
    {
      title: t('pages.admins.columns.username'),
      dataIndex: 'username',
      key: 'username',
      render: (name: string, row) => (
        <Space>
          <span>{name}</span>
          {row.isSelf && <Tag color="blue">{t('pages.admins.you')}</Tag>}
        </Space>
      ),
    },
    {
      title: t('pages.admins.columns.role'),
      dataIndex: 'roleName',
      key: 'roleName',
      render: (name: string, row) => <Tag color={row.ownerRole ? 'gold' : 'default'}>{name}</Tag>,
    },
    {
      title: t('pages.admins.columns.status'),
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'active' ? 'green' : 'red'}>
          {status === 'active' ? t('enabled') : t('disabled')}
        </Tag>
      ),
    },
    {
      title: t('pages.admins.columns.actions'),
      key: 'actions',
      render: (_: unknown, row) => (
        <Space>
          <Button size="small" onClick={() => openEdit(row)}>
            {t('edit')}
          </Button>
          <Button
            size="small"
            disabled={row.isSelf || row.ownerRole}
            onClick={async () => {
              try {
                await setStatus.mutateAsync({ id: row.id, enabled: row.status !== 'active' });
                void refetch();
              } catch (e) {
                messageApi.error(String((e as Error).message ?? e));
              }
            }}
          >
            {row.status === 'active' ? t('pages.admins.disable') : t('pages.admins.enable')}
          </Button>
          <Popconfirm
            title={t('pages.admins.deleteConfirm')}
            onConfirm={async () => {
              try {
                await remove.mutateAsync(row.id);
                messageApi.success(t('pages.admins.toasts.deleted'));
                void refetch();
              } catch (e) {
                messageApi.error(String((e as Error).message ?? e));
              }
            }}
            disabled={row.isSelf || row.ownerRole}
          >
            <Button size="small" danger disabled={row.isSelf || row.ownerRole}>
              {t('delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      {messageContextHolder}
      <Layout className={isDark ? 'dark' : undefined}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={loading} delay={200} description={t('loading')} size="large">
              <Typography.Title level={4}>{t('pages.admins.title')}</Typography.Title>
              <Typography.Paragraph type="secondary">
                {t('pages.admins.description')}
              </Typography.Paragraph>

              <Row gutter={16} style={{ marginBottom: 16 }}>
                <Col span={8}>
                  <Card size="small">
                    <Statistic
                      title={t('pages.admins.stats.total')}
                      value={stats?.totalAdmins ?? admins.length}
                    />
                  </Card>
                </Col>
                <Col span={8}>
                  <Card size="small">
                    <Statistic
                      title={t('pages.admins.stats.active')}
                      value={stats?.activeAdmins ?? 0}
                    />
                  </Card>
                </Col>
                <Col span={8}>
                  <Card size="small">
                    <Statistic
                      title={t('pages.admins.stats.disabled')}
                      value={stats?.disabledAdmins ?? 0}
                    />
                  </Card>
                </Col>
              </Row>

              <Card
                size="small"
                title={t('pages.admins.listTitle')}
                extra={
                  <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                    {t('pages.admins.create')}
                  </Button>
                }
              >
                <Table<AdminAccount>
                  rowKey="id"
                  size="small"
                  loading={loading}
                  columns={columns}
                  dataSource={admins}
                  pagination={false}
                  locale={{ emptyText: fetchError || t('noData') }}
                />
              </Card>
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>

      <Modal
        open={open}
        title={editing ? t('pages.admins.edit') : t('pages.admins.create')}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={create.isPending || update.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ status: 'active' }}>
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
            rules={editing ? [] : [{ required: true, message: t('pages.admins.passwordRequired') }]}
            extra={editing ? t('pages.admins.passwordKeepHint') : undefined}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="roleId"
            label={t('pages.admins.columns.role')}
            rules={[{ required: true, message: t('pages.admins.roleRequired') }]}
          >
            <Select options={roleOptions} disabled={!!editing?.ownerRole} />
          </Form.Item>
          <Form.Item name="status" label={t('status')}>
            <Select
              disabled={!!editing?.ownerRole}
              options={[
                { value: 'active', label: t('enabled') },
                { value: 'disabled', label: t('disabled') },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
