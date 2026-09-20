import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Checkbox,
  Collapse,
  Form,
  Input,
  InputNumber,
  Layout,
  Modal,
  Popconfirm,
  Radio,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { CopyOutlined, PlusOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { NamePath } from 'antd/es/form/interface';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
import { setMessageInstance } from '@/utils/messageBus';
import { useAdminRoleMutations, useAdminRolesQuery } from '@/api/queries/useAdmins';
import type { AdminRoleDoc, PermissionScope } from '@/lib/rbac';
import { FEATURE_KEYS, LIMIT_KEYS, PERMISSION_GROUPS } from '@/lib/rbac';

/** Permission values are `true`/`false` for plain actions, 0/1/2 for scoped ones. */
type PermissionFormValue = boolean | PermissionScope;
type PermissionFormMap = Record<string, Record<string, PermissionFormValue | undefined>>;

interface RoleFormValues {
  name: string;
  permissions: PermissionFormMap;
  limits: Record<string, number | null>;
  features: Record<string, boolean>;
  access: {
    allowAllGroups: boolean;
    allowedGroups: string;
    allowAllInbounds: boolean;
    allowedInboundIds: string;
  };
}

const DEFAULT_ACCESS = {
  allowAllGroups: true,
  allowedGroups: '',
  allowAllInbounds: true,
  allowedInboundIds: '',
};

function toPermissionMap(raw: unknown): PermissionFormMap {
  if (!raw || typeof raw !== 'object') return {};
  const out: PermissionFormMap = {};
  for (const [resource, actions] of Object.entries(raw as Record<string, unknown>)) {
    if (!actions || typeof actions !== 'object') continue;
    const section: Record<string, PermissionFormValue | undefined> = {};
    for (const [action, value] of Object.entries(actions as Record<string, unknown>)) {
      if (typeof value === 'boolean') {
        section[action] = value;
      } else if (value && typeof value === 'object') {
        const scope = (value as { scope?: unknown }).scope;
        if (scope === 0 || scope === 1 || scope === 2) section[action] = scope;
      } else if (typeof value === 'string') {
        const lowered = value.trim().toLowerCase();
        if (lowered === 'own') section[action] = 1;
        else if (lowered === 'all') section[action] = 2;
        else if (lowered === 'true') section[action] = true;
      }
    }
    out[resource] = section;
  }
  return out;
}

export default function AdminRolesPage() {
  const { t } = useTranslation();
  const { isDark } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const { roles, loading, fetchError, refetch } = useAdminRolesQuery();
  const { create, update, duplicate, remove } = useAdminRoleMutations();

  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<AdminRoleDoc | null>(null);
  const [form] = Form.useForm<RoleFormValues>();

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({
      name: '',
      permissions: {},
      limits: {},
      features: {},
      access: DEFAULT_ACCESS,
    });
    setOpen(true);
  };

  const openEdit = (role: AdminRoleDoc) => {
    setEditing(role);
    const access = (role.access ?? {}) as Record<string, unknown>;
    const groups = Array.isArray(access.allowedGroups)
      ? (access.allowedGroups as string[]).join(', ')
      : '';
    const inbounds = Array.isArray(access.allowed_inbound_ids)
      ? (access.allowed_inbound_ids as number[]).join(', ')
      : '';
    form.resetFields();
    form.setFieldsValue({
      name: role.name,
      permissions: toPermissionMap(role.permissions),
      limits: (role.limits ?? {}) as Record<string, number | null>,
      features: (role.features ?? {}) as Record<string, boolean>,
      access: {
        allowAllGroups: access.allowAllGroups !== false,
        allowedGroups: groups,
        allowAllInbounds: access.allowAllInbounds !== false,
        allowedInboundIds: inbounds,
      },
    });
    setOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields();
    const groups = (values.access?.allowedGroups ?? '')
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    const inboundIds = (values.access?.allowedInboundIds ?? '')
      .split(',')
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n) && n > 0);

    const payload = {
      name: values.name,
      permissions: values.permissions ?? {},
      limits: values.limits ?? {},
      features: values.features ?? {},
      access: {
        allowAllGroups: values.access?.allowAllGroups ?? true,
        allowedGroups: groups,
        allowAllInbounds: values.access?.allowAllInbounds ?? true,
        allowed_inbound_ids: inboundIds,
      },
    };

    try {
      if (editing) {
        await update.mutateAsync({ id: editing.id, payload });
        messageApi.success(t('pages.adminRoles.toasts.updated'));
      } else {
        await create.mutateAsync(payload);
        messageApi.success(t('pages.adminRoles.toasts.created'));
      }
      setOpen(false);
      void refetch();
    } catch (e) {
      messageApi.error(String((e as Error).message ?? e));
    }
  };

  const columns: ColumnsType<AdminRoleDoc> = [
    {
      title: t('pages.adminRoles.columns.name'),
      dataIndex: 'name',
      key: 'name',
      render: (name: string, row) => (
        <Space>
          <span>{name}</span>
          {row.ownerRole && <Tag color="gold">{t('pages.adminRoles.owner')}</Tag>}
          {row.builtIn && !row.ownerRole && <Tag>{t('pages.adminRoles.builtIn')}</Tag>}
        </Space>
      ),
    },
    { title: t('pages.adminRoles.columns.slug'), dataIndex: 'slug', key: 'slug' },
    {
      title: t('pages.adminRoles.columns.admins'),
      dataIndex: 'adminCount',
      key: 'adminCount',
      width: 90,
    },
    {
      title: t('pages.adminRoles.columns.actions'),
      key: 'actions',
      render: (_: unknown, row) => (
        <Space>
          <Button size="small" onClick={() => openEdit(row)} disabled={row.ownerRole}>
            {t('edit')}
          </Button>
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={async () => {
              try {
                await duplicate.mutateAsync(row.id);
                void refetch();
              } catch (e) {
                messageApi.error(String((e as Error).message ?? e));
              }
            }}
          >
            {t('copy')}
          </Button>
          <Popconfirm
            title={t('pages.adminRoles.deleteConfirm')}
            onConfirm={async () => {
              try {
                await remove.mutateAsync(row.id);
                messageApi.success(t('pages.adminRoles.toasts.deleted'));
                void refetch();
              } catch (e) {
                messageApi.error(String((e as Error).message ?? e));
              }
            }}
            disabled={row.builtIn || row.ownerRole || row.adminCount > 0}
          >
            <Button
              size="small"
              danger
              disabled={row.builtIn || row.ownerRole || row.adminCount > 0}
            >
              {t('delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // One collapsible panel per resource group; forceRender keeps every field
  // registered so an untouched panel still submits its (empty) values.
  const permissionSections = useMemo(
    () =>
      PERMISSION_GROUPS.map((group) => ({
        key: group.labelKey,
        forceRender: true,
        label: t(`pages.adminRoles.groups.${group.labelKey}`, group.labelKey),
        children: (
          <Space direction="vertical" style={{ width: '100%' }}>
            {group.actions.map((item) => {
              const path: NamePath = ['permissions', item.resource, item.action];
              return (
                <div
                  key={`${item.resource}.${item.action}`}
                  style={{ display: 'flex', alignItems: 'center', gap: 12 }}
                >
                  <span style={{ flex: 1 }}>
                    {item.resource}.{item.action}
                  </span>
                  {item.scoped ? (
                    <Form.Item name={path} noStyle>
                      <Radio.Group size="small">
                        <Radio.Button value={0}>{t('none')}</Radio.Button>
                        <Radio.Button value={1}>{t('pages.adminRoles.scopeOwn')}</Radio.Button>
                        <Radio.Button value={2}>{t('pages.adminRoles.scopeAll')}</Radio.Button>
                      </Radio.Group>
                    </Form.Item>
                  ) : (
                    <Form.Item name={path} valuePropName="checked" noStyle>
                      <Checkbox />
                    </Form.Item>
                  )}
                </div>
              );
            })}
          </Space>
        ),
      })),
    [t],
  );

  return (
    <>
      {messageContextHolder}
      <Layout className={isDark ? 'dark' : undefined}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={loading} delay={200} description={t('loading')} size="large">
              <Typography.Title level={4}>{t('pages.adminRoles.title')}</Typography.Title>
              <Typography.Paragraph type="secondary">
                {t('pages.adminRoles.subtitle')}
              </Typography.Paragraph>

              <Card
                size="small"
                title={t('pages.adminRoles.listTitle')}
                extra={
                  <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                    {t('pages.adminRoles.create')}
                  </Button>
                }
              >
                <Table<AdminRoleDoc>
                  rowKey="id"
                  size="small"
                  loading={loading}
                  columns={columns}
                  dataSource={roles}
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
        width={760}
        title={editing ? t('pages.adminRoles.edit') : t('pages.adminRoles.create')}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={create.isPending || update.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label={t('pages.adminRoles.columns.name')}
            rules={[{ required: true, message: t('pages.adminRoles.nameRequired') }]}
          >
            <Input disabled={!!editing?.builtIn || !!editing?.ownerRole} />
          </Form.Item>

          <Collapse
            items={[
              {
                key: 'permissions',
                forceRender: true,
                label: t('pages.adminRoles.permissions'),
                children: <Collapse ghost items={permissionSections} />,
              },
              {
                key: 'limits',
                forceRender: true,
                label: t('pages.adminRoles.limits'),
                children: (
                  <Space wrap>
                    {LIMIT_KEYS.map((key) => (
                      <Form.Item
                        key={key}
                        name={['limits', key]}
                        label={key}
                        style={{ marginBottom: 8 }}
                      >
                        <InputNumber min={0} style={{ width: 140 }} />
                      </Form.Item>
                    ))}
                  </Space>
                ),
              },
              {
                key: 'features',
                forceRender: true,
                label: t('pages.adminRoles.features'),
                children: (
                  <Space direction="vertical">
                    {FEATURE_KEYS.map((key) => (
                      <Form.Item key={key} name={['features', key]} valuePropName="checked" noStyle>
                        <Checkbox>{key}</Checkbox>
                      </Form.Item>
                    ))}
                  </Space>
                ),
              },
              {
                key: 'access',
                forceRender: true,
                label: t('pages.adminRoles.access'),
                children: (
                  <>
                    <Form.Item name={['access', 'allowAllGroups']} valuePropName="checked">
                      <Checkbox>{t('pages.adminRoles.allowAllGroups')}</Checkbox>
                    </Form.Item>
                    <Form.Item
                      noStyle
                      shouldUpdate={(a: RoleFormValues, b: RoleFormValues) =>
                        a?.access?.allowAllGroups !== b?.access?.allowAllGroups
                      }
                    >
                      {({ getFieldValue }) =>
                        !getFieldValue(['access', 'allowAllGroups']) && (
                          <Form.Item
                            name={['access', 'allowedGroups']}
                            label={t('pages.adminRoles.allowedGroups')}
                            extra={t('pages.adminRoles.commaHint')}
                          >
                            <Input placeholder="vip, free" />
                          </Form.Item>
                        )
                      }
                    </Form.Item>

                    <Form.Item name={['access', 'allowAllInbounds']} valuePropName="checked">
                      <Checkbox>{t('pages.adminRoles.allowAllInbounds')}</Checkbox>
                    </Form.Item>
                    <Form.Item
                      noStyle
                      shouldUpdate={(a: RoleFormValues, b: RoleFormValues) =>
                        a?.access?.allowAllInbounds !== b?.access?.allowAllInbounds
                      }
                    >
                      {({ getFieldValue }) =>
                        !getFieldValue(['access', 'allowAllInbounds']) && (
                          <Form.Item
                            name={['access', 'allowedInboundIds']}
                            label={t('pages.adminRoles.allowedInbounds')}
                            extra={t('pages.adminRoles.commaHint')}
                          >
                            <Input placeholder="1, 2, 3" />
                          </Form.Item>
                        )
                      }
                    </Form.Item>
                  </>
                ),
              },
            ]}
          />
        </Form>
      </Modal>
    </>
  );
}
