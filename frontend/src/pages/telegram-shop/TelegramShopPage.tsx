import { useState, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Layout,
  ConfigProvider,
  Card,
  Form,
  Input,
  Button,
  Space,
  Row,
  Col,
  Typography,
  message,
  Popconfirm,
  Spin,
  Alert,
  Statistic,
} from 'antd';
import {
  ShoppingCartOutlined,
  ReloadOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  DeleteOutlined,
  SendOutlined,
  CheckCircleOutlined,
  ThunderboltOutlined,
  UserOutlined,
  SafetyCertificateOutlined,
  InfoCircleOutlined,
  GlobalOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import './TelegramShopPage.css';

const { Title, Text, Paragraph } = Typography;

interface TgShopStatus {
  installed: boolean;
  running: boolean;
  bot_token?: string;
  admin_chat_id?: string;
  card_number?: string;
  card_holder?: string;
  bot_username?: string;
  bot_first_name?: string;
  support_username?: string;
  channel_username?: string;
  total_trials: number;
  total_orders: number;
  domain?: string;
}

export default function TelegramShopPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [form] = Form.useForm();

  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState(false);
  const [installingDeps, setInstallingDeps] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [status, setStatus] = useState<TgShopStatus | null>(null);

  const pageClass = useMemo(() => {
    const classes = ['tgshop-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const fetchStatus = async () => {
    try {
      setLoading(true);
      const res = await HttpUtil.get<TgShopStatus>('/panel/api/telegram-shop/status');
      if (res && res.obj) {
        setStatus(res.obj);
        if (res.obj.installed) {
          form.setFieldsValue({
            bot_token: res.obj.bot_token,
            admin_chat_id: res.obj.admin_chat_id,
          });
        }
      }
    } catch (err: any) {
      message.error(err?.message || t('pages.telegramShop.toasts.fetchError'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStatus();
  }, []);

  const handleInstall = async (values: any) => {
    try {
      setInstalling(true);
      const res = await HttpUtil.post('/panel/api/telegram-shop/install', values, { silent: true });
      if (res && res.success) {
        message.success(t('pages.telegramShop.toasts.installedSuccess'));
        fetchStatus();
      } else {
        message.error(res?.msg || t('pages.telegramShop.toasts.installError'));
      }
    } catch (err: any) {
      message.error(err?.message || t('pages.telegramShop.toasts.installError'));
    } finally {
      setInstalling(false);
    }
  };

  const handleInstallDeps = async () => {
    try {
      setInstallingDeps(true);
      const res = await HttpUtil.post('/panel/api/telegram-shop/install-deps');
      if (res && res.success) {
        message.success(t('pages.telegramShop.installDepsSuccess'));
      } else {
        message.error(res?.msg || t('pages.telegramShop.installDepsError'));
      }
    } catch (err: any) {
      message.error(err?.message || t('pages.telegramShop.installDepsError'));
    } finally {
      setInstallingDeps(false);
    }
  };

  const handleAction = async (action: 'start' | 'stop' | 'restart' | 'uninstall') => {
    try {
      setActionLoading(true);
      const res = await HttpUtil.post('/panel/api/telegram-shop/action', { action }, { silent: true });
      if (res && res.success) {
        if (action === 'uninstall') {
          message.warning(t('pages.telegramShop.toasts.uninstalledSuccess'));
          form.resetFields();
        } else {
          message.success(t('pages.telegramShop.toasts.actionSuccess', { action }));
        }
        fetchStatus();
      } else {
        message.error(res?.msg || t('pages.telegramShop.toasts.actionError'));
      }
    } catch (err: any) {
      message.error(err?.message || t('pages.telegramShop.toasts.actionError'));
    } finally {
      setActionLoading(false);
    }
  };

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            {/* HERO BANNER */}
            <div className="tgshop-hero">
              <Row justify="space-between" align="middle" gutter={[16, 16]}>
                <Col xs={24} md={16}>
                  <Space align="center" size="middle">
                    <ShoppingCartOutlined style={{ fontSize: 36, color: '#06b6d4', flexShrink: 0 }} />
                    <div style={{ textAlign: 'start' }}>
                      <Title level={2} className="tgshop-hero-title-text" style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                        <span>{t('pages.telegramShop.title')}</span>
                        <span
                          style={{
                            color: '#fbbf24',
                            backgroundColor: 'rgba(245, 158, 11, 0.16)',
                            border: '1px solid rgba(245, 158, 11, 0.45)',
                            padding: '1px 8px',
                            borderRadius: 4,
                            fontSize: 12,
                            fontWeight: 700,
                            lineHeight: '18px',
                          }}
                        >
                          beta
                        </span>
                      </Title>
                      <Paragraph style={{ margin: 0, color: '#94a3b8', fontSize: 13, marginTop: 4 }}>
                        {t('pages.telegramShop.subtitle')}
                      </Paragraph>
                    </div>
                  </Space>
                </Col>
                <Col xs={24} md={8} style={{ textAlign: 'end' }}>
                  <Space wrap>
                    <Button icon={<ReloadOutlined />} onClick={fetchStatus} loading={loading}>
                      {t('pages.telegramShop.refreshStatus')}
                    </Button>
                    {status?.installed && (
                      <span className={status.running ? 'tgshop-badge-online' : 'tgshop-badge-offline'}>
                        <span className="tgshop-dot" style={{ backgroundColor: status.running ? '#10b981' : '#ef4444' }} />
                        {status.running ? t('pages.telegramShop.statusOnline') : t('pages.telegramShop.statusOffline')}
                      </span>
                    )}
                  </Space>
                </Col>
              </Row>
            </div>

            {loading ? (
              <div style={{ textAlign: 'center', padding: '60px 0' }}>
                <Spin size="large" />
                <div style={{ marginTop: 12, color: '#94a3b8' }}>{t('pages.telegramShop.loadingStatus')}</div>
              </div>
            ) : !status?.installed ? (
              /* INSTALLER VIEW (NOT INSTALLED) */
              <Row gutter={[24, 24]}>
                <Col xs={24} lg={15}>
                  <Card
                    className="tgshop-card"
                    title={
                      <Space>
                        <ThunderboltOutlined style={{ color: '#06b6d4' }} />
                        <span style={{ color: '#fff', fontWeight: 700 }}>{t('pages.telegramShop.installerTitle')}</span>
                      </Space>
                    }
                  >
                    <Alert
                      type="info"
                      showIcon
                      icon={<InfoCircleOutlined />}
                      style={{ marginBottom: 20, background: 'rgba(6, 182, 212, 0.08)', borderColor: 'rgba(6, 182, 212, 0.2)' }}
                      message={t('pages.telegramShop.installerAlertTitle')}
                      description={t('pages.telegramShop.installerAlertDesc')}
                    />

                    <Form form={form} layout="vertical" onFinish={handleInstall}>
                      <Form.Item
                        name="bot_token"
                        label={<span style={{ color: '#e2e8f0' }}>{t('pages.telegramShop.botTokenLabel')}</span>}
                        extra={t('pages.telegramShop.botTokenExtra')}
                        rules={[{ required: true, message: t('pages.telegramShop.botTokenRequired') }]}
                      >
                        <Input
                          placeholder="8781465649:AAEEKMK_umrtGoi8..."
                          prefix={<SendOutlined style={{ color: '#06b6d4' }} />}
                          size="large"
                        />
                      </Form.Item>

                      <Form.Item
                        name="admin_chat_id"
                        label={<span style={{ color: '#e2e8f0' }}>{t('pages.telegramShop.adminChatIdLabel')}</span>}
                        extra={t('pages.telegramShop.adminChatIdExtra')}
                        rules={[{ required: true, message: t('pages.telegramShop.adminChatIdRequired') }]}
                      >
                        <Input
                          placeholder="123456789"
                          prefix={<UserOutlined style={{ color: '#8b5cf6' }} />}
                          size="large"
                        />
                      </Form.Item>

                      <Space direction="vertical" style={{ width: '100%', marginTop: 12 }}>
                        <Button
                          type="primary"
                          htmlType="submit"
                          size="large"
                          block
                          className="tgshop-btn-install"
                          loading={installing}
                          icon={<ThunderboltOutlined />}
                        >
                          {t('pages.telegramShop.installBtn')}
                        </Button>

                        <Button
                          size="large"
                          block
                          icon={<SettingOutlined />}
                          loading={installingDeps}
                          onClick={handleInstallDeps}
                          style={{ borderColor: 'rgba(6, 182, 212, 0.4)', color: '#06b6d4', background: 'rgba(6, 182, 212, 0.05)' }}
                        >
                          {t('pages.telegramShop.installDepsBtn')}
                        </Button>
                      </Space>
                    </Form>
                  </Card>
                </Col>

                <Col xs={24} lg={9}>
                  <Card className="tgshop-card" title={<span style={{ color: '#fff' }}>{t('pages.telegramShop.featuresTitle')}</span>}>
                    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                      <div>
                        <Text strong style={{ color: '#06b6d4' }}>{t('pages.telegramShop.feature1Title')}</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          {t('pages.telegramShop.feature1Desc')}
                        </Paragraph>
                      </div>

                      <div>
                        <Text strong style={{ color: '#10b981' }}>{t('pages.telegramShop.feature2Title')}</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          {t('pages.telegramShop.feature2Desc')}
                        </Paragraph>
                      </div>

                      <div>
                        <Text strong style={{ color: '#8b5cf6' }}>{t('pages.telegramShop.feature3Title')}</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          {t('pages.telegramShop.feature3Desc')}
                        </Paragraph>
                      </div>

                      <div>
                        <Text strong style={{ color: '#ec4899' }}>{t('pages.telegramShop.feature4Title')}</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          {t('pages.telegramShop.feature4Desc')}
                        </Paragraph>
                      </div>
                    </Space>
                  </Card>
                </Col>
              </Row>
            ) : (
              /* MANAGEMENT VIEW (INSTALLED & ACTIVE) */
              <div>
                <Row gutter={[20, 20]} style={{ marginBottom: 24 }}>
                  <Col xs={24} md={8}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>{t('pages.telegramShop.connectedBot')}</span>}
                        value={status.bot_username ? `@${status.bot_username}` : t('pages.telegramShop.connected')}
                        prefix={<SendOutlined style={{ color: '#06b6d4' }} />}
                        valueStyle={{ color: '#22d3ee', fontSize: 18 }}
                      />
                      {status.bot_username && (
                        <Button
                          type="link"
                          href={`https://t.me/${status.bot_username}`}
                          target="_blank"
                          style={{ padding: 0, marginTop: 8 }}
                        >
                          {t('pages.telegramShop.openInTelegram')}
                        </Button>
                      )}
                    </Card>
                  </Col>

                  <Col xs={12} md={4}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>{t('pages.telegramShop.freeTrials')}</span>}
                        value={status.total_trials}
                        prefix={<CheckCircleOutlined style={{ color: '#10b981' }} />}
                        valueStyle={{ color: '#10b981' }}
                      />
                    </Card>
                  </Col>

                  <Col xs={12} md={4}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>{t('pages.telegramShop.ordersPlaced')}</span>}
                        value={status.total_orders}
                        prefix={<ShoppingCartOutlined style={{ color: '#ec4899' }} />}
                        valueStyle={{ color: '#ec4899' }}
                      />
                    </Card>
                  </Col>

                  <Col xs={24} md={8}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>{t('pages.telegramShop.connectedAdminId')}</span>}
                        value={status.admin_chat_id || t('pages.telegramShop.unspecified')}
                        prefix={<SafetyCertificateOutlined style={{ color: '#8b5cf6' }} />}
                        valueStyle={{ color: '#818cf8', fontSize: 18 }}
                      />
                    </Card>
                  </Col>
                </Row>

                {/* ACTION & CONTROLS */}
                <Card
                  className="tgshop-card"
                  title={<span style={{ color: '#fff' }}>{t('pages.telegramShop.actionsTitle')}</span>}
                  style={{ marginBottom: 24 }}
                >
                  <Space wrap size="middle">
                    <Button
                      type="primary"
                      href={status?.domain ? `https://${status.domain}/panel/` : '/panel/'}
                      target="_blank"
                      icon={<GlobalOutlined />}
                      style={{ background: '#6366f1', borderColor: '#6366f1' }}
                    >
                      {t('pages.telegramShop.mirzaAdminBtn')}
                    </Button>

                    <Button
                      icon={<ReloadOutlined />}
                      onClick={() => handleAction('restart')}
                      loading={actionLoading}
                    >
                      {t('pages.telegramShop.restartBtn')}
                    </Button>

                    <Button
                      icon={<SettingOutlined />}
                      loading={installingDeps}
                      onClick={handleInstallDeps}
                      style={{ borderColor: 'rgba(6, 182, 212, 0.4)', color: '#06b6d4' }}
                    >
                      {t('pages.telegramShop.installDepsBtn')}
                    </Button>

                    {status.running ? (
                      <Button
                        danger
                        icon={<PauseCircleOutlined />}
                        onClick={() => handleAction('stop')}
                        loading={actionLoading}
                      >
                        {t('pages.telegramShop.stopBtn')}
                      </Button>
                    ) : (
                      <Button
                        type="primary"
                        icon={<PlayCircleOutlined />}
                        onClick={() => handleAction('start')}
                        loading={actionLoading}
                        style={{ background: '#10b981', borderColor: '#10b981' }}
                      >
                        {t('pages.telegramShop.startBtn')}
                      </Button>
                    )}

                    <Popconfirm
                      title={t('pages.telegramShop.uninstallConfirmTitle')}
                      description={t('pages.telegramShop.uninstallConfirmDesc')}
                      onConfirm={() => handleAction('uninstall')}
                      okText={t('pages.telegramShop.yesDelete')}
                      cancelText={t('cancel')}
                      okButtonProps={{ danger: true }}
                    >
                      <Button danger icon={<DeleteOutlined />} loading={actionLoading}>
                        {t('pages.telegramShop.uninstallBtn')}
                      </Button>
                    </Popconfirm>
                  </Space>
                </Card>

                {/* CONFIG DETAILS */}
                <Card
                  className="tgshop-card"
                  title={<span style={{ color: '#fff' }}>{t('pages.telegramShop.configTitle')}</span>}
                >
                  <Form form={form} layout="vertical" onFinish={handleInstall}>
                    <Row gutter={16}>
                      <Col xs={24} md={12}>
                        <Form.Item
                          name="bot_token"
                          label={<span style={{ color: '#e2e8f0' }}>{t('pages.telegramShop.botTokenLabel')}</span>}
                          rules={[{ required: true }]}
                        >
                          <Input prefix={<SendOutlined />} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12}>
                        <Form.Item
                          name="admin_chat_id"
                          label={<span style={{ color: '#e2e8f0' }}>{t('pages.telegramShop.adminChatIdLabel')}</span>}
                          rules={[{ required: true }]}
                        >
                          <Input prefix={<UserOutlined />} />
                        </Form.Item>
                      </Col>
                    </Row>

                    <Button type="primary" htmlType="submit" loading={installing} className="tgshop-btn-install">
                      {t('pages.telegramShop.saveChanges')}
                    </Button>
                  </Form>
                </Card>
              </div>
            )}
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
