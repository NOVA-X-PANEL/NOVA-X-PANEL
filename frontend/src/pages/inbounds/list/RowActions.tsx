import { useTranslation } from 'react-i18next';
import { Button, Dropdown, type MenuProps } from 'antd';
import {
  MoreOutlined,
  EditOutlined,
  CopyOutlined,
  ExportOutlined,
  RetweetOutlined,
  BlockOutlined,
  DeleteOutlined,
  InfoCircleOutlined,
  TagsOutlined,
  UsergroupAddOutlined,
  UsergroupDeleteOutlined,
} from '@ant-design/icons';

import { isInboundMultiUser } from './helpers';
import type { DBInboundRecord, RowAction } from './types';

interface RowActionsMenuProps {
  record: DBInboundRecord;
  subEnable: boolean;
  hasClients: boolean;
  onClick: (key: RowAction) => void;
  isMobile?: boolean;
  canEdit?: boolean;
  canDelete?: boolean;
  canClone?: boolean;
  canResetUsage?: boolean;
}

export function buildRowActionsMenu({
  record,
  subEnable,
  t,
  isMobile,
  hasClients,
  canEdit = true,
  canDelete = true,
  canClone = true,
  canResetUsage = true,
}: {
  record: DBInboundRecord;
  subEnable: boolean;
  t: (k: string) => string;
  isMobile?: boolean;
  hasClients?: boolean;
  canEdit?: boolean;
  canDelete?: boolean;
  canClone?: boolean;
  canResetUsage?: boolean;
}): MenuProps['items'] {
  const items: MenuProps['items'] = [];
  if (isMobile && canEdit) {
    items.push({ key: 'edit', icon: <EditOutlined />, label: t('edit') });
  }
  if (isInboundMultiUser(record)) {
    items.push({ key: 'export', icon: <ExportOutlined />, label: t('pages.inbounds.export') });
    if (subEnable) {
      items.push({
        key: 'subs',
        icon: <ExportOutlined />,
        label: `${t('pages.inbounds.export')} — ${t('pages.settings.subSettings')}`,
      });
    }
  } else {
    items.push({
      key: 'showInfo',
      icon: <InfoCircleOutlined />,
      label: t('pages.inbounds.inboundInfo'),
    });
  }
  items.push({
    key: 'clipboard',
    icon: <CopyOutlined />,
    label: t('pages.inbounds.exportInbound'),
  });
  if (canResetUsage) {
    items.push({
      key: 'resetTraffic',
      icon: <RetweetOutlined />,
      label: t('pages.inbounds.resetTraffic'),
    });
  }
  if (canClone) {
    items.push({ key: 'clone', icon: <BlockOutlined />, label: t('pages.inbounds.clone') });
  }
  if (isInboundMultiUser(record) && canEdit) {
    items.push({
      key: 'attachExisting',
      icon: <UsergroupAddOutlined />,
      label: t('pages.inbounds.attachExistingClients'),
    });
  }
  if (isInboundMultiUser(record) && hasClients) {
    if (canEdit) {
      items.push({
        key: 'attachClients',
        icon: <UsergroupAddOutlined />,
        label: t('pages.inbounds.attachClients'),
      });
      items.push({
        key: 'detachClients',
        icon: <UsergroupDeleteOutlined />,
        label: t('pages.inbounds.detachClients'),
      });
      items.push({
        key: 'addToGroup',
        icon: <TagsOutlined />,
        label: t('pages.inbounds.addClientsToGroup'),
      });
    }
    if (canDelete) {
      items.push({ type: 'divider' });
      items.push({
        key: 'delAllClients',
        icon: <UsergroupDeleteOutlined />,
        danger: true,
        label: t('pages.inbounds.delAllClients'),
      });
    }
  } else if (canDelete) {
    items.push({ type: 'divider' });
  }
  if (canDelete) {
    items.push({ key: 'delete', icon: <DeleteOutlined />, danger: true, label: t('delete') });
  }
  return items;
}

export function RowActionsCell({
  record,
  subEnable,
  hasClients,
  onClick,
  canEdit = true,
  canDelete = true,
  canClone = true,
  canResetUsage = true,
}: RowActionsMenuProps) {
  const { t } = useTranslation();
  return (
    <div className="action-buttons cyber-action-buttons">
      {canEdit && (
        <Button
          type="text"
          size="small"
          className="cyber-action-btn cyber-edit-btn"
          style={{ fontSize: 16 }}
          icon={<EditOutlined />}
          aria-label={t('edit')}
          onClick={() => onClick('edit')}
        />
      )}
      <Dropdown
        trigger={['click']}
        overlayClassName="cyber-action-dropdown"
        menu={{
          items: buildRowActionsMenu({
            record,
            subEnable,
            t,
            hasClients,
            canEdit,
            canDelete,
            canClone,
            canResetUsage,
          }),
          onClick: ({ key }) => onClick(key as RowAction),
        }}
      >
        <Button
          type="text"
          size="small"
          className="cyber-action-btn cyber-more-btn"
          style={{ fontSize: 16 }}
          icon={<MoreOutlined />}
          aria-label={t('more')}
        />
      </Dropdown>
    </div>
  );
}
