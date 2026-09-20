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
  CheckCircleOutlined,
  CloseCircleOutlined,
  SafetyCertificateOutlined,
  UserSwitchOutlined,
} from '@ant-design/icons';

import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { setMessageInstance } from '@/utils/messageBus';
import {
  useAdminMutations,
  useAdminRolesQuery,
  useAdminStatsQuery,
  useAdminsQuery,
} from '@/api/queries/useAdmins';
import type { AdminAccount } from '@/api/queries/useAdmins';
import AdminList from './AdminList';
import AdminFormModal, { type AdminFormValues } from './AdminFormModal';
import './AdminsPage.css';

export default function AdminsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [modal, modalContextHolder] = Modal.useModal();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const { admins, loading, fetched, fetchError, refetch } = useAdminsQuery();
  const { stats } = useAdminStatsQuery();
  const { roles } = useAdminRolesQuery();
  const { create, update, remove, setStatus } = useAdminMutations();

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'create' | 'edit'>('create');
  const [formAdmin, setFormAdmin] = useState<AdminAccount | null>(null);

  const pageClass = useMemo(() => {
    const classes = ['admins-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const openCreate = () => {
    setFormMode('create');
    setFormAdmin(null);
    setFormOpen(true);
  };

  const openEdit = (admin: AdminAccount) => {
    setFormMode('edit');
    setFormAdmin(admin);
    setFormOpen(true);
  };

  const onSubmit = async (values: AdminFormValues) => {
    const payload = {
      username: values.username,
      password: values.password || undefined,
      roleId: values.roleId,
      status: values.status ? 'active' : 'disabled',
    };
    try {
      if (formMode === 'edit' && formAdmin) {
        await update.mutateAsync({ id: formAdmin.id, payload });
        messageApi.success(t('pages.admins.toasts.updated'));
      } else {
        await create.mutateAsync(payload);
        messageApi.success(t('pages.admins.toasts.created'));
      }
      setFormOpen(false);
      void refetch();
    } catch (e) {
      messageApi.error(String((e as Error).message ?? e));
    }
  };

  const onToggleStatus = async (admin: AdminAccount, next: boolean) => {
    try {
      await setStatus.mutateAsync({ id: admin.id, enabled: next });
      messageApi.success(t('pages.admins.toasts.statusChanged'));
      void refetch();
    } catch (e) {
      messageApi.error(String((e as Error).message ?? e));
    }
  };

  const onDelete = (admin: AdminAccount) => {
    modal.confirm({
      title: t('pages.admins.deleteConfirm'),
      okText: t('delete'),
      okButtonProps: { danger: true },
      cancelText: t('cancel'),
      onOk: () =>
        remove
          .mutateAsync(admin.id)
          .then(() => {
            messageApi.success(t('pages.admins.toasts.deleted'));
            void refetch();
          })
          .catch((e: unknown) => messageApi.error(String((e as Error).message ?? e))),
    });
  };

  // Opening the form in edit mode with an empty password field is the panel's
  // reset flow: saving a new password rotates it.
  const onResetPassword = (admin: AdminAccount) => openEdit(admin);

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
                            title={t('pages.admins.stats.total')}
                            value={String(stats?.totalAdmins ?? admins.length)}
                            prefix={<UserSwitchOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.stats.active')}
                            value={String(stats?.activeAdmins ?? 0)}
                            prefix={
                              <CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />
                            }
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.stats.disabled')}
                            value={String(stats?.disabledAdmins ?? 0)}
                            prefix={
                              <CloseCircleOutlined style={{ color: 'var(--ant-color-error)' }} />
                            }
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.stats.roles')}
                            value={String(roles.length)}
                            prefix={<SafetyCertificateOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <AdminList
                      admins={admins}
                      loading={loading}
                      isMobile={isMobile}
                      onAdd={openCreate}
                      onEdit={openEdit}
                      onDelete={onDelete}
                      onToggleStatus={onToggleStatus}
                      onResetPassword={onResetPassword}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <AdminFormModal
          open={formOpen}
          mode={formMode}
          admin={formAdmin}
          roles={roles}
          saving={create.isPending || update.isPending}
          onOpenChange={setFormOpen}
          onSubmit={onSubmit}
        />
      </Layout>
    </ConfigProvider>
  );
}
