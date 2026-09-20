import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  Modal,
  Result,
  Row,
  Spin,
  Statistic,
  message,
} from 'antd';
import {
  CrownOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
} from '@ant-design/icons';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { setMessageInstance } from '@/utils/messageBus';
import { useAdminRoleMutations, useAdminRolesQuery } from '@/api/queries/useAdmins';
import type { AdminRoleDoc } from '@/api/queries/useAdmins';
import AdminRoleList from './AdminRoleList';
import AdminRoleModal, { type AdminRoleFormValues } from './AdminRoleModal';
import './AdminRolesPage.css';

export default function AdminRolesPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [modal, modalContextHolder] = Modal.useModal();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const { roles, loading, fetched, fetchError, refetch } = useAdminRolesQuery();
  const { create, update, duplicate, remove } = useAdminRoleMutations();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'create' | 'edit'>('create');
  const [formRole, setFormRole] = useState<AdminRoleDoc | null>(null);

  const pageClass = useMemo(() => {
    const classes = ['admin-roles-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const totals = useMemo(() => {
    let builtIn = 0;
    let custom = 0;
    let assigned = 0;
    for (const role of roles) {
      if (role.builtIn) builtIn += 1;
      else custom += 1;
      assigned += role.adminCount;
    }
    return { builtIn, custom, assigned };
  }, [roles]);

  const openCreate = () => {
    setFormMode('create');
    setFormRole(null);
    setFormOpen(true);
  };

  const openEdit = (role: AdminRoleDoc) => {
    setFormMode('edit');
    setFormRole(role);
    setFormOpen(true);
  };

  const onSubmit = async (values: AdminRoleFormValues) => {
    const groups = (values.access?.allowedGroups ?? '')
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
    const inboundIds = (values.access?.allowedInboundIds ?? '')
      .split(',')
      .map((item) => Number(item.trim()))
      .filter((id) => Number.isFinite(id) && id > 0);

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
      if (formMode === 'edit' && formRole) {
        await update.mutateAsync({ id: formRole.id, payload });
        messageApi.success(t('pages.adminRoles.toasts.updated'));
      } else {
        await create.mutateAsync(payload);
        messageApi.success(t('pages.adminRoles.toasts.created'));
      }
      setFormOpen(false);
      void refetch();
    } catch (e) {
      messageApi.error(String((e as Error).message ?? e));
    }
  };

  const onDuplicate = (role: AdminRoleDoc) => {
    void duplicate
      .mutateAsync(role.id)
      .then(() => {
        messageApi.success(t('pages.adminRoles.toasts.duplicated'));
        void refetch();
      })
      .catch((e: unknown) => messageApi.error(String((e as Error).message ?? e)));
  };

  const onDelete = (role: AdminRoleDoc) => {
    modal.confirm({
      title: t('pages.adminRoles.deleteConfirm'),
      content: t('pages.adminRoles.deleteHint'),
      okText: t('delete'),
      okButtonProps: { danger: true },
      cancelText: t('cancel'),
      onOk: () =>
        remove
          .mutateAsync(role.id)
          .then(() => {
            messageApi.success(t('pages.adminRoles.toasts.deleted'));
            void refetch();
          })
          .catch((e: unknown) => messageApi.error(String((e as Error).message ?? e))),
    });
  };

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={
                    <Button type="primary" loading={loading} onClick={() => refetch()}>
                      {t('refresh')}
                    </Button>
                  }
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.adminRoles.stats.total')}
                            value={String(roles.length)}
                            prefix={<SafetyCertificateOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.adminRoles.stats.builtIn')}
                            value={String(totals.builtIn)}
                            prefix={<LockOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.adminRoles.stats.custom')}
                            value={String(totals.custom)}
                            prefix={<CrownOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.adminRoles.stats.assigned')}
                            value={String(totals.assigned)}
                            prefix={<TeamOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <AdminRoleList
                      roles={roles}
                      loading={loading}
                      isMobile={isMobile}
                      onAdd={openCreate}
                      onEdit={openEdit}
                      onDuplicate={onDuplicate}
                      onDelete={onDelete}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <AdminRoleModal
          open={formOpen}
          mode={formMode}
          role={formRole}
          saving={create.isPending || update.isPending}
          onOpenChange={setFormOpen}
          onSubmit={onSubmit}
        />
      </Layout>
    </ConfigProvider>
  );
}
