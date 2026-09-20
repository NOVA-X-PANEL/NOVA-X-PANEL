import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge, Button, Card, Dropdown, Space, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  CopyOutlined,
  CrownOutlined,
  DeleteOutlined,
  EditOutlined,
  LockOutlined,
  MoreOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import type { AdminRoleDoc } from '@/api/queries/useAdmins';
import './AdminRoleList.css';

interface AdminRoleListProps {
  roles: AdminRoleDoc[];
  loading?: boolean;
  isMobile?: boolean;
  onAdd: () => void;
  onEdit: (role: AdminRoleDoc) => void;
  onDuplicate: (role: AdminRoleDoc) => void;
  onDelete: (role: AdminRoleDoc) => void;
}

function countPermissions(role: AdminRoleDoc): number {
  let total = 0;
  for (const actions of Object.values(role.permissions ?? {})) {
    if (!actions || typeof actions !== 'object') continue;
    for (const value of Object.values(actions)) {
      if (value === true) total += 1;
      else if (value && typeof value === 'object') {
        const scope = (value as { scope?: unknown }).scope;
        if (scope === 1 || scope === 2) total += 1;
      }
    }
  }
  return total;
}

export default function AdminRoleList({
  roles,
  loading,
  isMobile,
  onAdd,
  onEdit,
  onDuplicate,
  onDelete,
}: AdminRoleListProps) {
  const { t } = useTranslation();

  const columns: ColumnsType<AdminRoleDoc> = useMemo(
    () => [
      {
        title: t('pages.adminRoles.columns.name'),
        dataIndex: 'name',
        key: 'name',
        render: (_: string, record) => (
          <div className="name-cell">
            <span className="name">
              {record.ownerRole && <CrownOutlined className="role-glyph" />} {record.name}
            </span>
            <span className="remark">{record.slug}</span>
          </div>
        ),
      },
      {
        title: t('pages.adminRoles.columns.type'),
        key: 'type',
        responsive: ['sm'],
        render: (_: unknown, record) => (
          <Space size={4} wrap>
            {record.ownerRole && (
              <Tag icon={<CrownOutlined />} color="gold" style={{ margin: 0 }}>
                {t('pages.adminRoles.owner')}
              </Tag>
            )}
            {record.builtIn && !record.ownerRole && (
              <Tag icon={<LockOutlined />} color="blue" style={{ margin: 0 }}>
                {t('pages.adminRoles.builtIn')}
              </Tag>
            )}
            {!record.builtIn && (
              <Tag icon={<SafetyCertificateOutlined />} style={{ margin: 0 }}>
                {t('pages.adminRoles.custom')}
              </Tag>
            )}
          </Space>
        ),
      },
      {
        title: t('pages.adminRoles.columns.admins'),
        dataIndex: 'adminCount',
        key: 'adminCount',
        width: 110,
        responsive: ['md'],
        render: (value: number) => (
          <Badge
            count={value}
            showZero
            overflowCount={99}
            color={value > 0 ? 'var(--ant-color-primary)' : 'var(--ant-color-fill-secondary)'}
          />
        ),
      },
      {
        title: t('pages.adminRoles.columns.permissions'),
        key: 'permissions',
        width: 130,
        responsive: ['lg'],
        render: (_: unknown, record) => (
          <Tooltip title={t('pages.adminRoles.permissionsHint')}>
            <Tag style={{ margin: 0 }}>
              <TeamOutlined /> {countPermissions(record)}
            </Tag>
          </Tooltip>
        ),
      },
      {
        title: t('pages.adminRoles.columns.actions'),
        key: 'actions',
        width: 60,
        align: 'right',
        render: (_: unknown, record) => (
          <Dropdown
            trigger={['click']}
            placement="bottomRight"
            menu={{
              items: [
                {
                  key: 'edit',
                  icon: <EditOutlined />,
                  label: t('edit'),
                  disabled: record.ownerRole,
                  onClick: () => onEdit(record),
                },
                {
                  key: 'duplicate',
                  icon: <CopyOutlined />,
                  label: t('pages.adminRoles.duplicate'),
                  onClick: () => onDuplicate(record),
                },
                { type: 'divider' },
                {
                  key: 'delete',
                  icon: <DeleteOutlined />,
                  danger: true,
                  label: t('delete'),
                  disabled: record.builtIn || record.ownerRole || record.adminCount > 0,
                  onClick: () => onDelete(record),
                },
              ],
            }}
          >
            <Button
              type="text"
              size="small"
              className="row-action-trigger"
              icon={<MoreOutlined />}
              aria-label={t('more')}
            />
          </Dropdown>
        ),
      },
    ],
    [t, onEdit, onDuplicate, onDelete],
  );

  return (
    <Card size="small" hoverable className="admin-role-list">
      <div className="toolbar">
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {t('pages.adminRoles.create')}
        </Button>
        <Tooltip title={t('pages.adminRoles.deleteHint')}>
          <span className="hint">
            <LockOutlined /> {t('pages.adminRoles.protectedHint')}
          </span>
        </Tooltip>
      </div>

      <Table<AdminRoleDoc>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={roles}
        pagination={false}
        scroll={isMobile ? { x: 'max-content' } : undefined}
        locale={{
          emptyText: (
            <div className="empty-state">
              <SafetyCertificateOutlined style={{ fontSize: 28, opacity: 0.5 }} />
              <div>{t('noData')}</div>
            </div>
          ),
        }}
      />
    </Card>
  );
}
