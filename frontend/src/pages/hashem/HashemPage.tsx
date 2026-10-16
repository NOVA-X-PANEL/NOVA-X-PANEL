import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  Col,
  ConfigProvider,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Layout,
  message,
  Modal,
  Popconfirm,
  Radio,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import {
  CodeOutlined,
  CopyOutlined,
  DashboardOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  GatewayOutlined,
  NodeIndexOutlined,
  PlusOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  SwapOutlined,
  SyncOutlined,
  ThunderboltOutlined,
  WifiOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import { useHashemQuery, type CarrierMetric } from '@/api/queries/useHashemQuery';
import { useHashemMutations, type HashemSetupPayload, type HashemOneLinerPayload } from '@/api/queries/useHashemMutations';
import './HashemPage.css';

const { Title, Text, Paragraph } = Typography;

const getBackhaulTransportOptions = (t: TFunction) => [
  { label: t('pages.hashem.transports.tcpmux', { defaultValue: 'TCPMux (Multi-Channel High-Speed - Recommended)' }), value: 'tcpmux' },
  { label: t('pages.hashem.transports.xtcpmux', { defaultValue: 'X-TCPMux (Anti-Throttling Enhanced)' }), value: 'xtcpmux' },
  { label: t('pages.hashem.transports.anytls', { defaultValue: 'AnyTLS (Full Anti-DPI Obfuscation)' }), value: 'anytls' },
  { label: t('pages.hashem.transports.wssmux', { defaultValue: 'WSSMux (Secure WebSocket + TLS)' }), value: 'wssmux' },
  { label: t('pages.hashem.transports.ws', { defaultValue: 'WS (Standard WebSocket without TLS)' }), value: 'ws' },
  { label: t('pages.hashem.transports.tcp', { defaultValue: 'TCP / TCPO (Direct Single-Connection)' }), value: 'tcp' },
  { label: t('pages.hashem.transports.tun', { defaultValue: 'TUN (L3 Network Interface Tunnel)' }), value: 'tun' },
];

export default function HashemPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { status, loading, refetch } = useHashemQuery();
  const backhaulTransportOptions = useMemo(() => getBackhaulTransportOptions(t), [t]);
  const {
    setCarrier,
    isSettingCarrier,
    restart,
    isRestarting,
    setWatchdog,
    isSettingWatchdog,
    syncInbounds,
    isSyncingInbounds,
    editPorts,
    isEditingPorts,
    setup,
    isSettingUp,
    generateOneLiner,
    isGeneratingOneLiner,
    install,
    isInstalling,
    removeTunnel,
    isRemovingTunnel,
    runBenchmark,
    isRunningBenchmark,
    setAutoPilot,
    isSettingAutoPilot,
    autoCreateInbound,
    isAutoCreatingInbound,
  } = useHashemMutations();

  const [setupModalOpen, setSetupModalOpen] = useState(false);
  const [setupTab, setSetupTab] = useState<'oneliner' | 'advanced'>('oneliner');
  const [generatedCmd, setGeneratedCmd] = useState<string | null>(null);

  const [editPortsModalOpen, setEditPortsModalOpen] = useState(false);
  const [portChips, setPortChips] = useState<number[]>([]);
  const [newPortInput, setNewPortInput] = useState<number | null>(null);

  const [oneLinerForm] = Form.useForm();
  const [form] = Form.useForm<HashemSetupPayload>();

  const oneLinerEngine = Form.useWatch('engine', oneLinerForm) || 'tunnel-panel';
  const advancedEngine = Form.useWatch('engine', form) || 'tunnel-panel';

  const handleRemove = async () => {
    try {
      await removeTunnel();
      refetch();
    } catch {
      // toast shown by mutation
    }
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

  const handleOpenEditPorts = () => {
    setPortChips(status.ports ? [...status.ports] : []);
    setNewPortInput(null);
    setEditPortsModalOpen(true);
  };

  const handleAddPortChip = () => {
    if (!newPortInput || newPortInput < 1 || newPortInput > 65535) {
      message.warning(t('pages.hashem.portMustBeValid', { defaultValue: 'Port number must be between 1 and 65535' }));
      return;
    }
    if (portChips.includes(newPortInput)) {
      message.warning(t('pages.hashem.portAlreadyExists', { defaultValue: 'This port already exists in the list' }));
      return;
    }
    setPortChips([...portChips, newPortInput].sort((a, b) => a - b));
    setNewPortInput(null);
  };

  const handleRemovePortChip = (portToRemove: number) => {
    if (portChips.length <= 1) {
      message.warning(t('pages.hashem.atLeastOnePortRequired', { defaultValue: 'At least one port must remain in the list' }));
      return;
    }
    setPortChips(portChips.filter((p: number) => p !== portToRemove));
  };

  const handleSavePorts = async () => {
    if (portChips.length === 0) {
      message.warning(t('pages.hashem.atLeastOnePortRequired', { defaultValue: 'At least one port must remain in the list' }));
      return;
    }
    try {
      await editPorts(portChips);
      setEditPortsModalOpen(false);
      refetch();
    } catch {
      // toast shown by mutation
    }
  };

  const handleRestart = async () => {
    await restart();
  };

  const handleInstall = async () => {
    await install();
  };

  const handleOneLinerSubmit = async (values: HashemOneLinerPayload) => {
    try {
      const res = await generateOneLiner({
        engine: values.engine || 'tunnel-panel',
        transport: values.transport || 'tcpmux',
        backhaulPort: values.backhaulPort || 3080,
        snappy: values.snappy ?? true,
        iranIp: values.iranIp,
        ports: values.ports || status.ports?.join(', ') || '8080',
        carrier: values.carrier || 'direct',
        autoCreateInbound: values.autoCreateInbound ?? true,
        inboundHost: values.inboundHost || values.iranIp,
      });
      if (res?.oneLinerCommand) {
        setGeneratedCmd(res.oneLinerCommand);
      }
    } catch {
      // toast shown by mutation
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    message.success(t('pages.hashem.commandCopied'));
  };

  const handleSetupSubmit = async (values: HashemSetupPayload) => {
    await setup({
      ...values,
      autoCreateInbound: values.autoCreateInbound ?? true,
      inboundHost: values.inboundHost || values.remotePub,
    });
    setSetupModalOpen(false);
    form.resetFields();
  };

  const pageClass = useMemo(() => {
    const classes = ['hashem-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const isEngineTunnelPanel = status.engine === 'tunnel-panel';
  const isEngineBackhaul = status.engine === 'backhaul' || status.engine === 'gre-backhaul';
  const isHealthy = status.running && status.pingMs > 0;
  const engineStatus = isEngineTunnelPanel
    ? (isHealthy ? 'active' : 'stopped')
    : isEngineBackhaul
      ? status.backhaulStatus
      : status.frpStatus;

  const isConnecting = engineStatus === 'connecting';

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
                      color={isHealthy ? 'success' : isConnecting ? 'warning' : 'error'}
                      style={{ marginLeft: 8, fontSize: 13, padding: '2px 10px', borderRadius: 12 }}
                    >
                      {isHealthy
                        ? t('pages.hashem.statusHealthy')
                        : isConnecting
                          ? t('pages.hashem.statusConnecting')
                          : t('pages.hashem.statusDisconnected')}
                    </Tag>
                  ) : (
                    <Tag
                      style={{
                        marginLeft: 8,
                        borderRadius: 12,
                        backgroundColor: 'rgba(239, 68, 68, 0.16)',
                        border: '1px solid rgba(239, 68, 68, 0.45)',
                        color: '#f87171',
                        fontWeight: 600,
                        padding: '2px 10px',
                      }}
                    >
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
                      icon={<DownloadOutlined />}
                      onClick={handleInstall}
                      loading={isInstalling}
                      title={t('pages.hashem.updateScriptTooltip', { defaultValue: 'Download and update latest NOVA script and cores from GitHub' })}
                    >
                      {t('pages.hashem.updateScriptBtn', { defaultValue: 'Update NOVA Script' })}
                    </Button>
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
                    <Popconfirm
                      title={t('pages.hashem.removeConfirmTitle')}
                      description={t('pages.hashem.removeConfirmDesc')}
                      onConfirm={handleRemove}
                      okText={t('pages.hashem.btnRemoveTunnel')}
                      cancelText={t('cancel')}
                      okButtonProps={{ danger: true, loading: isRemovingTunnel }}
                    >
                      <Button danger icon={<DeleteOutlined />} loading={isRemovingTunnel}>
                        {t('pages.hashem.btnRemoveTunnel')}
                      </Button>
                    </Popconfirm>
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
                          {status.localGreIp || '-'}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t('pages.hashem.peerGreIp')}: </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.remoteGreIp || '-'}
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
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>{t('pages.hashem.activeMode', { defaultValue: 'Active Mode:' })}</Text>
                      <Tag color="cyan" style={{ fontSize: 12, padding: '2px 8px', borderRadius: 6, margin: 0 }}>
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

                {/* Latency */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        <WifiOutlined style={{ color: status.pingMs > 0 ? '#52c41a' : '#ff4d4f' }} />
                        <span>{t('pages.hashem.cardLatencyTitle')}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 10 }}>
                      {status.pingMs > 0 ? (
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
                    <Text type="secondary">{t('pages.hashem.testedAgainst', { ip: status.remoteGreIp || 'Peer' })}</Text>
                  </Card>
                </Col>

                {/* Tunnel Transport / Engine */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        <SafetyCertificateOutlined style={{ color: '#00f2fe' }} />
                        <span>{t('pages.hashem.cardTunnelPanelTitle', { defaultValue: 'GRETAP Kernel Tunnel' })}</span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 14 }}>
                      <Badge
                        status={isHealthy ? 'success' : 'error'}
                        text={
                          <Text
                            strong
                            style={{
                              color: isHealthy ? '#52c41a' : '#ff4d4f',
                            }}
                          >
                            {isHealthy ? t('pages.hashem.statusRunning', { defaultValue: 'Active (Fast-Path)' }) : t('pages.hashem.frpStopped')}
                          </Text>
                        }
                      />
                    </div>
                    <div style={{ fontSize: 13, display: 'flex', flexDirection: 'column', gap: 6 }}>
                      <div>
                        <Text type="secondary">
                          {t('pages.hashem.tunnelRouting', { defaultValue: 'Routing Engine' })}:{' '}
                        </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          NFTables / Kernel L2 GRETAP
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t('pages.hashem.engineLabel', { defaultValue: 'Engine Type' })}: </Text>
                        <Tag color="cyan">
                          🛡️ GRETAP
                        </Tag>
                      </div>
                    </div>
                  </Card>
                </Col>
              </Row>

              {/* Carrier Benchmark & AutoPilot Card */}
              <Card
                className="hashem-card"
                style={{ marginBottom: 20 }}
                title={
                  <Space wrap>
                    <DashboardOutlined style={{ color: '#00f2fe' }} />
                    <span style={{ fontWeight: 600 }}>
                      {t('pages.hashem.benchmarkCardTitle', { defaultValue: 'Carrier Benchmark & Real-time Metrics' })}
                    </span>
                    {status.benchmark?.bestCarrier && (
                      <Tag color="success" style={{ borderRadius: 6, fontWeight: 600 }}>
                        {t('pages.hashem.bestCarrier', { defaultValue: 'Best Route:' })} {status.benchmark.bestCarrier}
                      </Tag>
                    )}
                    {status.benchmark?.updatedAt && (
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        ({t('pages.hashem.lastUpdated', { defaultValue: 'Last Check:' })} {status.benchmark.updatedAt})
                      </Text>
                    )}
                  </Space>
                }
                extra={
                  <Space wrap>
                    <Space style={{ marginRight: 8 }}>
                      <Text style={{ fontSize: 13, color: 'var(--nc-text-2, #94a3b8)' }}>
                        {t('pages.hashem.autoPilotLabel', { defaultValue: 'Smart Auto-Pilot:' })}
                      </Text>
                      <Switch
                        checked={status.autoPilot}
                        onChange={(checked: boolean) => setAutoPilot(checked)}
                        loading={isSettingAutoPilot}
                        checkedChildren="ON"
                        unCheckedChildren="OFF"
                      />
                    </Space>
                    <Button
                      type="primary"
                      icon={<ThunderboltOutlined />}
                      onClick={() => runBenchmark()}
                      loading={isRunningBenchmark}
                      className="hashem-btn-sync"
                    >
                      {t('pages.hashem.runBenchmarkBtn', { defaultValue: 'Run Live Benchmark' })}
                    </Button>
                  </Space>
                }
              >
                {status.benchmark && status.benchmark.metrics && status.benchmark.metrics.length > 0 ? (
                  <Table
                    rowKey="id"
                    pagination={false}
                    size="small"
                    dataSource={status.benchmark.metrics}
                    columns={[
                      {
                        title: t('pages.hashem.carrierName', { defaultValue: 'Carrier / Route' }),
                        dataIndex: 'name',
                        key: 'name',
                        render: (_: unknown, record: CarrierMetric) => (
                          <Space>
                            <Text strong style={{ color: 'var(--nc-text, #f1f5f9)' }}>
                              {record.name}
                            </Text>
                            {status.activeCarrier === record.id || status.carrier === record.id ? (
                              <Tag color="cyan">{t('pages.hashem.active', { defaultValue: 'Active' })}</Tag>
                            ) : null}
                          </Space>
                        ),
                      },
                      {
                        title: t('pages.hashem.carrierType', { defaultValue: 'Protocol' }),
                        dataIndex: 'type',
                        key: 'type',
                        render: (type: string) => <Tag color="blue">{type}</Tag>,
                      },
                      {
                        title: t('pages.hashem.avgRtt', { defaultValue: 'Latency (RTT)' }),
                        dataIndex: 'avgRttMs',
                        key: 'avgRttMs',
                        render: (ms: number) => (
                          <Text
                            style={{
                              color: ms > 0 && ms < 100 ? '#52c41a' : ms < 200 ? '#faad14' : '#ff4d4f',
                              fontWeight: 600,
                            }}
                          >
                            {ms > 0 ? `${ms.toFixed(1)} ms` : '-'}
                          </Text>
                        ),
                      },
                      {
                        title: t('pages.hashem.packetLoss', { defaultValue: 'Packet Loss' }),
                        dataIndex: 'packetLoss',
                        key: 'packetLoss',
                        render: (loss: number) => (
                          <Text style={{ color: loss === 0 ? '#52c41a' : loss < 10 ? '#faad14' : '#ff4d4f' }}>
                            {loss.toFixed(1)}%
                          </Text>
                        ),
                      },
                      {
                        title: t('pages.hashem.jitter', { defaultValue: 'Jitter' }),
                        dataIndex: 'jitterMs',
                        key: 'jitterMs',
                        render: (jitter: number) => <Text>{jitter > 0 ? `${jitter.toFixed(1)} ms` : '-'}</Text>,
                      },
                      {
                        title: t('pages.hashem.score', { defaultValue: 'Quality Score' }),
                        dataIndex: 'score',
                        key: 'score',
                        render: (score: number) => (
                          <Tag color={score >= 80 ? 'green' : score >= 50 ? 'orange' : 'red'}>
                            {score.toFixed(0)} / 100
                          </Tag>
                        ),
                      },
                      {
                        title: t('pages.hashem.carrierStatus', { defaultValue: 'Status' }),
                        dataIndex: 'status',
                        key: 'status',
                        render: (st: string) => (
                          <Badge
                            status={st === 'optimal' ? 'success' : st === 'good' ? 'processing' : 'error'}
                            text={st === 'optimal' ? t('pages.hashem.optimal', { defaultValue: 'Optimal' }) : st === 'good' ? t('pages.hashem.good', { defaultValue: 'Good' }) : t('pages.hashem.poor', { defaultValue: 'Poor' })}
                          />
                        ),
                      },
                      {
                        title: t('pages.hashem.action', { defaultValue: 'Action' }),
                        key: 'action',
                        render: (_: unknown, record: CarrierMetric) => (
                          <Button
                            size="small"
                            type="dashed"
                            disabled={status.activeCarrier === record.id || status.carrier === record.id}
                            loading={isSettingCarrier}
                            onClick={() => handleCarrierChange(record.id)}
                          >
                            {t('pages.hashem.switchToCarrier', { defaultValue: 'Select Route' })}
                          </Button>
                        ),
                      },
                    ]}
                  />
                ) : (
                  <div style={{ textAlign: 'center', padding: '24px 0' }}>
                    <Text type="secondary">
                      {t('pages.hashem.noBenchmarkYet', {
                        defaultValue: 'No benchmark recorded yet. Click "Run Live Benchmark" to evaluate carrier latency.',
                      })}
                    </Text>
                  </div>
                )}
              </Card>

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
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                        <Text strong style={{ color: 'var(--nc-text, #f1f5f9)' }}>
                          {t('pages.hashem.forwardedPortsList')}:{' '}
                        </Text>
                        <Space>
                          <Button
                            size="small"
                            type="dashed"
                            icon={<ThunderboltOutlined />}
                            onClick={() =>
                              autoCreateInbound({
                                ports: status.ports?.join(', '),
                                host: status.remotePubIp,
                              })
                            }
                            loading={isAutoCreatingInbound}
                            style={{ borderRadius: 6, fontSize: 12, borderColor: '#00f2fe', color: '#00f2fe' }}
                          >
                            {t('pages.hashem.autoCreateInboundBtn', { defaultValue: 'Auto-Create VLESS-WS Inbound' })}
                          </Button>
                          <Button
                            size="small"
                            type="primary"
                            ghost
                            icon={<EditOutlined />}
                            onClick={handleOpenEditPorts}
                            style={{ borderRadius: 6, fontSize: 12 }}
                          >
                            {t('pages.hashem.editPortsBtn', { defaultValue: 'Edit Ports' })}
                          </Button>
                        </Space>
                      </div>
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
                          {status.localPubIp || '-'}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label={t('pages.hashem.remotePublicIp')}>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.remotePubIp || '-'}
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
          title={
            <Space>
              <ThunderboltOutlined style={{ color: '#00f2fe' }} />
              <span>{t('pages.hashem.setupModalTitle')}</span>
            </Space>
          }
          open={setupModalOpen}
          onCancel={() => {
            setSetupModalOpen(false);
            setGeneratedCmd(null);
          }}
          footer={null}
          destroyOnClose
          width={640}
          wrapClassName="hashem-modal"
        >
          <Tabs
            activeKey={setupTab}
            onChange={(k) => setSetupTab(k as 'advanced' | 'oneliner')}
            items={[
              {
                key: 'oneliner',
                label: (
                  <Space>
                    <CodeOutlined />
                    <span>{t('pages.hashem.tabOneLiner')}</span>
                  </Space>
                ),
                children: (
                  <Form
                    form={oneLinerForm}
                    layout="vertical"
                    initialValues={{
                      ports: status.ports?.join(', ') || '8080',
                    }}
                    onFinish={handleOneLinerSubmit}
                    style={{ marginTop: 12 }}
                  >
                    <Alert
                      type="info"
                      showIcon
                      message={t('pages.hashem.oneLinerIranCommand', { defaultValue: 'Iran Server One-Liner Command' })}
                      description={t('pages.hashem.oneLinerIranCommandDesc', { defaultValue: 'Enter Iran server IP and desired ports. Foreign server configuration will be applied immediately and a one-liner command will be generated to run on your Iran server.' })}
                      style={{ marginBottom: 16 }}
                    />

                    <Form.Item
                      name="engine"
                      label={t('pages.hashem.engineLabel', { defaultValue: 'Tunnel Engine' })}
                      initialValue="tunnel-panel"
                    >
                      <Radio.Group buttonStyle="solid" style={{ width: '100%' }}>
                        <Radio.Button value="tunnel-panel">{t('pages.hashem.tunnelPanelOption', { defaultValue: '🛡️ GRETAP (کرنل لینوکس + NFTables)' })}</Radio.Button>
                      </Radio.Group>
                    </Form.Item>

                    {oneLinerEngine !== 'frp' && oneLinerEngine !== 'tunnel-panel' && (
                      <Row gutter={16}>
                        <Col span={14}>
                          <Form.Item
                            name="transport"
                            label={t('pages.hashem.transportLabel', { defaultValue: 'Transport Protocol' })}
                            initialValue="tcpmux"
                          >
                            <Select options={backhaulTransportOptions} />
                          </Form.Item>
                        </Col>
                        <Col span={10}>
                          <Form.Item
                            name="backhaulPort"
                            label={t('pages.hashem.backhaulPortLabel', { defaultValue: 'Backhaul Server Port' })}
                            initialValue={3080}
                          >
                            <InputNumber style={{ width: '100%' }} min={1} max={65535} />
                          </Form.Item>
                        </Col>
                      </Row>
                    )}

                    <Form.Item
                      name="iranIp"
                      label={t('pages.hashem.iranIpLabel')}
                      rules={[{ required: true, message: t('pages.hashem.iranIpRequired') }]}
                    >
                      <Input placeholder={t('pages.hashem.iranIpPlaceholder', { defaultValue: 'your iran server ip' })} />
                    </Form.Item>

                    <Form.Item
                      name="ports"
                      label={t('pages.hashem.portsToTunnel')}
                      extra={t('pages.hashem.portsToTunnelExtra')}
                    >
                      <Input placeholder="8080" />
                    </Form.Item>

                    <Form.Item
                      name="autoCreateInbound"
                      valuePropName="checked"
                      initialValue={true}
                      extra={t('pages.hashem.autoCreateInboundExtra', {
                        defaultValue: 'When enabled, creates a matching VLESS-WS inbound for the tunneled port (similar to port 8080) with automatic host header configuration.',
                      })}
                    >
                      <Checkbox style={{ color: '#38bdf8', fontWeight: 600 }}>
                        ⚡ {t('pages.hashem.autoCreateInboundLabel', { defaultValue: 'Auto-create matching VLESS-WS inbound for tunnel port' })}
                      </Checkbox>
                    </Form.Item>

                    <Form.Item
                      name="inboundHost"
                      label={t('pages.hashem.inboundHostLabel', { defaultValue: 'WebSocket Host Header (WS Host Header)' })}
                      extra={t('pages.hashem.inboundHostExtra', {
                        defaultValue: 'Host domain or IP set in inbound WebSocket header (e.g. your iran domain or Iran server IP). If left blank, Iran server IP is used.',
                      })}
                    >
                      <Input placeholder={t('pages.hashem.inboundHostPlaceholder', { defaultValue: 'your iran domain' })} />
                    </Form.Item>

                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 16 }}>
                      <Button
                        type="primary"
                        htmlType="submit"
                        loading={isGeneratingOneLiner}
                        className="hashem-btn-setup"
                        icon={<CodeOutlined />}
                      >
                        {t('pages.hashem.generatingOneLinerBtn')}
                      </Button>
                    </div>

                    {generatedCmd && (
                      <div style={{ marginTop: 20 }}>
                        <Text strong style={{ color: '#00f2fe' }}>
                          {t('pages.hashem.oneLinerModalDesc')}
                        </Text>
                        <div
                          style={{
                            marginTop: 8,
                            padding: 12,
                            background: 'rgba(0, 0, 0, 0.4)',
                            border: '1px solid rgba(0, 242, 254, 0.3)',
                            borderRadius: 8,
                            wordBreak: 'break-all',
                            fontFamily: 'monospace',
                            fontSize: 12,
                            color: '#00f2fe',
                          }}
                        >
                          {generatedCmd}
                        </div>
                        <Button
                          type="primary"
                          icon={<CopyOutlined />}
                          style={{ marginTop: 12, width: '100%' }}
                          onClick={() => copyToClipboard(generatedCmd)}
                        >
                          {t('pages.hashem.copyCommand')}
                        </Button>
                      </div>
                    )}
                  </Form>
                ),
              },
              {
                key: 'advanced',
                label: (
                  <Space>
                    <SettingOutlined />
                    <span>{t('pages.hashem.tabAdvanced')}</span>
                  </Space>
                ),
                children: (
                  <Form
                    form={form}
                    layout="vertical"
                    initialValues={{
                      role: 'foreign',
                      carrier: 'direct',
                      frpPort: 36067,
                      ports: status.ports?.join(', ') || '8080',
                      inboundHost: '',
                      autoCreateInbound: true,
                    }}
                    onFinish={handleSetupSubmit}
                    style={{ marginTop: 12 }}
                  >
                    <Form.Item name="role" label={t('pages.hashem.serverRole')} rules={[{ required: true }]}>
                      <Radio.Group buttonStyle="solid">
                        <Radio.Button value="foreign">{t('pages.hashem.roleForeign')}</Radio.Button>
                        <Radio.Button value="iran">{t('pages.hashem.roleIran')}</Radio.Button>
                      </Radio.Group>
                    </Form.Item>

                    <Form.Item
                      name="engine"
                      label={t('pages.hashem.engineLabel', { defaultValue: 'Tunnel Engine' })}
                      initialValue="tunnel-panel"
                    >
                      <Radio.Group buttonStyle="solid" style={{ width: '100%' }}>
                        <Radio.Button value="tunnel-panel">{t('pages.hashem.tunnelPanelOption', { defaultValue: '🛡️ GRETAP (کرنل لینوکس + NFTables)' })}</Radio.Button>
                      </Radio.Group>
                    </Form.Item>

                    {advancedEngine !== 'frp' && advancedEngine !== 'tunnel-panel' && (
                      <Row gutter={16}>
                        <Col span={14}>
                          <Form.Item
                            name="transport"
                            label={t('pages.hashem.transportLabel', { defaultValue: 'Transport Protocol' })}
                            initialValue="tcpmux"
                          >
                            <Select options={backhaulTransportOptions} />
                          </Form.Item>
                        </Col>
                        <Col span={10}>
                          <Form.Item
                            name="backhaulPort"
                            label={t('pages.hashem.backhaulPortLabel', { defaultValue: 'Backhaul Server Port' })}
                            initialValue={3080}
                          >
                            <InputNumber style={{ width: '100%' }} min={1} max={65535} />
                          </Form.Item>
                        </Col>
                      </Row>
                    )}

                    <Form.Item
                      name="remotePub"
                      label={t('pages.hashem.peerPublicIp')}
                      rules={[{ required: true, message: t('pages.hashem.remotePubRequired') }]}
                    >
                      <Input placeholder={t('pages.hashem.iranIpPlaceholder', { defaultValue: 'your iran server ip' })} />
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
                          { label: t('pages.hashem.directGreLabel', { defaultValue: 'Direct GRE (Raw IP proto 47 - Default & Stable)' }), value: 'direct' },
                          { label: 'FoU:443 (Foo-over-UDP Port 443)', value: 'fou:443' },
                          { label: 'WSS:8443 (WebSocket TLS)', value: 'wss:8443' },
                        ]}
                      />
                    </Form.Item>

                    <Form.Item name="ports" label={t('pages.hashem.portsLabel')} extra={t('pages.hashem.portsExtra')}>
                      <Input placeholder="8080" />
                    </Form.Item>

                    <Form.Item
                      name="autoCreateInbound"
                      valuePropName="checked"
                      initialValue={true}
                      extra={t('pages.hashem.autoCreateInboundExtra', {
                        defaultValue: 'When enabled, creates a matching VLESS-WS inbound for the tunneled port (similar to port 8080) with automatic host header configuration.',
                      })}
                    >
                      <Checkbox style={{ color: '#38bdf8', fontWeight: 600 }}>
                        ⚡ {t('pages.hashem.autoCreateInboundLabel', { defaultValue: 'Auto-create matching VLESS-WS inbound for tunnel port' })}
                      </Checkbox>
                    </Form.Item>

                    <Form.Item
                      name="inboundHost"
                      label={t('pages.hashem.inboundHostLabel', { defaultValue: 'WebSocket Host Header (WS Host Header)' })}
                      extra={t('pages.hashem.inboundHostExtra', {
                        defaultValue: 'Host domain or IP set in inbound WebSocket header (e.g. your iran domain or Iran server IP). If left blank, Iran server IP is used.',
                      })}
                    >
                      <Input placeholder={t('pages.hashem.inboundHostPlaceholder', { defaultValue: 'your iran domain' })} />
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
                ),
              },
            ]}
          />
        </Modal>

        <Modal
          title={
            <Space>
              <EditOutlined style={{ color: '#38bdf8' }} />
              <span>{t('pages.hashem.editPortsModalTitle', { defaultValue: 'Edit Tunnel Forwarded Ports' })}</span>
            </Space>
          }
          open={editPortsModalOpen}
          onCancel={() => setEditPortsModalOpen(false)}
          footer={[
            <Button key="cancel" onClick={() => setEditPortsModalOpen(false)}>
              {t('cancel', { defaultValue: 'Cancel' })}
            </Button>,
            <Button
              key="save"
              type="primary"
              loading={isEditingPorts}
              onClick={handleSavePorts}
              style={{
                background: 'linear-gradient(135deg, #0284c7, #2563eb)',
                borderColor: '#38bdf8',
              }}
            >
              {t('pages.hashem.saveAndApply', { defaultValue: 'Save & Apply Immediately' })}
            </Button>,
          ]}
          className="hashem-modal"
        >
          <Paragraph style={{ color: 'var(--nc-subtext, #94a3b8)', fontSize: 13, marginBottom: 16 }}>
            {t('pages.hashem.editPortsDesc', {
              defaultValue: 'Add, remove or save forwarded ports instantly without tunnel downtime.',
            })}
          </Paragraph>

          <div style={{ marginBottom: 16 }}>
            <Text strong style={{ display: 'block', marginBottom: 8, color: 'var(--nc-text, #f1f5f9)' }}>
              {t('pages.hashem.forwardedPortsList', { defaultValue: 'Current Ports' })}:
            </Text>
            <div
              style={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: 8,
                padding: 12,
                background: 'rgba(0, 0, 0, 0.25)',
                borderRadius: 8,
                border: '1px solid rgba(255, 255, 255, 0.08)',
                minHeight: 46,
                alignItems: 'center',
              }}
            >
              {portChips.map((p: number) => (
                <Tag
                  key={p}
                  closable
                  onClose={(e: React.MouseEvent<HTMLElement>) => {
                    e.preventDefault();
                    handleRemovePortChip(p);
                  }}
                  color="cyan"
                  style={{ fontSize: 13, padding: '4px 10px', borderRadius: 6 }}
                >
                  Port {p}
                </Tag>
              ))}
            </div>
          </div>

          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
            <div style={{ flex: 1 }}>
              <label
                style={{
                  display: 'block',
                  fontSize: 12,
                  fontWeight: 600,
                  marginBottom: 6,
                  color: 'var(--nc-subtext, #94a3b8)',
                }}
              >
                {t('pages.hashem.addPortLabel', { defaultValue: 'Add New Port' })}:
              </label>
              <InputNumber
                min={1}
                max={65535}
                value={newPortInput}
                onChange={(val: number | null) => setNewPortInput(val)}
                onPressEnter={handleAddPortChip}
                placeholder={t('pages.hashem.enterPortPlaceholder', { defaultValue: 'e.g. 8080' })}
                style={{ width: '100%' }}
              />
            </div>
            <Button
              icon={<PlusOutlined />}
              onClick={handleAddPortChip}
              type="dashed"
              style={{ borderColor: 'rgba(56, 189, 248, 0.4)' }}
            >
              {t('pages.hashem.addPortBtn', { defaultValue: 'Add' })}
            </Button>
          </div>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
