import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  CopyOutlined,
  DownloadOutlined,
  ExclamationCircleOutlined,
  GatewayOutlined,
  NodeIndexOutlined,
  ReloadOutlined,
  SendOutlined,
  SwapOutlined,
  SyncOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import { useHashemQuery } from '@/api/queries/useHashemQuery';
import { useHashemMutations, type HashemSetupPayload } from '@/api/queries/useHashemMutations';

const { Title, Text, Paragraph } = Typography;

export default function HashemPage() {
  const { t } = useTranslation();
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

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    message.success(t('pages.hashem.toasts.copied', { label }));
  };

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

  if (loading && !status.installed) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  const isHealthy = status.running && status.frpStatus === 'active' && status.pingMs >= 0;

  return (
    <div style={{ padding: '20px 24px', maxWidth: 1400, margin: '0 auto' }}>
      {/* Top Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24, flexWrap: 'wrap', gap: 16 }}>
        <div>
          <Title level={2} style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 12 }}>
            <ThunderboltOutlined style={{ color: '#00f2fe' }} />
            {t('pages.hashem.title')}
            {status.installed ? (
              <Tag color={isHealthy ? 'success' : 'warning'} style={{ marginLeft: 8, fontSize: 13, padding: '2px 10px' }}>
                {isHealthy ? t('pages.hashem.statusHealthy') : t('pages.hashem.statusDegraded')}
              </Tag>
            ) : (
              <Tag color="error" style={{ marginLeft: 8 }}>{t('pages.hashem.statusNotInstalled')}</Tag>
            )}
          </Title>
          <Text type="secondary">{t('pages.hashem.subtitle')}</Text>
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
                type="primary"
                style={{ background: 'linear-gradient(135deg, #00c6ff, #0072ff)', borderColor: 'transparent' }}
              >
                {t('pages.hashem.syncInboundsBtn')}
              </Button>
              <Button icon={<SwapOutlined />} onClick={handleRestart} loading={isRestarting}>
                {t('pages.hashem.restartBtn')}
              </Button>
            </>
          )}
          <Button
            type="primary"
            icon={<NodeIndexOutlined />}
            onClick={() => setSetupModalOpen(true)}
            style={{ background: 'linear-gradient(135deg, #f857a6, #ff5858)', borderColor: 'transparent' }}
          >
            {t('pages.hashem.setupBtn')}
          </Button>
        </Space>
      </div>

      {!status.installed && (
        <Card style={{ marginBottom: 24, borderColor: '#ff4d4f' }}>
          <Alert
            message={t('pages.hashem.notInstalledTitle')}
            description={t('pages.hashem.notInstalledDesc')}
            type="warning"
            showIcon
            action={
              <Button type="primary" danger icon={<DownloadOutlined />} onClick={handleInstall} loading={isInstalling}>
                {t('pages.hashem.installNowBtn')}
              </Button>
            }
          />
        </Card>
      )}

      {/* Metric Cards Row */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        {/* Layer 3 GRE Interface */}
        <Col xs={24} sm={12} lg={6}>
          <Card
            title={
              <Space>
                <GatewayOutlined style={{ color: '#00f2fe' }} />
                <span>{t('pages.hashem.cardGreTitle')}</span>
              </Space>
            }
            bordered
          >
            <div style={{ marginBottom: 12 }}>
              <Badge
                status={status.running ? 'processing' : 'error'}
                text={
                  <Text strong>
                    {status.running ? t('pages.hashem.greInterfaceUp') : t('pages.hashem.greInterfaceDown')}
                  </Text>
                }
              />
            </div>
            <div style={{ fontSize: 13 }}>
              <div><Text type="secondary">{t('pages.hashem.localGreIp')}: </Text><Text code>{status.localGreIP || '-'}</Text></div>
              <div><Text type="secondary">{t('pages.hashem.peerGreIp')}: </Text><Text code>{status.remoteGreIP || '-'}</Text></div>
            </div>
          </Card>
        </Col>

        {/* Carrier Mode */}
        <Col xs={24} sm={12} lg={6}>
          <Card
            title={
              <Space>
                <ThunderboltOutlined style={{ color: '#faad14' }} />
                <span>{t('pages.hashem.cardCarrierTitle')}</span>
              </Space>
            }
            bordered
          >
            <div style={{ marginBottom: 12 }}>
              <Tag color="cyan" style={{ fontSize: 14, padding: '4px 10px' }}>
                {status.activeCarrier || status.carrier || t('pages.hashem.unknown')}
              </Tag>
            </div>
            <Space direction="vertical" style={{ width: '100%' }}>
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
            </Space>
          </Card>
        </Col>

        {/* Latency & Ping */}
        <Col xs={24} sm={12} lg={6}>
          <Card
            title={
              <Space>
                <CheckCircleOutlined style={{ color: '#52c41a' }} />
                <span>{t('pages.hashem.cardLatencyTitle')}</span>
              </Space>
            }
            bordered
          >
            <div style={{ marginBottom: 8 }}>
              {status.pingMs >= 0 ? (
                <Title level={3} style={{ margin: 0, color: status.pingMs < 120 ? '#52c41a' : '#faad14' }}>
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

        {/* Reverse FRP Proxy */}
        <Col xs={24} sm={12} lg={6}>
          <Card
            title={
              <Space>
                <SendOutlined style={{ color: '#722ed1' }} />
                <span>{t('pages.hashem.cardFrpTitle')}</span>
              </Space>
            }
            bordered
          >
            <div style={{ marginBottom: 12 }}>
              <Badge
                status={status.frpStatus === 'active' ? 'success' : 'error'}
                text={
                  <Text strong>
                    {status.frpStatus === 'active' ? t('pages.hashem.frpActive') : t('pages.hashem.frpInactive')}
                  </Text>
                }
              />
            </div>
            <div style={{ fontSize: 13 }}>
              <div><Text type="secondary">{t('pages.hashem.frpControlPort')}: </Text><Text code>{status.frpPort || '-'}</Text></div>
              <div><Text type="secondary">{t('pages.hashem.role')}: </Text><Tag>{status.role || 'none'}</Tag></div>
            </div>
          </Card>
        </Col>
      </Row>

      {/* Main Details and Peer Setup Card */}
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={16}>
          <Card title={t('pages.hashem.peerSyncTitle')} style={{ marginBottom: 16 }}>
            <Paragraph>{t('pages.hashem.peerSyncDesc')}</Paragraph>

            {status.setupCommand ? (
              <div style={{ marginTop: 16 }}>
                <Text strong style={{ display: 'block', marginBottom: 6 }}>
                  {t('pages.hashem.oneLinerIranCommand')}:
                </Text>
                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    border: '1px solid rgba(255,255,255,0.1)',
                    borderRadius: 8,
                    padding: 12,
                    position: 'relative',
                    wordBreak: 'break-all',
                    fontFamily: 'monospace',
                  }}
                >
                  <Text copyable={{ text: status.setupCommand }}>{status.setupCommand}</Text>
                </div>
              </div>
            ) : null}

            {status.bundle ? (
              <div style={{ marginTop: 16 }}>
                <Text strong style={{ display: 'block', marginBottom: 6 }}>
                  {t('pages.hashem.bundleString')}:
                </Text>
                <div
                  style={{
                    background: 'rgba(0,0,0,0.3)',
                    border: '1px solid rgba(255,255,255,0.1)',
                    borderRadius: 8,
                    padding: 12,
                    position: 'relative',
                    wordBreak: 'break-all',
                    fontFamily: 'monospace',
                  }}
                >
                  <Text copyable={{ text: status.bundle }}>{status.bundle}</Text>
                </div>
              </div>
            ) : null}

            <Divider />

            <div>
              <Text strong>{t('pages.hashem.forwardedPortsList')}: </Text>
              <div style={{ marginTop: 8 }}>
                {status.ports && status.ports.length > 0 ? (
                  status.ports.map((p) => (
                    <Tag key={p} color="blue" style={{ fontSize: 14, padding: '2px 8px', marginBottom: 6 }}>
                      {p}
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
          <Card title={t('pages.hashem.automationTitle')} style={{ marginBottom: 16 }}>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label={t('pages.hashem.watchdogAutoRecovery')}>
                <Switch
                  checked={status.watchdogEnabled}
                  onChange={handleWatchdogChange}
                  loading={isSettingWatchdog}
                />
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.hashem.localPublicIp')}>
                <Text code>{status.localPubIP || '-'}</Text>
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.hashem.remotePublicIp')}>
                <Text code>{status.remotePubIP || '-'}</Text>
              </Descriptions.Item>
            </Descriptions>

            <div style={{ marginTop: 16 }}>
              <Alert
                message={t('pages.hashem.tipTitle')}
                description={t('pages.hashem.tipDesc')}
                type="info"
                showIcon
              />
            </div>
          </Card>
        </Col>
      </Row>

      {/* Setup Tunnel Modal */}
      <Modal
        title={t('pages.hashem.setupModalTitle')}
        open={setupModalOpen}
        onCancel={() => setSetupModalOpen(false)}
        footer={null}
        destroyOnClose
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

          <Form.Item name="bundle" label={t('pages.hashem.bundleImportLabel')} extra={t('pages.hashem.bundleImportExtra')}>
            <Input placeholder="hsh1_..." />
          </Form.Item>

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 24 }}>
            <Button onClick={() => setSetupModalOpen(false)}>{t('cancel')}</Button>
            <Button type="primary" htmlType="submit" loading={isSettingUp}>
              {t('pages.hashem.applyTunnelBtn')}
            </Button>
          </div>
        </Form>
      </Modal>
    </div>
  );
}
