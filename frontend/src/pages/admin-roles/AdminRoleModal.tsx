import { useCallback, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Checkbox,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Space,
  Tabs,
  Typography,
} from 'antd';
import { CrownOutlined, LockOutlined } from '@ant-design/icons';

import type { AdminRoleDoc } from '@/api/queries/useAdmins';
import type { PermissionScope } from '@/lib/rbac';
import { FEATURE_KEYS, LIMIT_KEYS, PERMISSION_GROUPS } from '@/lib/rbac';

/** `true`/`false` for plain actions; 0/1/2 for scoped ones. */
type PermissionFormValue = boolean | PermissionScope;
type PermissionFormMap = Record<string, Record<string, PermissionFormValue | undefined>>;

export interface AdminRoleFormValues {
  name: string;
  permissions: PermissionFormMap;
  limits: Record<string, number | null | undefined>;
  features: Record<string, boolean | undefined>;
  access: {
    allowAllGroups: boolean;
    allowedGroups: string;
    allowAllInbounds: boolean;
    allowedInboundIds: string;
  };
}

interface AdminRoleModalProps {
  open: boolean;
  mode: 'create' | 'edit';
  role: AdminRoleDoc | null;
  saving?: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: AdminRoleFormValues) => void | Promise<void>;
}

const DEFAULT_ACCESS = {
  allowAllGroups: true,
  allowedGroups: '',
  allowAllInbounds: true,
  allowedInboundIds: '',
};

/** Normalises the stored permission document into flat form values. */
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

function toList(value: unknown): string[] {
  return Array.isArray(value) ? (value as unknown[]).map(String) : [];
}

