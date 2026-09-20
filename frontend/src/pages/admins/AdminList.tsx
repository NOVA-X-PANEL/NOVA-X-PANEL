import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge, Button, Card, Dropdown, Progress, Space, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  CrownOutlined,
  DeleteOutlined,
  EditOutlined,
  KeyOutlined,
  LockOutlined,
  MoreOutlined,
  PlusOutlined,
  PoweroffOutlined,
  SafetyCertificateOutlined,
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

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** exp).toFixed(exp === 0 ? 0 : 1)} ${units[exp]}`;
}

function AdminStatusBadge({ admin }: { admin: AdminAccount }) {
  const { t } = useTranslation();
  if (admin.limited) {
    return <Badge status="warning" text={t('pages.admins.limited')} />;
  }
  return (
    <Badge
      status={admin.status === 'active' ? 'success' : 'default'}
      text={admin.status === 'active' ? t('pages.admins.active') : t('pages.admins.disabled')}
    />
  );
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
            <span className="name">
              {record.ownerRole ? <CrownOutlined className="role-glyph" /> : <UserSwitchOutlined />}{' '}
              {record.username}
            </span>
            <span className="remark">{record.roleName || '—'}</span>
          </div>
        ),
      },
      {
        title: t('pages.admins.columns.role'),
        dataIndex: 'roleName',
        key: 'roleName',
        responsive: ['md'],
        render: (_: string, record) => (
          <Tag
            icon={record.ownerRole ? <CrownOutlined /> : <SafetyCertificateOutlined />}
            color={record.ownerRole ? 'gold' : undefined}
            style={{ margin: 0 }}
          >
            {record.roleName}
          </Tag>
        ),
      },
      {
        title: t('pages.admins.columns.status'),
        key: 'status',
        width: 130,
        render: (_: unknown, record) => <AdminStatusBadge admin={record} />,
      },
      {
        title: t('pages.admins.columns.users'),
        dataIndex: 'totalUsers',
        key: 'totalUsers',
        width: 100,
        responsive: ['sm'],
        render: (value: number) => <Badge count={value} showZero overflowCount={9999} />,
      },
      {
        title: t('pages.admins.columns.usage'),
        key: 'usage',
        width: 170,
        responsive: ['lg'],
        render: (_: unknown, record) => {
          if (!record.dataLimit || record.dataLimit <= 0) {
            return (
              <Tooltip title={t('pages.admins.unlimitedHint')}>
                <span className="usage-cell">
                  {formatBytes(record.usedBytes)} · {t('pages.admins.unlimited')}
                </span>
              </Tooltip>
            );
          }
          const percent = Math.min(100, Math.round((record.usedBytes / record.dataLimit) * 100));
          return (
            <Tooltip title={`${formatBytes(record.usedBytes)} / ${formatBytes(record.dataLimit)}`}>
              <Progress
                percent={percent}
                size="small"
                status={percent >= 100 ? 'exception' : 'normal'}
                format={(value) => `${value}%`}
              />
            </Tooltip>
          );
        },
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
                  label: t('pages.admins.edit'),
                  disabled: record.ownerRole,
                  onClick: () => onEdit(record),
                },
                {
                  key: 'resetPassword',
                  icon: <KeyOutlined />,
                  label: t('pages.admins.resetPassword'),
                  onClick: () => onResetPassword(record),
                },
                { type: 'divider' },
                {
                  key: 'toggle',
                  icon: <PoweroffOutlined />,
                  label:
                    record.status === 'active'
                      ? t('pages.admins.disable')
                      : t('pages.admins.enable'),
                  disabled: isProtected(record),
                  onClick: () => onToggleStatus(record, record.status !== 'active'),
                },
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
        <Space size={6}>
          <Tooltip title={t('pages.admins.ownerHint')}>
            <Tag icon={<CrownOutlined />} color="gold" style={{ margin: 0 }}>
              {t('pages.admins.ownerTag')}
            </Tag>
          </Tooltip>
          <Tooltip title={t('pages.admins.roleHint')}>
            <Tag icon={<LockOutlined />} style={{ margin: 0 }}>
              {t('pages.adminRoles.title')}
            </Tag>
          </Tooltip>
        </Space>
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
              <ThunderboltOutlined style={{ fontSize: 28, opacity: 0.5 }} />
              <div>{t('noData')}</div>
            </div>
          ),
        }}
        expandable={{
          expandedRowRender: (record) => (
            <div className="admin-detail">
              <span className="admin-detail__label">{t('pages.admins.columns.role')}</span>
              <span className="admin-detail__value">{record.roleSlug || '—'}</span>
              <span className="admin-detail__label">{t('pages.admins.columns.usage')}</span>
              <span className="admin-detail__value">
                {formatBytes(record.usedBytes)}
                {record.dataLimit > 0 ? ` / ${formatBytes(record.dataLimit)}` : ''}
              </span>
              <span className="admin-detail__label">{t('pages.admins.columns.users')}</span>
              <span className="admin-detail__value">{record.totalUsers}</span>
              <span className="admin-detail__label">{t('pages.admins.columns.status')}</span>
              <span className="admin-detail__value">
                {record.limited
                  ? t('pages.admins.limited')
                  : record.status === 'active'
                    ? t('pages.admins.active')
                    : t('pages.admins.disabled')}
              </span>
            </div>
          ),
        }}
      />
    </Card>
  );
}
