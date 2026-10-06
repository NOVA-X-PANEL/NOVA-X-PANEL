import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
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
  ApiOutlined,
  CheckCircleOutlined,
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
  RocketOutlined,
  SendOutlined,
  SettingOutlined,
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
    editPorts,
    isEditingPorts,
    setup,
    isSettingUp,
    setupSSH,
    isSettingUpSSH,
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
  const [setupTab, setSetupTab] = useState<'ssh' | 'oneliner' | 'advanced'>('ssh');
  const [generatedCmd, setGeneratedCmd] = useState<string | null>(null);

  const [editPortsModalOpen, setEditPortsModalOpen] = useState(false);
  const [portChips, setPortChips] = useState<number[]>([]);
  const [newPortInput, setNewPortInput] = useState<number | null>(null);

  const [sshForm] = Form.useForm();
  const [oneLinerForm] = Form.useForm();
  const [form] = Form.useForm<HashemSetupPayload>();

  const sshEngine = Form.useWatch('engine', sshForm) || 'frp';
  const oneLinerEngine = Form.useWatch('engine', oneLinerForm) || 'frp';
  const advancedEngine = Form.useWatch('engine', form) || 'frp';

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
      message.warning(t('pages.hashem.portMustBeValid', { defaultValue: 'شماره پورت باید بین ۱ تا ۶۵۵۳۵ باشد' }));
      return;
    }
    if (portChips.includes(newPortInput)) {
      message.warning(t('pages.hashem.portAlreadyExists', { defaultValue: 'این پورت قبلاً در لیست وجود دارد' }));
      return;
    }
    setPortChips([...portChips, newPortInput].sort((a, b) => a - b));
    setNewPortInput(null);
  };

  const handleRemovePortChip = (portToRemove: number) => {
    if (portChips.length <= 1) {
      message.warning(t('pages.hashem.atLeastOnePortRequired', { defaultValue: 'حداقل یک پورت باید در لیست باقی بماند' }));
      return;
    }
    setPortChips(portChips.filter((p: number) => p !== portToRemove));
  };

  const handleSavePorts = async () => {
    if (portChips.length === 0) {
      message.warning(t('pages.hashem.atLeastOnePortRequired', { defaultValue: 'حداقل یک پورت باید در لیست باقی بماند' }));
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

  const handleSSHSubmit = async (values: any) => {
    try {
      await setupSSH({
        engine: values.engine || 'frp',
        transport: values.transport || 'tcpmux',
        backhaulPort: values.backhaulPort || 3080,
        snappy: values.snappy ?? true,
        iranIp: values.iranIp,
        sshPort: values.sshPort || 22,
        sshUser: values.sshUser || 'root',
        sshPassword: values.sshPassword,
        ports: values.ports || status.ports?.join(', ') || '8080',
        carrier: values.carrier || 'fou:443',
        autoCreateInbound: values.autoCreateInbound ?? true,
        inboundHost: values.inboundHost || values.iranIp,
      });
      setSetupModalOpen(false);
      sshForm.resetFields();
    } catch {
      // toast shown by mutation
    }
  };

  const handleOneLinerSubmit = async (values: any) => {
    try {
      const res = await generateOneLiner({
        engine: values.engine || 'frp',
        transport: values.transport || 'tcpmux',
        backhaulPort: values.backhaulPort || 3080,
        snappy: values.snappy ?? true,
        iranIp: values.iranIp,
        ports: values.ports || status.ports?.join(', ') || '8080',
        carrier: values.carrier || 'fou:443',
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
      autoCreateInbound: (values as any).autoCreateInbound ?? true,
      inboundHost: (values as any).inboundHost || values.remotePub,
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

  const isEngineBackhaul = status.engine === 'backhaul' || status.engine === 'gre-backhaul';
  const engineStatus = isEngineBackhaul ? status.backhaulStatus : status.frpStatus;

  const isHealthy = status.running && engineStatus === 'active' && status.pingMs > 0;
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
                          {(status as any).localGreIp || (status as any).localGreIP || '-'}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t('pages.hashem.peerGreIp')}: </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {(status as any).remoteGreIp || (status as any).remoteGreIP || '-'}
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
                      ) : isHealthy ? (
                        <Title level={3} style={{ margin: 0, color: '#52c41a', textShadow: '0 0 10px rgba(82, 196, 26, 0.4)' }}>
                          {t('pages.hashem.frpRunning')}
                        </Title>
                      ) : (
                        <Title level={3} style={{ margin: 0, color: '#ff4d4f' }}>
                          {t('pages.hashem.unreachable')}
                        </Title>
                      )}
                    </div>
                    <Text type="secondary">{t('pages.hashem.testedAgainst', { ip: (status as any).remoteGreIp || (status as any).remoteGreIP || 'Peer' })}</Text>
                  </Card>
                </Col>

                {/* Tunnel Transport / Engine */}
                <Col xs={24} sm={12} lg={6}>
                  <Card
                    className="hashem-card"
                    title={
                      <Space>
                        {status.engine === 'backhaul' ? (
                          <RocketOutlined style={{ color: '#d946ef' }} />
                        ) : (
                          <SendOutlined style={{ color: '#a855f7' }} />
                        )}
                        <span>
                          {isEngineBackhaul
                            ? t('pages.hashem.cardBackhaulTitle', { defaultValue: 'تانل Backhaul' })
                            : t('pages.hashem.cardFrpTitle')}
                        </span>
                      </Space>
                    }
                  >
                    <div style={{ marginBottom: 14 }}>
                      <Badge
                        status={
                          engineStatus === 'active'
                            ? 'success'
                            : engineStatus === 'connecting'
                              ? 'warning'
                              : 'error'
                        }
                        text={
                          <Text
                            strong
                            style={{
                              color:
                                engineStatus === 'active'
                                  ? '#52c41a'
                                  : engineStatus === 'connecting'
                                    ? '#faad14'
                                    : '#ff4d4f',
                            }}
                          >
                            {engineStatus === 'active'
                              ? t('pages.hashem.frpRunning')
                              : engineStatus === 'connecting'
                                ? t('pages.hashem.frpConnecting', { defaultValue: 'در حال برقراری اتصال...' })
                                : t('pages.hashem.frpStopped')}
                          </Text>
                        }
                      />
                    </div>
                    <div style={{ fontSize: 13, display: 'flex', flexDirection: 'column', gap: 6 }}>
                      <div>
                        <Text type="secondary">
                          {status.engine === 'backhaul'
                            ? t('pages.hashem.backhaulPort', { defaultValue: 'پورت Backhaul' })
                            : t('pages.hashem.frpControlPort')}
                          :{' '}
                        </Text>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {status.engine === 'backhaul' ? status.backhaulPort || 3080 : status.frpPort || '-'}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t('pages.hashem.engineLabel', { defaultValue: 'نوع موتور' })}: </Text>
                        <Tag color={status.engine === 'backhaul' ? 'magenta' : status.engine === 'gre-backhaul' ? 'cyan' : 'purple'}>
                          {status.engine === 'backhaul'
                            ? `Backhaul (${status.transport || status.backhaulType || 'tcpmux'})`
                            : status.engine === 'gre-backhaul'
                              ? `GRE + Backhaul`
                              : 'FRP Reverse'}
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
                      {t('pages.hashem.benchmarkCardTitle', { defaultValue: 'تست عملکرد و بنچمارک هوشمند کریرها (Carrier Benchmark)' })}
                    </span>
                    {status.benchmark?.bestCarrier && (
                      <Tag color="success" style={{ borderRadius: 6, fontWeight: 600 }}>
                        {t('pages.hashem.bestCarrier', { defaultValue: 'بهترین مسیر:' })} {status.benchmark.bestCarrier}
                      </Tag>
                    )}
                    {status.benchmark?.updatedAt && (
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        ({t('pages.hashem.lastUpdated', { defaultValue: 'آخرین بررسی:' })} {status.benchmark.updatedAt})
                      </Text>
                    )}
                  </Space>
                }
                extra={
                  <Space wrap>
                    <Space style={{ marginRight: 8 }}>
                      <Text style={{ fontSize: 13, color: 'var(--nc-text-2, #94a3b8)' }}>
                        {t('pages.hashem.autoPilotLabel', { defaultValue: 'اتوپایلوت هوشمند:' })}
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
                      {t('pages.hashem.runBenchmarkBtn', { defaultValue: 'اجرای بنچمارک زنده' })}
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
                        title: t('pages.hashem.carrierName', { defaultValue: 'کریر / مسیر' }),
                        dataIndex: 'name',
                        key: 'name',
                        render: (_: any, record: any) => (
                          <Space>
                            <Text strong style={{ color: 'var(--nc-text, #f1f5f9)' }}>
                              {record.name}
                            </Text>
                            {status.activeCarrier === record.id || status.carrier === record.id ? (
                              <Tag color="cyan">{t('pages.hashem.active', { defaultValue: 'فعال' })}</Tag>
                            ) : null}
                          </Space>
                        ),
                      },
                      {
                        title: t('pages.hashem.carrierType', { defaultValue: 'نوع پروتکل' }),
                        dataIndex: 'type',
                        key: 'type',
                        render: (type: string) => <Tag color="blue">{type}</Tag>,
                      },
                      {
                        title: t('pages.hashem.avgRtt', { defaultValue: 'تاخیر (RTT)' }),
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
                        title: t('pages.hashem.packetLoss', { defaultValue: 'پکت لاس' }),
                        dataIndex: 'packetLoss',
                        key: 'packetLoss',
                        render: (loss: number) => (
                          <Text style={{ color: loss === 0 ? '#52c41a' : loss < 10 ? '#faad14' : '#ff4d4f' }}>
                            {loss.toFixed(1)}%
                          </Text>
                        ),
                      },
                      {
                        title: t('pages.hashem.jitter', { defaultValue: 'جیتر (Jitter)' }),
                        dataIndex: 'jitterMs',
                        key: 'jitterMs',
                        render: (jitter: number) => <Text>{jitter > 0 ? `${jitter.toFixed(1)} ms` : '-'}</Text>,
                      },
                      {
                        title: t('pages.hashem.score', { defaultValue: 'امتیاز کیفیت' }),
                        dataIndex: 'score',
                        key: 'score',
                        render: (score: number) => (
                          <Tag color={score >= 80 ? 'green' : score >= 50 ? 'orange' : 'red'}>
                            {score.toFixed(0)} / 100
                          </Tag>
                        ),
                      },
                      {
                        title: t('pages.hashem.carrierStatus', { defaultValue: 'وضعیت' }),
                        dataIndex: 'status',
                        key: 'status',
                        render: (st: string) => (
                          <Badge
                            status={st === 'optimal' ? 'success' : st === 'good' ? 'processing' : 'error'}
                            text={st === 'optimal' ? 'عالی' : st === 'good' ? 'خوب' : 'ضعیف'}
                          />
                        ),
                      },
                      {
                        title: t('pages.hashem.action', { defaultValue: 'عملیات' }),
                        key: 'action',
                        render: (_: any, record: any) => (
                          <Button
                            size="small"
                            type="dashed"
                            disabled={status.activeCarrier === record.id || status.carrier === record.id}
                            loading={isSettingCarrier}
                            onClick={() => handleCarrierChange(record.id)}
                          >
                            {t('pages.hashem.switchToCarrier', { defaultValue: 'انتخاب این مسیر' })}
                          </Button>
                        ),
                      },
                    ]}
                  />
                ) : (
                  <div style={{ textAlign: 'center', padding: '24px 0' }}>
                    <Text type="secondary">
                      {t('pages.hashem.noBenchmarkYet', {
                        defaultValue: 'هنوز بنچمارکی ثبت نشده است. برای ارزیابی تاخیر و پکت‌لاس تمامی کریرها، روی دکمه «اجرای بنچمارک زنده» کلیک کنید.',
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
                                host: status.remotePubIP,
                              })
                            }
                            loading={isAutoCreatingInbound}
                            style={{ borderRadius: 6, fontSize: 12, borderColor: '#00f2fe', color: '#00f2fe' }}
                          >
                            {t('pages.hashem.autoCreateInboundBtn', { defaultValue: '⚡ ساخت اینباند VLESS-WS' })}
                          </Button>
                          <Button
                            size="small"
                            type="primary"
                            ghost
                            icon={<EditOutlined />}
                            onClick={handleOpenEditPorts}
                            style={{ borderRadius: 6, fontSize: 12 }}
                          >
                            {t('pages.hashem.editPortsBtn', { defaultValue: 'ویرایش پورت‌ها' })}
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
                          {(status as any).localPubIp || (status as any).localPubIP || '-'}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label={t('pages.hashem.remotePublicIp')}>
                        <Text code style={{ background: 'rgba(0,0,0,0.4)', borderColor: 'rgba(56,189,248,0.2)' }}>
                          {(status as any).remotePubIp || (status as any).remotePubIP || '-'}
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
            onChange={(k) => setSetupTab(k as any)}
            items={[
              {
                key: 'ssh',
                label: (
                  <Space>
                    <ApiOutlined />
                    <span>{t('pages.hashem.tabSSHAuto')}</span>
                  </Space>
                ),
                children: (
                  <Form
                    form={sshForm}
                    layout="vertical"
                    initialValues={{
                      sshPort: 22,
                      sshUser: 'root',
                      ports: status.ports?.join(', ') || '8080',
                    }}
                    onFinish={handleSSHSubmit}
                    style={{ marginTop: 12 }}
                  >
                    <Alert
                      type="info"
                      showIcon
                      message="راه‌اندازی کاملاً خودکار تانل با اتصال SSH"
                      description="با وارد کردن آی‌پی و رمز عبور سرور ایران، پنل مستقیماً از طریق SSH به سرور ایران متصل می‌شود. (نکته: در صورتی که دیتاسنتر ایران دسترسی مستقیم پورت ۲۲ را مسدود کرده باشد و تایم‌اوت دریافت کردید، لطفاً از تب دوم «دستور تک‌خطی سرور ایران» استفاده فرمایید.)"
                      style={{ marginBottom: 16 }}
                    />

                    <Form.Item
                      name="engine"
                      label={t('pages.hashem.engineLabel', { defaultValue: 'موتور تانل (Tunnel Engine)' })}
                      initialValue="frp"
                    >
                      <Radio.Group buttonStyle="solid" style={{ width: '100%' }}>
                        <Radio.Button value="frp">⚡ FRP (معکوس)</Radio.Button>
                        <Radio.Button value="backhaul">🚀 Backhaul (پرسرعت)</Radio.Button>
                        <Radio.Button value="gre-backhaul">🌐 GRE + Backhaul</Radio.Button>
                      </Radio.Group>
                    </Form.Item>

                    {sshEngine !== 'frp' && (
                      <Row gutter={16}>
                        <Col span={14}>
                          <Form.Item
                            name="transport"
                            label={t('pages.hashem.transportLabel', { defaultValue: 'پروتکل انتقال (Transport)' })}
                            initialValue="tcpmux"
                          >
                            <Select
                              options={[
                                { label: 'TCPMux (پرسرعت چندکاناله - پیشنهاد شده)', value: 'tcpmux' },
                                { label: 'WSSMux (وب‌سوکت ایمن با TLS)', value: 'wssmux' },
                                { label: 'TCPO (تک کانکشن مستقیم)', value: 'tcpo' },
                              ]}
                            />
                          </Form.Item>
                        </Col>
                        <Col span={10}>
                          <Form.Item
                            name="backhaulPort"
                            label={t('pages.hashem.backhaulPortLabel', { defaultValue: 'پورت سرور Backhaul' })}
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
                      <Input placeholder="مثلاً 94.183.210.29" />
                    </Form.Item>

                    <Row gutter={16}>
                      <Col span={14}>
                        <Form.Item
                          name="sshPassword"
                          label={t('pages.hashem.sshPasswordLabel')}
                          rules={[{ required: true, message: t('pages.hashem.sshPasswordRequired') }]}
                        >
                          <Input.Password placeholder="Password..." />
                        </Form.Item>
                      </Col>
                      <Col span={5}>
                        <Form.Item name="sshPort" label={t('pages.hashem.sshPortLabel')}>
                          <InputNumber style={{ width: '100%' }} min={1} max={65535} />
                        </Form.Item>
                      </Col>
                      <Col span={5}>
                        <Form.Item name="sshUser" label={t('pages.hashem.sshUserLabel')}>
                          <Input />
                        </Form.Item>
                      </Col>
                    </Row>

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
                        defaultValue: 'با فعال بودن این گزینه، همزمان با اجرای تانل یک اینباند VLESS-WS متناظر با پورت انتخابی (مشابه اینباند ۸۰۸۰) با تنظیم خودکار هاست ساخته می‌شود.',
                      })}
                    >
                      <Checkbox style={{ color: '#38bdf8', fontWeight: 600 }}>
                        ⚡ {t('pages.hashem.autoCreateInboundLabel', { defaultValue: 'ساخت خودکار اینباند VLESS-WS متناظر با پورت تانل' })}
                      </Checkbox>
                    </Form.Item>

                    <Form.Item
                      name="inboundHost"
                      label={t('pages.hashem.inboundHostLabel', { defaultValue: 'هاست هدر وب‌سوکت (WS Host Header)' })}
                      extra={t('pages.hashem.inboundHostExtra', {
                        defaultValue: 'آدرس هاست یا دامنه‌ای که در هدر وب‌سوکت اینباند ست می‌شود (مثلاً pro.ksmrx2.ir یا آی‌پی سرور ایران). در صورت خالی ماندن، آی‌پی سرور ایران درج می‌شود.',
                      })}
                    >
                      <Input placeholder="pro.ksmrx2.ir" />
                    </Form.Item>

                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 24 }}>
                      <Button onClick={() => setSetupModalOpen(false)}>{t('cancel')}</Button>
                      <Button
                        type="primary"
                        htmlType="submit"
                        loading={isSettingUpSSH}
                        className="hashem-btn-setup"
                        icon={<ApiOutlined />}
                      >
                        {isSettingUpSSH ? t('pages.hashem.sshConnecting') : t('pages.hashem.autoSetupBtn')}
                      </Button>
                    </div>
                  </Form>
                ),
              },
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
                      message="دستور تک‌خطی سرور ایران"
                      description="اگر مایل به ارائه پسورد SSH نیستید، کافیست آی‌پی ایران و پورت‌ها را مشخص کنید. پنل خارج آماده شده و یک دستور تک‌خطی به شما تحویل می‌دهد تا در ترمینال ایران اجرا کنید."
                      style={{ marginBottom: 16 }}
                    />

                    <Form.Item
                      name="engine"
                      label={t('pages.hashem.engineLabel', { defaultValue: 'موتور تانل (Tunnel Engine)' })}
                      initialValue="frp"
                    >
                      <Radio.Group buttonStyle="solid" style={{ width: '100%' }}>
                        <Radio.Button value="frp">⚡ FRP (معکوس)</Radio.Button>
                        <Radio.Button value="backhaul">🚀 Backhaul (پرسرعت)</Radio.Button>
                        <Radio.Button value="gre-backhaul">🌐 GRE + Backhaul</Radio.Button>
                      </Radio.Group>
                    </Form.Item>

                    {oneLinerEngine !== 'frp' && (
                      <Row gutter={16}>
                        <Col span={14}>
                          <Form.Item
                            name="transport"
                            label={t('pages.hashem.transportLabel', { defaultValue: 'پروتکل انتقال (Transport)' })}
                            initialValue="tcpmux"
                          >
                            <Select
                              options={[
                                { label: 'TCPMux (پرسرعت چندکاناله - پیشنهاد شده)', value: 'tcpmux' },
                                { label: 'WSSMux (وب‌سوکت ایمن با TLS)', value: 'wssmux' },
                                { label: 'TCPO (تک کانکشن مستقیم)', value: 'tcpo' },
                              ]}
                            />
                          </Form.Item>
                        </Col>
                        <Col span={10}>
                          <Form.Item
                            name="backhaulPort"
                            label={t('pages.hashem.backhaulPortLabel', { defaultValue: 'پورت سرور Backhaul' })}
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
                      <Input placeholder="مثلاً 94.183.210.29" />
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
                        defaultValue: 'با فعال بودن این گزینه، همزمان با اجرای تانل یک اینباند VLESS-WS متناظر با پورت انتخابی (مشابه اینباند ۸۰۸۰) با تنظیم خودکار هاست ساخته می‌شود.',
                      })}
                    >
                      <Checkbox style={{ color: '#38bdf8', fontWeight: 600 }}>
                        ⚡ {t('pages.hashem.autoCreateInboundLabel', { defaultValue: 'ساخت خودکار اینباند VLESS-WS متناظر با پورت تانل' })}
                      </Checkbox>
                    </Form.Item>

                    <Form.Item
                      name="inboundHost"
                      label={t('pages.hashem.inboundHostLabel', { defaultValue: 'هاست هدر وب‌سوکت (WS Host Header)' })}
                      extra={t('pages.hashem.inboundHostExtra', {
                        defaultValue: 'آدرس هاست یا دامنه‌ای که در هدر وب‌سوکت اینباند ست می‌شود (مثلاً pro.ksmrx2.ir یا آی‌پی سرور ایران). در صورت خالی ماندن، آی‌پی سرور ایران درج می‌شود.',
                      })}
                    >
                      <Input placeholder="pro.ksmrx2.ir" />
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
                      carrier: 'fou:443',
                      frpPort: 36067,
                      ports: status.ports?.join(', ') || '8080',
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
                      label={t('pages.hashem.engineLabel', { defaultValue: 'موتور تانل (Tunnel Engine)' })}
                      initialValue="frp"
                    >
                      <Radio.Group buttonStyle="solid" style={{ width: '100%' }}>
                        <Radio.Button value="frp">⚡ FRP (معکوس)</Radio.Button>
                        <Radio.Button value="backhaul">🚀 Backhaul (پرسرعت)</Radio.Button>
                        <Radio.Button value="gre-backhaul">🌐 GRE + Backhaul</Radio.Button>
                      </Radio.Group>
                    </Form.Item>

                    {advancedEngine !== 'frp' && (
                      <Row gutter={16}>
                        <Col span={14}>
                          <Form.Item
                            name="transport"
                            label={t('pages.hashem.transportLabel', { defaultValue: 'پروتکل انتقال (Transport)' })}
                            initialValue="tcpmux"
                          >
                            <Select
                              options={[
                                { label: 'TCPMux (پرسرعت چندکاناله - پیشنهاد شده)', value: 'tcpmux' },
                                { label: 'WSSMux (وب‌سوکت ایمن با TLS)', value: 'wssmux' },
                                { label: 'TCPO (تک کانکشن مستقیم)', value: 'tcpo' },
                              ]}
                            />
                          </Form.Item>
                        </Col>
                        <Col span={10}>
                          <Form.Item
                            name="backhaulPort"
                            label={t('pages.hashem.backhaulPortLabel', { defaultValue: 'پورت سرور Backhaul' })}
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
                      <Input placeholder="e.g. 94.183.210.29" />
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
                      <Input placeholder="8080" />
                    </Form.Item>

                    <Form.Item
                      name="autoCreateInbound"
                      valuePropName="checked"
                      initialValue={true}
                      extra={t('pages.hashem.autoCreateInboundExtra', {
                        defaultValue: 'با فعال بودن این گزینه، همزمان با اجرای تانل یک اینباند VLESS-WS متناظر با پورت انتخابی (مشابه اینباند ۸۰۸۰) با تنظیم خودکار هاست ساخته می‌شود.',
                      })}
                    >
                      <Checkbox style={{ color: '#38bdf8', fontWeight: 600 }}>
                        ⚡ {t('pages.hashem.autoCreateInboundLabel', { defaultValue: 'ساخت خودکار اینباند VLESS-WS متناظر با پورت تانل' })}
                      </Checkbox>
                    </Form.Item>

                    <Form.Item
                      name="inboundHost"
                      label={t('pages.hashem.inboundHostLabel', { defaultValue: 'هاست هدر وب‌سوکت (WS Host Header)' })}
                      extra={t('pages.hashem.inboundHostExtra', {
                        defaultValue: 'آدرس هاست یا دامنه‌ای که در هدر وب‌سوکت اینباند ست می‌شود (مثلاً pro.ksmrx2.ir یا آی‌پی سرور ایران). در صورت خالی ماندن، آی‌پی سرور ایران درج می‌شود.',
                      })}
                    >
                      <Input placeholder="pro.ksmrx2.ir" />
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
              <span>{t('pages.hashem.editPortsModalTitle', { defaultValue: 'ویرایش پورت‌های فوروارد شده تانل' })}</span>
            </Space>
          }
          open={editPortsModalOpen}
          onCancel={() => setEditPortsModalOpen(false)}
          footer={[
            <Button key="cancel" onClick={() => setEditPortsModalOpen(false)}>
              {t('cancel', { defaultValue: 'انصراف' })}
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
              {t('pages.hashem.saveAndApply', { defaultValue: 'ذخیره و اعمال آنی' })}
            </Button>,
          ]}
          className="hashem-modal"
        >
          <Paragraph style={{ color: 'var(--nc-subtext, #94a3b8)', fontSize: 13, marginBottom: 16 }}>
            {t('pages.hashem.editPortsDesc', {
              defaultValue: 'پورت‌های فوروارد شده را بدون قطعی تانل به صورت آنی اضافه، حذف یا ذخیره کنید.',
            })}
          </Paragraph>

          <div style={{ marginBottom: 16 }}>
            <Text strong style={{ display: 'block', marginBottom: 8, color: 'var(--nc-text, #f1f5f9)' }}>
              {t('pages.hashem.forwardedPortsList', { defaultValue: 'پورت‌های فعلی' })}:
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
                  onClose={(e: any) => {
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
                {t('pages.hashem.addPortLabel', { defaultValue: 'افزودن پورت جدید' })}:
              </label>
              <InputNumber
                min={1}
                max={65535}
                value={newPortInput}
                onChange={(val: number | null) => setNewPortInput(val)}
                onPressEnter={handleAddPortChip}
                placeholder={t('pages.hashem.enterPortPlaceholder', { defaultValue: 'مثال: 8080' })}
                style={{ width: '100%' }}
              />
            </div>
            <Button
              icon={<PlusOutlined />}
              onClick={handleAddPortChip}
              type="dashed"
              style={{ borderColor: 'rgba(56, 189, 248, 0.4)' }}
            >
              {t('pages.hashem.addPortBtn', { defaultValue: 'افزودن' })}
            </Button>
          </div>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