export default function AdminRoleModal({
  open,
  mode,
  role,
  saving,
  onOpenChange,
  onSubmit,
}: AdminRoleModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<AdminRoleFormValues>();

  const lockedIdentity = mode === 'edit' && !!role?.builtIn;

  useEffect(() => {
    if (!open) return;
    if (mode === 'edit' && role) {
      const access = (role.access ?? {}) as Record<string, unknown>;
      form.setFieldsValue({
        name: role.name,
        permissions: toPermissionMap(role.permissions),
        limits: (role.limits ?? {}) as Record<string, number | null>,
        features: (role.features ?? {}) as Record<string, boolean>,
        access: {
          allowAllGroups: access.allowAllGroups !== false,
          allowedGroups: toList(access.allowedGroups).join(', '),
          allowAllInbounds: access.allowAllInbounds !== false,
          allowedInboundIds: toList(access.allowed_inbound_ids ?? access.allowedInboundIds).join(
            ', ',
          ),
        },
      });
    } else {
      form.resetFields();
      form.setFieldsValue({
        name: '',
        permissions: {},
        limits: {},
        features: {},
        access: DEFAULT_ACCESS,
      });
    }
  }, [open, mode, role, form]);

  const handleOk = async () => {
    const values = await form.validateFields();
    await onSubmit(values);
  };

  // Group-level bulk toggles keep the large permission grid manageable.
  const applyGroup = useCallback(
    (resource: string, actions: { action: string; scoped?: boolean }[], enable: boolean) => {
      const current = (form.getFieldValue(['permissions', resource]) ?? {}) as Record<
        string,
        PermissionFormValue | undefined
      >;
      const next: Record<string, PermissionFormValue | undefined> = { ...current };
      for (const item of actions) {
        next[item.action] = enable ? (item.scoped ? 1 : true) : item.scoped ? 0 : false;
      }
      const permissions = (form.getFieldValue('permissions') ?? {}) as PermissionFormMap;
      form.setFieldValue('permissions', { ...permissions, [resource]: next });
    },
    [form],
  );

  const permissionPanel = useMemo(
    () => (
      <div className="permission-grid">
        {PERMISSION_GROUPS.map((group) => (
          <div className="permission-group" key={group.labelKey}>
            <div className="permission-group__head">
              <Typography.Text strong>
                {t(`pages.adminRoles.groups.${group.labelKey}`, group.labelKey)}
              </Typography.Text>
              <Space size={4}>
                <Typography.Link
                  onClick={() => applyGroup(group.actions[0].resource, group.actions, true)}
                >
                  {t('pages.adminRoles.selectAll')}
                </Typography.Link>
                <Typography.Text type="secondary">·</Typography.Text>
                <Typography.Link
                  onClick={() => applyGroup(group.actions[0].resource, group.actions, false)}
                >
                  {t('none')}
                </Typography.Link>
              </Space>
            </div>

            <div className="permission-rows">
              {group.actions.map((item) => {
                const path = ['permissions', item.resource, item.action];
                return (
                  <div className="permission-row" key={`${item.resource}.${item.action}`}>
                    <span className="permission-row__label">
                      {item.action}
                      <span className="permission-row__resource">{item.resource}</span>
                    </span>
                    {item.scoped ? (
                      <Form.Item name={path} noStyle>
                        <Radio.Group size="small" optionType="button" buttonStyle="solid">
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
            </div>
          </div>
        ))}
      </div>
    ),
    [t, applyGroup],
  );

  const limitsPanel = (
    <>
      <Typography.Paragraph type="secondary" style={{ marginTop: 0, fontSize: 13 }}>
        {t('pages.adminRoles.limitsHint')}
      </Typography.Paragraph>
      <div className="limits-grid">
        {LIMIT_KEYS.map((key) => (
          <Form.Item
            key={key}
            name={['limits', key]}
            label={t(`pages.adminRoles.limitFields.${key}`)}
          >
            <InputNumber min={0} style={{ width: '100%' }} placeholder="—" />
          </Form.Item>
        ))}
      </div>
    </>
  );

  const featuresPanel = (
    <div className="features-list">
      {FEATURE_KEYS.map((key) => (
        <Form.Item key={key} name={['features', key]} valuePropName="checked" noStyle>
          <Checkbox>
            <span className="feature-label">{t(`pages.adminRoles.featureLabels.${key}`)}</span>
            <span className="feature-key">{key}</span>
          </Checkbox>
        </Form.Item>
      ))}
    </div>
  );

  const accessPanel = (
    <>
      <Form.Item name={['access', 'allowAllGroups']} valuePropName="checked">
        <Checkbox>{t('pages.adminRoles.allowAllGroups')}</Checkbox>
      </Form.Item>
      <Form.Item
        noStyle
        shouldUpdate={(a: AdminRoleFormValues, b: AdminRoleFormValues) =>
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

      <Divider style={{ margin: '4px 0 16px' }} />

      <Form.Item name={['access', 'allowAllInbounds']} valuePropName="checked">
        <Checkbox>{t('pages.adminRoles.allowAllInbounds')}</Checkbox>
      </Form.Item>
      <Form.Item
        noStyle
        shouldUpdate={(a: AdminRoleFormValues, b: AdminRoleFormValues) =>
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
  );

  return (
    <Modal
      open={open}
      title={mode === 'create' ? t('pages.adminRoles.create') : t('pages.adminRoles.edit')}
      width={720}
      okText={t('save')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      onOk={handleOk}
      onCancel={() => onOpenChange(false)}
      destroyOnHidden
      maskClosable={false}
      className="admin-role-modal"
    >
      <Form form={form} layout="vertical">
        {mode === 'edit' && role && (
          <Alert
            type={role.ownerRole ? 'warning' : 'info'}
            showIcon
            icon={role.ownerRole ? <CrownOutlined /> : <LockOutlined />}
            style={{ marginBottom: 12 }}
            message={
              role.ownerRole
                ? t('pages.adminRoles.ownerReadOnly')
                : role.builtIn
                  ? t('pages.adminRoles.builtInHint')
                  : role.name
            }
          />
        )}

        <Form.Item
          name="name"
          label={t('pages.adminRoles.columns.name')}
          rules={[{ required: true, message: t('pages.adminRoles.nameRequired') }]}
          extra={lockedIdentity ? t('pages.adminRoles.nameLockedHint') : undefined}
        >
          <Input disabled={lockedIdentity || !!role?.ownerRole} />
        </Form.Item>

        <Tabs
          defaultActiveKey="permissions"
          items={[
            {
              key: 'permissions',
              forceRender: true,
              label: t('pages.adminRoles.permissions'),
              children: permissionPanel,
            },
            {
              key: 'limits',
              forceRender: true,
              label: t('pages.adminRoles.limits'),
              children: limitsPanel,
            },
            {
              key: 'features',
              forceRender: true,
              label: t('pages.adminRoles.features'),
              children: featuresPanel,
            },
            {
              key: 'access',
              forceRender: true,
              label: t('pages.adminRoles.access'),
              children: accessPanel,
            },
          ]}
        />
      </Form>
    </Modal>
  );
}
