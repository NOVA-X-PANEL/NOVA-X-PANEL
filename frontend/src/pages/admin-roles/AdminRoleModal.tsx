import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Tabs,
  Typography,
} from 'antd';
import { CrownOutlined, DownOutlined, LockOutlined, RightOutlined } from '@ant-design/icons';

import type { AdminRoleDoc } from '@/api/queries/useAdmins';
import { useGroupOptions } from '@/api/queries/useAdmins';
import { useInboundOptions } from '@/api/queries/useInboundOptions';
import type { PermissionScope } from '@/lib/rbac';
import { FEATURE_KEYS, LIMIT_KEYS, PERMISSION_GROUPS, humanizeKey } from '@/lib/rbac';

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
    allowedGroups: string[];
    allowAllInbounds: boolean;
    allowedInboundIds: number[];
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
  allowedGroups: [] as string[],
  allowAllInbounds: true,
  allowedInboundIds: [] as number[],
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

function toList(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
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
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({ clients: true });

  // Reset the expanded groups when the dialog opens. Adjusting state during
  // render (rather than in an effect) is React's documented pattern for
  // "reset state when a prop changes" and avoids a cascading render.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setOpenGroups({ clients: true });
  }

  const { groups } = useGroupOptions(open);
  const inboundsQuery = useInboundOptions();
  const inboundOptions = useMemo(
    () =>
      (inboundsQuery.data ?? []).map((inbound) => ({
        value: inbound.id,
        label: inbound.remark || inbound.tag || `#${inbound.id}`,
      })),
    [inboundsQuery.data],
  );

  const lockedIdentity = mode === 'edit' && !!role?.builtIn;
  const readOnly = mode === 'edit' && !!role?.ownerRole;

  useEffect(() => {
    if (!open) return;
    if (mode === 'edit' && role) {
      const access = (role.access ?? {}) as Record<string, unknown>;
      const allowedGroups = toList(access.allowedGroups).map(String);
      const allowedInboundIds = toList(access.allowed_inbound_ids ?? access.allowedInboundIds).map(
        Number,
      );
      form.setFieldsValue({
        name: role.name,
        permissions: toPermissionMap(role.permissions),
        limits: (role.limits ?? {}) as Record<string, number | null>,
        features: (role.features ?? {}) as Record<string, boolean>,
        access: {
          allowAllGroups: access.allowAllGroups !== false,
          allowedGroups,
          allowAllInbounds: access.allowAllInbounds !== false,
          allowedInboundIds: allowedInboundIds.filter((id) => Number.isFinite(id)),
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

  const permissions = Form.useWatch('permissions', form) as PermissionFormMap | undefined;

  const setPermission = useCallback(
    (resource: string, action: string, value: PermissionFormValue) => {
      const next: PermissionFormMap = { ...(permissions ?? {}) };
      next[resource] = { ...(next[resource] || {}), [action]: value };
      form.setFieldValue('permissions', next);
    },
    [form, permissions],
  );

  const setGroupAll = useCallback(
    (resource: string, actions: { action: string; scoped?: boolean }[], enable: boolean) => {
      const next: PermissionFormMap = { ...(permissions ?? {}) };
      const inner: Record<string, PermissionFormValue | undefined> = { ...(next[resource] || {}) };
      for (const item of actions) {
        inner[item.action] = enable ? (item.scoped ? 1 : true) : item.scoped ? 0 : false;
      }
      next[resource] = inner;
      form.setFieldValue('permissions', next);
    },
    [form, permissions],
  );

  const resourceLabel = (resource: string) =>
    t(`pages.adminRoles.resources.${resource}`, humanizeKey(resource));
  const actionLabel = (resource: string, action: string) =>
    t(`pages.adminRoles.actionLabels.${action}`, humanizeKey(action));

  const permissionsPanel = (
    <div className="permission-panel">
      <Typography.Paragraph type="secondary" className="section-hint">
        {t('pages.adminRoles.permissionsHint')}
      </Typography.Paragraph>

      <div className="permission-groups">
        {PERMISSION_GROUPS.map((group) => {
          const resource = group.actions[0].resource;
          const section = permissions?.[resource] ?? {};
          const enabled = group.actions.reduce((acc, item) => {
            const value = section[item.action];
            if (value === true) return acc + 1;
            if (typeof value === 'number' && value > 0) return acc + 1;
            return acc;
          }, 0);
          const isOpen = !!openGroups[group.labelKey];
          const showResourcePrefix = group.actions.some((a) =>
            group.actions.some(
              (b) => b !== a && b.action === a.action && b.resource !== a.resource,
            ),
          );

          return (
            <div className="permission-group" key={group.labelKey}>
              <div className="permission-group__head">
                <button
                  type="button"
                  className="permission-group__toggle"
                  aria-expanded={isOpen}
                  onClick={() =>
                    setOpenGroups((prev) => ({ ...prev, [group.labelKey]: !prev[group.labelKey] }))
                  }
                >
                  {isOpen ? <DownOutlined /> : <RightOutlined />}
                  <span className="permission-group__label">{resourceLabel(resource)}</span>
                  <span className="permission-group__count">
                    {enabled}/{group.actions.length}
                  </span>
                </button>
                <Space size={4}>
                  <Button
                    type="text"
                    size="small"
                    disabled={readOnly}
                    onClick={() => setGroupAll(resource, group.actions, true)}
                  >
                    {t('pages.adminRoles.selectAll')}
                  </Button>
                  <Button
                    type="text"
                    size="small"
                    disabled={readOnly}
                    onClick={() => setGroupAll(resource, group.actions, false)}
                  >
                    {t('pages.adminRoles.clearAll')}
                  </Button>
                </Space>
              </div>

              {isOpen && (
                <div className="permission-rows">
                  {group.actions.map((item) => {
                    const value = section[item.action];
                    const scopeValue: PermissionScope =
                      typeof value === 'number' ? value : value === true ? 2 : 0;
                    const boolValue = value === true;
                    const key = `${item.resource}.${item.action}`;

                    return (
                      <div className="permission-row" key={key}>
                        <div className="permission-row__copy">
                          <span className="permission-row__label">
                            {showResourcePrefix
                              ? `${resourceLabel(item.resource)} · ${actionLabel(item.resource, item.action)}`
                              : actionLabel(item.resource, item.action)}
                          </span>
                          {item.scoped && (
                            <span className="permission-row__scoped">
                              {t('pages.adminRoles.scopedBadge')}
                            </span>
                          )}
                        </div>

                        {item.scoped ? (
                          <Select
                            size="small"
                            className="permission-row__scope"
                            value={scopeValue}
                            disabled={readOnly}
                            onChange={(next) => setPermission(item.resource, item.action, next)}
                            options={[
                              { value: 0, label: t('pages.adminRoles.scopes.none') },
                              { value: 1, label: t('pages.adminRoles.scopes.own') },
                              { value: 2, label: t('pages.adminRoles.scopes.all') },
                            ]}
                          />
                        ) : (
                          <Switch
                            size="small"
                            checked={boolValue}
                            disabled={readOnly}
                            aria-label={actionLabel(item.resource, item.action)}
                            onChange={(next) => setPermission(item.resource, item.action, next)}
                          />
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );

  const limitsPanel = (
    <div className="limits-panel">
      <Typography.Paragraph type="secondary" className="section-hint">
        {t('pages.adminRoles.limitsHint')}
      </Typography.Paragraph>

      <Form.Item name={['limits', 'max_users']} label={t('pages.adminRoles.limitFields.max_users')}>
        <InputNumber
          min={0}
          style={{ width: '100%' }}
          placeholder={t('pages.adminRoles.unlimited')}
          disabled={readOnly}
        />
      </Form.Item>

      <div className="field-grid">
        {LIMIT_KEYS.filter((key) => key !== 'max_users').map((key) => (
          <Form.Item
            key={key}
            name={['limits', key]}
            label={t(`pages.adminRoles.limitFields.${key}`)}
          >
            <InputNumber
              min={0}
              style={{ width: '100%' }}
              placeholder={t('pages.adminRoles.inherit')}
              disabled={readOnly}
            />
          </Form.Item>
        ))}
      </div>
    </div>
  );

  const featuresPanel = (
    <div className="feature-rows">
      {FEATURE_KEYS.map((key) => (
        <div className="feature-row" key={key}>
          <div className="feature-row__copy">
            <span className="feature-row__title">
              {t(`pages.adminRoles.featureLabels.${key}.title`, humanizeKey(key))}
            </span>
            <span className="feature-row__desc">
              {t(`pages.adminRoles.featureLabels.${key}.hint`, '')}
            </span>
          </div>
          <Form.Item name={['features', key]} valuePropName="checked" noStyle>
            <Switch size="small" disabled={readOnly} />
          </Form.Item>
        </div>
      ))}
    </div>
  );

  const accessPanel = (
    <div className="access-panel">
      <div className="access-row">
        <div className="access-row__copy">
          <span className="access-row__title">{t('pages.adminRoles.allowAllGroups')}</span>
          <span className="access-row__desc">{t('pages.adminRoles.allowedGroupsDescription')}</span>
        </div>
        <Form.Item name={['access', 'allowAllGroups']} valuePropName="checked" noStyle>
          <Switch size="small" disabled={readOnly} />
        </Form.Item>
      </div>
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
            >
              <Select
                mode="multiple"
                placeholder={t('pages.adminRoles.chooseGroups')}
                disabled={readOnly}
                options={groups.map((name) => ({ value: name, label: name }))}
                notFoundContent={t('pages.adminRoles.noGroups')}
              />
            </Form.Item>
          )
        }
      </Form.Item>

      <Divider />

      <div className="access-row">
        <div className="access-row__copy">
          <span className="access-row__title">{t('pages.adminRoles.allowAllInbounds')}</span>
          <span className="access-row__desc">
            {t('pages.adminRoles.allowedInboundsDescription')}
          </span>
        </div>
        <Form.Item name={['access', 'allowAllInbounds']} valuePropName="checked" noStyle>
          <Switch size="small" disabled={readOnly} />
        </Form.Item>
      </div>
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
            >
              <Select
                mode="multiple"
                placeholder={t('pages.adminRoles.chooseInbounds')}
                disabled={readOnly}
                options={inboundOptions}
                notFoundContent={t('pages.adminRoles.noInbounds')}
              />
            </Form.Item>
          )
        }
      </Form.Item>
    </div>
  );

  return (
    <Modal
      open={open}
      title={mode === 'create' ? t('pages.adminRoles.create') : t('pages.adminRoles.edit')}
      width={760}
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
        <Alert
          type={role?.ownerRole ? 'warning' : 'info'}
          showIcon
          icon={role?.ownerRole ? <CrownOutlined /> : <LockOutlined />}
          style={{ marginBottom: 12 }}
          message={
            role?.ownerRole
              ? t('pages.adminRoles.ownerReadOnly')
              : role?.builtIn
                ? t('pages.adminRoles.builtInHint')
                : t('pages.adminRoles.roleFormHint')
          }
        />

        <Form.Item
          name="name"
          label={t('pages.adminRoles.columns.name')}
          rules={[{ required: true, message: t('pages.adminRoles.nameRequired') }]}
          extra={lockedIdentity ? t('pages.adminRoles.nameLockedHint') : undefined}
        >
          <Input disabled={lockedIdentity || readOnly} />
        </Form.Item>

        <Tabs
          defaultActiveKey="permissions"
          items={[
            {
              key: 'permissions',
              forceRender: true,
              label: t('pages.adminRoles.permissions'),
              children: permissionsPanel,
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
