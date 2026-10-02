import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Layout,
  Modal,
  Radio,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';
import {
  CheckCircleOutlined,
  DownloadOutlined,
  GatewayOutlined,
  NodeIndexOutlined,
  ReloadOutlined,
  SendOutlined,
  SwapOutlined,
  SyncOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { useHashemQuery } from '@/api/queries/useHashemQuery';
import { useHashemMutations, type HashemSetupPayload } from '@/api/queries/useHashemMutations';
import './HashemPage.css';

const { Title, Text, Paragraph } = Typography;

export default function HashemPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { status, loading, refetch } = useHashemQuery();
  const {
    setCarrier,
    isSettingCarrier,
    restart,
    isRestarting,
    setWatchdog,
    isSettingWatchdog,
    syncInbounds,
    isSyncingInbounds,
    setup,
    isSettingUp,
    install,
    isInstalling,
  } = useHashemMutations();

  const [setupModalOpen, setSetupModalOpen] = useState(false);
  const [form] = Form.useForm<HashemSetupPayload>();

  const handleCarrierChange = async (carrier: string) => {
    await setCarrier(carrier);
  };

  const handleWatchdogChange = async (enabled: boolean) => {
    await setWatchdog(enabled);
  };

  const handleSyncInbounds = async () => {
    await syncInbounds();
  };

  const handleRestart = async () => {
    await restart();
  };

  const handleInstall = async () => {
    await install();
  };

  const handleSetupSubmit = async (values: HashemSetupPayload) => {
    await setup(values);
    setSetupModalOpen(false);
    form.resetFields();
  };

  const pageClass = useMemo(() => {
    const classes = ['hashem-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const isHealthy = status.running && status.frpStatus === 'active' && status.pingMs >= 0;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            {/* Top Hero Glass Bar */}
            <div className="hashem-hero">
              <div>
                <Title level={2} className="hashem-hero-title">
                  <ThunderboltOutlined style={{ color: '#00f2fe', filter: 'drop-shadow(0 0 8px rgba(0,242,254,0.6))' }} />
                  <span className="hashem-hero-title-text">{t('pages.hashem.title')}</span>
                  {status.installed ? (
                    <Tag
                      color={isHealthy ? 'success' : 'warning'}
                      style={{ marginLeft: 8, fontSize: 13, padding: '2px 10px', borderRadius: 12 }}
                    >
                      {isHealthy ? t('pages.hashem.statusHealthy') : t('pages.hashem.statusDegraded')}
                    </Tag>
                  ) : (
                    <Tag color="error" style={{ marginLeft: 8, borderRadius: 12 }}>
                      {t('pages.hashem.statusNotInstalled')}
                    </Tag>
                  )}
                </Title>
                <Text style={{ color: 'var(--nc-text-2, #94a3b8)', marginTop: 4, display: 'inline-block' }}>
                  {t('pages.hashem.subtitle')}
                </Text>
              </div>

              <Space wrap>
                <Button icon={<ReloadOutlined />} onClick={() => refetch()} loading={loading}>
                  {t('refresh')}
                </Button>
                {status.installed && (
                  <>
                    <Button
                      icon={<SyncOutlined />}
                      onClick={handleSyncInbounds}
                      loading={isSyncingInbounds}
                      className="hashem-btn-sync"
                    >
                      {t('pages.hashem.syncInboundsBtn')}
                    </Button>
                    <Button icon={<SwapOutlined />} onClick={handleRestart} loading={isRestarting}>
                      {t('pages.hashem.restartBtn')}
                    </Button>
                  </>
                )}
                <Button
                  icon={<NodeIndexOutlined />}
                  onClick={() => setSetupModalOpen(true)}
                  className="hashem-btn-setup"
                >
                  {t('pages.hashem.setupBtn')}
                </Button>
              </Space>
            </div>

            <Spin spinning={loading && !status.installed} delay={200} size="large">
              {!status.installed && (
                <div style={{ marginBottom: 20 }}>
                  <Alert
                    className="hashem-alert"
                    message={t('pages.hashem.notInstalledTitle')}
                    description={t('pages.hashem.notInstalledDesc')}
                    type="warning"
                    showIcon
                    action={
                      <Button
                        type="primary"
                        danger
                        icon={<DownloadOutlined />}
                        onClick={handleInstall}
                        loading={isInstalling}
                      >
                        {t('pages.hashem.installNowBtn')}
                      </Button>
                    }
                  />
                </div>
              )}

              {/* Status Metric Grid */}
              <Row gutter={[16, 16]} style={{ marginBottom: 20 }}>
                {/* Layer 3 GRE Interface */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        <GatewayOutlined style={{ color: '#00f2fe' }} />
                        <span>{t('pages.hashem.cardGreTitle')}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 14 }}>
                      <Badge
                        status={status.running ? 'processing' : 'error'}
                        text={
                          <Text strong style={{ color: status.running ? '#38bdf8' : '#ff4d4f' }}>
                            {status.running ? t('pages.hashem.greInterfaceUp') : t('pages.hashem.greInterfaceDown')}
                          </Text>
                        }
                      />
                    </div>
                    <div style={{ fontSize: 13, display: 'flex', flexDirection: 'column', gap: 6 }}>
                      <div>
                        <Text type="secondary">{t('pages.hashem.localGreIp')}: </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.localGreIP || '-'}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t('pages.hashem.peerGreIp')}: </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.remoteGreIP || '-'}
                        </Text>
                      </div>
                    </div>
                  </Card>
                </Col>

                {/* Carrier Mode */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        <ThunderboltOutlined style={{ color: '#faad14' }} />
                        <span>{t('pages.hashem.cardCarrierTitle')}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 14 }}>
                      <Tag color="cyan" style={{ fontSize: 13, padding: '3px 10px', borderRadius: 8 }}>
                        {status.activeCarrier || status.carrier || t('pages.hashem.unknown')}
                      </Tag>
                    </div>
                    <Select
                      value={status.carrier || 'direct'}
                      style={{ width: '100%' }}
                      onChange={handleCarrierChange}
                      loading={isSettingCarrier}
                      options={status.candidates.map((c) => ({
                        label: c === 'fou:443' ? `${c} (FoU anti-filter)` : c,
                        value: c,
                      }))}
                    />
                  </Card>
                </Col>

                {/* Latency & Ping */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        <CheckCircleOutlined style={{ color: '#52c41a' }} />
                        <span>{t('pages.hashem.cardLatencyTitle')}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 10 }}>
                      {status.pingMs >= 0 ? (
                        <Title
                          level={3}
                          style={{
                            margin: 0,
                            color: status.pingMs < 120 ? '#52c41a' : '#faad14',
                            textShadow: '0 0 10px rgba(82, 196, 26, 0.4)',
                          }}
                        >
                          {status.pingMs.toFixed(1)} <span style={{ fontSize: 14 }}>ms</span>
                        </Title>
                      ) : (
                        <Title level={3} style={{ margin: 0, color: '#ff4d4f' }}>
                          {t('pages.hashem.unreachable')}
                        </Title>
                      )}
                    </div>
                    <Text type="secondary">{t('pages.hashem.testedAgainst', { ip: status.remoteGreIP || 'Peer' })}</Text>
                  </Card>
                </Col>

                {/* FRP Reverse Tunnel */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        <SendOutlined style={{ color: '#a855f7' }} />
                        <span>{t('pages.hashem.cardFrpTitle')}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 14 }}>
                      <Badge
                        status={status.frpStatus === 'active' ? 'success' : 'error'}
                        text={
                          <Text strong style={{ color: status.frpStatus === 'active' ? '#a855f7' : '#ff4d4f' }}>
                            {status.frpStatus === 'active' ? t('pages.hashem.frpActive') : t('pages.hashem.frpInactive')}
                          </Text>
                        }
                      />
                    </div>
                    <div style={{ fontSize: 13, display: 'flex', flexDirection: 'column', gap: 6 }}>
                      <div>
                        <Text type="secondary">{t('pages.hashem.frpControlPort')}: </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.frpPort || '-'}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t('pages.hashem.role')}: </Text>
                        <Tag color="purple">{status.role || 'none'}</Tag>
                      </div>
                    </div>
                  </Card>
                </Col>
              </Row>

              {/* Lower Section: Peer Sync & Automation */}
              <Row gutter={[16, 16]}>
                <Col xs={24} lg={16}>
                  <Card className="hashem-card" title={t('pages.hashem.peerSyncTitle')}>
                    <Paragraph style={{ color: 'var(--nc-text-2, #94a3b8)' }}>
                      {t('pages.hashem.peerSyncDesc')}
                    </Paragraph>

                    {status.setupCommand ? (
                      <div style={{ marginTop: 16 }}>
                        <Text strong style={{ display: 'block', marginBottom: 8, color: '#38bdf8' }}>
                          {t('pages.hashem.oneLinerIranCommand')}:
                        </Text>
                        <div className="hashem-code-snippet">
                          <Text copyable={{ text: status.setupCommand }} style={{ color: '#00f2fe' }}>
                            {status.setupCommand}
                          </Text>
                        </div>
                      </div>
                    ) : null}

                    {status.bundle ? (
                      <div style={{ marginTop: 16 }}>
                        <Text strong style={{ display: 'block', marginBottom: 8, color: '#38bdf8' }}>
                          {t('pages.hashem.bundleString')}:
                        </Text>
                        <div className="hashem-code-snippet">
                          <Text copyable={{ text: status.bundle }} style={{ color: '#00f2fe' }}>
                            {status.bundle}
                          </Text>
                        </div>
                      </div>
                    ) : null}

                    <Divider style={{ borderColor: 'rgba(255,255,255,0.08)' }} />

                    <div>
                      <Text strong style={{ color: 'var(--nc-text, #f1f5f9)' }}>
                        {t('pages.hashem.forwardedPortsList')}:{' '}
                      </Text>
                      <div style={{ marginTop: 10 }}>
                        {status.ports && status.ports.length > 0 ? (
                          status.ports.map((p) => (
                            <Tag
                              key={p}
                              color="blue"
                              style={{ fontSize: 13, padding: '3px 10px', borderRadius: 8, marginBottom: 6 }}
                            >
                              Port {p}
                            </Tag>
                          ))
                        ) : (
                          <Text type="secondary">{t('pages.hashem.noPortsForwarded')}</Text>
                        )}
                      </div>
                    </div>
                  </Card>
                </Col>

                <Col xs={24} lg={8}>
                  <Card className="hashem-card" title={t('pages.hashem.automationTitle')}>
                    <Descriptions column={1} size="small" bordered className="hashem-desc-table">
                      <Descriptions.Item label={t('pages.hashem.watchdogAutoRecovery')}>
                        <Switch
                          checked={status.watchdogEnabled}
                          onChange={handleWatchdogChange}
                          loading={isSettingWatchdog}
                        />
                      </Descriptions.Item>
                      <Descriptions.Item label={t('pages.hashem.localPublicIp')}>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.localPubIP || '-'}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label={t('pages.hashem.remotePublicIp')}>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.remotePubIP || '-'}
                        </Text>
                      </Descriptions.Item>
                    </Descriptions>

                    <div style={{ marginTop: 18 }}>
                      <Alert
                        className="hashem-alert"
                        message={t('pages.hashem.tipTitle')}
                        description={t('pages.hashem.tipDesc')}
                        type="info"
                        showIcon
                      />
                    </div>
                  </Card>
                </Col>
              </Row>
            </Spin>
          </Layout.Content>
        </Layout>

        {/* Setup Tunnel Modal */}
        <Modal
          title={t('pages.hashem.setupModalTitle')}
          open={setupModalOpen}
          onCancel={() => setSetupModalOpen(false)}
          footer={null}
          destroyOnClose
          wrapClassName="hashem-modal"
        >
          <Form
            form={form}
            layout="vertical"
            initialValues={{
              role: 'foreign',
              carrier: 'fou:443',
              frpPort: 36067,
              ports: status.ports?.join(', ') || '443, 8080, 2053',
            }}
            onFinish={handleSetupSubmit}
          >
            <Form.Item name="role" label={t('pages.hashem.serverRole')} rules={[{ required: true }]}>
              <Radio.Group buttonStyle="solid">
                <Radio.Button value="foreign">{t('pages.hashem.roleForeign')}</Radio.Button>
                <Radio.Button value="iran">{t('pages.hashem.roleIran')}</Radio.Button>
              </Radio.Group>
            </Form.Item>

            <Form.Item
              name="remotePub"
              label={t('pages.hashem.peerPublicIp')}
              rules={[{ required: true, message: t('pages.hashem.remotePubRequired') }]}
            >
              <Input placeholder="e.g. 77.237.90.175 or 94.183.210.29" />
            </Form.Item>

            <Form.Item name="localPub" label={t('pages.hashem.thisServerPublicIp')}>
              <Input placeholder="Auto-detected if blank" />
            </Form.Item>

            <Form.Item name="frpPort" label={t('pages.hashem.frpPortLabel')}>
              <InputNumber style={{ width: '100%' }} min={1000} max={65535} />
            </Form.Item>

            <Form.Item name="carrier" label={t('pages.hashem.carrierModeLabel')}>
              <Select
                options={[
                  { label: 'FoU:443 (Recommended for bypassing filtering)', value: 'fou:443' },
                  { label: 'Direct GRE (Raw IP proto 47)', value: 'direct' },
                  { label: 'WSS:8443 (WebSocket TLS)', value: 'wss:8443' },
                ]}
              />
            </Form.Item>

            <Form.Item name="ports" label={t('pages.hashem.portsLabel')} extra={t('pages.hashem.portsExtra')}>
              <Input placeholder="443, 8080, 2053" />
            </Form.Item>

            <Form.Item
              name="bundle"
              label={t('pages.hashem.bundleImportLabel')}
              extra={t('pages.hashem.bundleImportExtra')}
            >
              <Input placeholder="hsh1_..." />
            </Form.Item>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 24 }}>
              <Button onClick={() => setSetupModalOpen(false)}>{t('cancel')}</Button>
              <Button type="primary" htmlType="submit" loading={isSettingUp} className="hashem-btn-setup">
                {t('pages.hashem.applyTunnelBtn')}
              </Button>
            </div>
          </Form>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
