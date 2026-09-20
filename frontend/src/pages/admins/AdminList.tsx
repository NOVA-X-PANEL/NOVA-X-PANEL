import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge, Button, Card, Dropdown, Space, Switch, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  CrownOutlined,
  DeleteOutlined,
  EditOutlined,
  KeyOutlined,
  MoreOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  StopOutlined,
  ThunderboltOutlined,
  UserSwitchOutlined,
} from '@ant-design/icons';

import type { AdminAccount } from '@/api/queries/useAdmins';
import './AdminList.css';

interface AdminListProps {
  admins: AdminAccount[];
  loading?: boolean;
  isMobile?: boolean;
  onAdd: () => void;
  onEdit: (admin: AdminAccount) => void;
  onDelete: (admin: AdminAccount) => void;
  onToggleStatus: (admin: AdminAccount, next: boolean) => void;
  onResetPassword: (admin: AdminAccount) => void;
}

function isProtected(admin: AdminAccount): boolean {
  return admin.isSelf || admin.ownerRole;
}

export default function AdminList({
  admins,
  loading,
  isMobile,
  onAdd,
  onEdit,
  onDelete,
  onToggleStatus,
  onResetPassword,
}: AdminListProps) {
  const { t } = useTranslation();

  const columns: ColumnsType<AdminAccount> = useMemo(
    () => [
      {
        title: t('pages.admins.columns.username'),
        dataIndex: 'username',
        key: 'username',
        render: (_: string, record) => (
          <div className="name-cell">
            <span className="name">{record.username}</span>
            <span className="remark">{record.roleName || '—'}</span>
          </div>
        ),
      },
      {
        title: t('pages.admins.columns.role'),
        dataIndex: 'roleName',
        key: 'roleName',
        responsive: ['sm'],
        render: (_: string, record) =>
          record.ownerRole ? (
            <Tag icon={<CrownOutlined />} color="gold" style={{ margin: 0 }}>
              {record.roleName}
            </Tag>
          ) : (
            <Tag icon={<SafetyCertificateOutlined />} style={{ margin: 0 }}>
              {record.roleName}
            </Tag>
          ),
      },
      {
        title: t('pages.admins.columns.status'),
        dataIndex: 'status',
        key: 'status',
        width: 120,
        render: (_: string, record) => (
          <Space size={8}>
            <Badge
              status={record.status === 'active' ? 'success' : 'default'}
              text={record.status === 'active' ? t('enabled') : t('disabled')}
            />
            <Switch
              size="small"
              checked={record.status === 'active'}
              disabled={isProtected(record)}
              aria-label={t('pages.admins.columns.status')}
              onChange={(next) => onToggleStatus(record, next)}
            />
          </Space>
        ),
      },
      {
        title: t('pages.admins.columns.actions'),
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
                  key: 'reset',
                  icon: <KeyOutlined />,
                  label: t('pages.admins.resetPassword'),
                  onClick: () => onResetPassword(record),
                },
                {
                  key: 'toggle',
                  icon: record.status === 'active' ? <StopOutlined /> : <ThunderboltOutlined />,
                  label:
                    record.status === 'active'
                      ? t('pages.admins.disable')
                      : t('pages.admins.enable'),
                  disabled: isProtected(record),
                  onClick: () => onToggleStatus(record, record.status !== 'active'),
                },
                { type: 'divider' },
                {
                  key: 'delete',
                  icon: <DeleteOutlined />,
                  danger: true,
                  label: t('delete'),
                  disabled: isProtected(record),
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
    [t, onEdit, onResetPassword, onDelete, onToggleStatus],
  );

  return (
    <Card size="small" hoverable className="admin-list">
      <div className="toolbar">
        <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {t('pages.admins.create')}
        </Button>
        <Tooltip title={t('pages.admins.ownerHint')}>
          <Tag icon={<UserSwitchOutlined />} style={{ margin: 0 }}>
            {t('pages.admins.ownerTag')}
          </Tag>
        </Tooltip>
      </div>

      <Table<AdminAccount>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={admins}
        pagination={false}
        scroll={isMobile ? { x: 'max-content' } : undefined}
        locale={{
          emptyText: (
            <div className="empty-state">
              <UserSwitchOutlined style={{ fontSize: 28, opacity: 0.5 }} />
              <div>{t('noData')}</div>
            </div>
          ),
        }}
        expandable={{
          expandedRowRender: (record) => (
            <div className="admin-detail">
              <span className="admin-detail__label">{t('pages.admins.columns.role')}</span>
              <span className="admin-detail__value">{record.roleSlug || '—'}</span>
              <span className="admin-detail__label">{t('pages.admins.columns.status')}</span>
              <span className="admin-detail__value">
                {record.status === 'active' ? t('enabled') : t('disabled')}
              </span>
            </div>
          ),
        }}
        rowClassName={(record) => (isProtected(record) ? 'row-protected' : '')}
      />
    </Card>
  );
}
