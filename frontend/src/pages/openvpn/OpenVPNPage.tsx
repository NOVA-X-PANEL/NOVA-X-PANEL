import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Layout,
  ConfigProvider,
  Card,
  Table,
  Button,
  Space,
  Row,
  Col,
  Typography,
  message,
  Popconfirm,
  Tag,
  Modal,
  Form,
  Input,
  InputNumber,
  Tooltip,
  Switch,
  Progress,
  DatePicker,
  Checkbox,
  Avatar,
  Empty,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import {
  SafetyOutlined,
  ReloadOutlined,
  PlusOutlined,
  DeleteOutlined,
  DownloadOutlined,
  CopyOutlined,
  SyncOutlined,
  GlobalOutlined,
  SettingOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  ThunderboltOutlined,
  EditOutlined,
  UndoOutlined,
  UserOutlined,
  KeyOutlined,
  ClockCircleOutlined,
  DashboardOutlined,
  StopOutlined,
  ShareAltOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import { useTheme } from '@/hooks/useTheme';
import AppSidebar from '@/layouts/AppSidebar';
import './OpenVPNPage.css';

const { Title, Text, Paragraph } = Typography;

interface OpenVPNClientInfo {
  name: string;
  password?: string;
  enable: boolean;
  total_gb: number;
  expiry_time: number;
  limit_ip: number;
  up: number;
  down: number;
  total_used: number;
  traffic_used_str: string;
  traffic_limit_str: string;
  traffic_percent: number;
  created_at: string;
  is_connected: boolean;
  connected_ips?: string[];
  current_connections: number;
  since?: string;
  is_expired: boolean;
  is_traffic_exceeded: boolean;
}

interface OpenVPNConnectedClient {
  common_name: string;
  real_address: string;
  bytes_received: string;
  bytes_sent: string;
  connected_since: string;
}

interface OpenVPNStatus {
  installed: boolean;
  running: boolean;
  port: number;
  proto: string;
  subnet: string;
  direct_ip: string;
  iran_ip: string;
  remote_host?: string;
  remote_port?: number;
  total_clients: number;
  online_clients: number;
  connected_clients: OpenVPNConnectedClient[];
  clients: OpenVPNClientInfo[];
}

export default function OpenVPNPage() {
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [messageApi, contextHolder] = message.useMessage();

  const pageClass = useMemo(() => {
    const classes = ['openvpn-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<OpenVPNStatus | null>(null);

  // Modals state
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [addingClient, setAddingClient] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [editingClient, setEditingClient] = useState<OpenVPNClientInfo | null>(null);
  const [savingEdit, setSavingEdit] = useState(false);
  const [restarting, setRestarting] = useState(false);

  // Password visibility map
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});
  const toggleShowPassword = (name: string) => {
    setShowPasswords((prev) => ({ ...prev, [name]: !prev[name] }));
  };

  // Forms
  const [addForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // Endpoint settings state with localStorage persistence
  const [endpointHost, setEndpointHost] = useState<string>(() => {
    return localStorage.getItem('nova_openvpn_endpoint_host') || '';
  });
  const [portInput, setPortInput] = useState<string>(() => {
    return localStorage.getItem('nova_openvpn_endpoint_port') || '8443';
  });

  // Dedicated Config Modal for downloading/copying .ovpn
  const [configModal, setConfigModal] = useState<{
    open: boolean;
    clientName: string;
    password: string;
    host: string;
    portStr: string;
    inlineAuth: boolean;
    previewContent: string;
    loadingPreview: boolean;
  }>({
    open: false,
    clientName: '',
    password: '',
    host: '',
    portStr: '',
    inlineAuth: false,
    previewContent: '',
    loadingPreview: false,
  });

  const generateRandomPassword = () => {
    const chars = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    let res = '';
    for (let i = 0; i < 10; i++) {
      res += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return res;
  };

  const handleHostChange = (val: string) => {
    const trimmed = val.trim();
    setEndpointHost(trimmed);
    localStorage.setItem('nova_openvpn_endpoint_host', trimmed);
  };

  const handlePortInputChange = (val: string) => {
    const digits = val.replace(/[^0-9]/g, '');
    setPortInput(digits);
    if (digits) {
      localStorage.setItem('nova_openvpn_endpoint_port', digits);
    }
  };

  const setPresetPort = (p: number) => {
    const s = p.toString();
    setPortInput(s);
    localStorage.setItem('nova_openvpn_endpoint_port', s);
  };

  const [savingSettings, setSavingSettings] = useState(false);

  const handleSaveSettings = async () => {
    if (!endpointHost) {
      messageApi.warning('لطفاً آدرس یا آی‌پی ریموت را وارد کنید');
      return;
    }
    const pNum = parseInt(portInput, 10);
    const finalPort = (!isNaN(pNum) && pNum > 0 && pNum <= 65535) ? pNum : (status?.port || 2083);
    setSavingSettings(true);
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/settings', {
        remote_host: endpointHost.trim(),
        remote_port: finalPort,
        server_port: status?.port || 2083,
      });
      if (res && res.success) {
        messageApi.success('تنظیمات OpenVPN با موفقیت در پایگاه داده سرور ذخیره شد و پایدار ماند');
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در ذخیره تنظیمات');
      }
    } catch (e: unknown) {
      const err = e as { message?: string };
      messageApi.error(err?.message || 'خطا در ارتباط با سرور');
    } finally {
      setSavingSettings(false);
    }
  };

  const fetchStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await HttpUtil.get<OpenVPNStatus>('/panel/api/openvpn/status');
      if (res && res.obj) {
        setStatus(res.obj);
        if (res.obj.remote_host) {
          setEndpointHost(res.obj.remote_host);
          localStorage.setItem('nova_openvpn_endpoint_host', res.obj.remote_host);
        } else {
          setEndpointHost((prev) => {
            if (!prev) {
              const initial = res.obj?.iran_ip || res.obj?.direct_ip || '';
              if (initial) localStorage.setItem('nova_openvpn_endpoint_host', initial);
              return initial;
            }
            return prev;
          });
        }
        if (res.obj.remote_port && res.obj.remote_port > 0) {
          setPortInput(res.obj.remote_port.toString());
          localStorage.setItem('nova_openvpn_endpoint_port', res.obj.remote_port.toString());
        }
      }
    } catch {
      messageApi.error('خطا در دریافت وضعیت OpenVPN');
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  const activeHost =
    endpointHost || status?.remote_host || status?.iran_ip || status?.direct_ip || '';
  const activePort = useMemo(() => {
    const p = parseInt(portInput, 10);
    if (!isNaN(p) && p > 0 && p <= 65535) {
      return p;
    }
    return status?.remote_port || status?.port || 2083;
  }, [portInput, status]);

  // Add client submit handler
  const handleAddClient = async (values: {
    name: string;
    password?: string;
    totalGB?: number;
    expiryDate?: Dayjs | null;
    limitIp?: number;
  }) => {
    const name = values.name.trim();
    const password = values.password ? values.password.trim() : '';
    const totalGB = values.totalGB || 0;
    const limitIp = values.limitIp || 0;
    const expiryTime = values.expiryDate ? values.expiryDate.valueOf() : 0;

    setAddingClient(true);
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/client/add', {
        name,
        password,
        totalGB,
        expiryTime,
        limitIp,
      });
      if (res && res.success) {
        messageApi.success(`کاربر ${name} با موفقیت ساخته شد`);
        setIsAddModalOpen(false);
        addForm.resetFields();
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در ساخت کاربر');
      }
    } catch (e: unknown) {
      const err = e as { message?: string };
      messageApi.error(err?.message || 'خطا در ارتباط با سرور');
    } finally {
      setAddingClient(false);
    }
  };

  // Open edit modal
  const openEditModal = (client: OpenVPNClientInfo) => {
    setEditingClient(client);
    editForm.setFieldsValue({
      password: client.password || '',
      enable: client.enable,
      totalGB: client.total_gb,
      expiryDate: client.expiry_time > 0 ? dayjs(client.expiry_time) : null,
      limitIp: client.limit_ip,
    });
    setIsEditModalOpen(true);
  };

  // Save edit handler
  const handleSaveEdit = async (values: {
    password?: string;
    enable: boolean;
    totalGB: number;
    expiryDate?: Dayjs | null;
    limitIp: number;
  }) => {
    if (!editingClient) return;

    setSavingEdit(true);
    try {
      const expiryTime = values.expiryDate ? values.expiryDate.valueOf() : 0;
      const res = await HttpUtil.post('/panel/api/openvpn/client/update', {
        name: editingClient.name,
        password: values.password ? values.password.trim() : '',
        enable: values.enable,
        totalGB: values.totalGB || 0,
        expiryTime,
        limitIp: values.limitIp || 0,
      });

      if (res && res.success) {
        messageApi.success(`تنظیمات کاربر ${editingClient.name} ذخیره شد`);
        setIsEditModalOpen(false);
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در ویرایش کاربر');
      }
    } catch (e: unknown) {
      const err = e as { message?: string };
      messageApi.error(err?.message || 'خطا در ذخیره تغییرات');
    } finally {
      setSavingEdit(false);
    }
  };

  // Quick enable/disable toggle
  const handleToggleEnable = async (client: OpenVPNClientInfo, newEnable: boolean) => {
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/client/update', {
        name: client.name,
        password: client.password || '',
        enable: newEnable,
        totalGB: client.total_gb,
        expiryTime: client.expiry_time,
        limitIp: client.limit_ip,
      });
      if (res && res.success) {
        messageApi.success(`کاربر ${client.name} ${newEnable ? 'فعال' : 'غیرفعال'} شد`);
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در تغییر وضعیت');
      }
    } catch {
      messageApi.error('خطا در تغییر وضعیت');
    }
  };

  // Reset traffic handler
  const handleResetTraffic = async (name: string) => {
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/client/reset', { name });
      if (res && res.success) {
        messageApi.success(`ترافیک مصرفی ${name} صفر شد`);
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در ریست ترافیک');
      }
    } catch {
      messageApi.error('خطا در ریست ترافیک');
    }
  };

  // Delete client handler
  const handleDeleteClient = async (name: string) => {
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/client/del', { name });
      if (res && res.success) {
        messageApi.success(`کاربر ${name} حذف شد`);
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در حذف کاربر');
      }
    } catch (e: unknown) {
      const err = e as { message?: string };
      messageApi.error(err?.message || 'خطا در برقراری ارتباط');
    }
  };

  // Disconnect active session
  const handleKillSession = async (name: string) => {
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/client/kill', { name });
      if (res && res.success) {
        messageApi.success(`اتصال کاربر ${name} قطع شد`);
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در قطع اتصال');
      }
    } catch {
      messageApi.error('خطا در قطع اتصال');
    }
  };

  const handleRestart = async () => {
    setRestarting(true);
    try {
      const res = await HttpUtil.post('/panel/api/openvpn/restart');
      if (res && res.success) {
        messageApi.success('سرویس OpenVPN با موفقیت ری‌استارت شد');
        fetchStatus();
      } else {
        messageApi.error(res?.msg || 'خطا در ری‌استارت سرویس');
      }
    } catch {
      messageApi.error('خطا در اجرای دستور ری‌استارت');
    } finally {
      setRestarting(false);
    }
  };

  const triggerDownload = async (name: string, host: string, port: number, inlineAuth = false) => {
    const h = host.trim() || activeHost;
    const p = port > 0 ? port : activePort;
    const inlineParam = inlineAuth ? '&inline=1' : '';
    try {
      messageApi.loading({ content: `در حال دریافت فایل کانفیگ ${name}...`, key: `dl-${name}` });
      const res = await HttpUtil.get<{ config: string }>(
        `/panel/api/openvpn/client/config?client=${encodeURIComponent(name)}&ip=${encodeURIComponent(h)}&port=${p}&raw=true${inlineParam}`,
      );
      const conf = res?.obj?.config;
      if (conf) {
        const blob = new Blob([conf], { type: 'application/x-openvpn-profile' });
        const blobUrl = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = blobUrl;
        a.download = `${name}_${h}_${p}.ovpn`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(blobUrl);
        messageApi.success({ content: `فایل ${name}_${h}_${p}.ovpn با موفقیت دانلود شد ✨`, key: `dl-${name}` });
      } else {
        messageApi.error({ content: 'خطا: فایل کانفیگ خالی است یا تولید نشد', key: `dl-${name}` });
      }
    } catch {
      messageApi.error({ content: 'خطا در دانلود فایل کانفیگ OpenVPN', key: `dl-${name}` });
    }
  };

  const copyConfigText = async (name: string, host: string, port: number, inlineAuth = false) => {
    const h = host.trim() || activeHost;
    const p = port > 0 ? port : activePort;
    const inlineParam = inlineAuth ? '&inline=1' : '';
    try {
      const res = await HttpUtil.get<{ config: string }>(
        `/panel/api/openvpn/client/config?client=${encodeURIComponent(name)}&ip=${encodeURIComponent(h)}&port=${p}&raw=true${inlineParam}`,
      );
      if (res?.obj?.config) {
        await navigator.clipboard.writeText(res.obj.config);
        messageApi.success(`کانفیگ ${name} (${h}:${p}) در کلیپ‌بورد کپی شد`);
      } else {
        messageApi.error('خطا در دریافت متن کانفیگ');
      }
    } catch {
      messageApi.error('خطا در کپی کانفیگ');
    }
  };

  const openConfigModal = async (client: OpenVPNClientInfo) => {
    setConfigModal({
      open: true,
      clientName: client.name,
      password: client.password || '',
      host: activeHost,
      portStr: activePort.toString(),
      inlineAuth: false,
      previewContent: '',
      loadingPreview: true,
    });

    try {
      const res = await HttpUtil.get<{ config: string }>(
        `/panel/api/openvpn/client/config?client=${encodeURIComponent(client.name)}&ip=${encodeURIComponent(activeHost)}&port=${activePort}&raw=true`,
      );
      const conf = res?.obj?.config;
      if (conf) {
        setConfigModal((prev) => ({
          ...prev,
          previewContent: conf,
          loadingPreview: false,
        }));
      }
    } catch {
      setConfigModal((prev) => ({ ...prev, loadingPreview: false }));
    }
  };

  const refreshModalPreview = async (host: string, portStr: string, inlineAuth: boolean) => {
    if (!configModal.clientName) return;
    const p = parseInt(portStr, 10) || activePort;
    setConfigModal((prev) => ({ ...prev, loadingPreview: true }));
    try {
      const inlineParam = inlineAuth ? '&inline=1' : '';
      const res = await HttpUtil.get<{ config: string }>(
        `/panel/api/openvpn/client/config?client=${encodeURIComponent(configModal.clientName)}&ip=${encodeURIComponent(host)}&port=${p}&raw=true${inlineParam}`,
      );
      const conf = res?.obj?.config;
      if (conf) {
        setConfigModal((prev) => ({
          ...prev,
          host,
          portStr,
          inlineAuth,
          previewContent: conf,
          loadingPreview: false,
        }));
      }
    } catch {
      setConfigModal((prev) => ({ ...prev, loadingPreview: false }));
    }
  };

  const copyAccountDetails = () => {
    const port = parseInt(configModal.portStr, 10) || activePort;
    const text =
      `🌐 مشخصات اکانت OpenVPN:\n` +
      `👤 نام کاربری: ${configModal.clientName}\n` +
      (configModal.password ? `🔑 رمز عبور: ${configModal.password}\n` : '') +
      `🌍 آدرس سرور: ${configModal.host}\n` +
      `🔌 پورت: ${port}\n` +
      `⚡ پروتکل: TCP`;
    navigator.clipboard.writeText(text);
    messageApi.success('اطلاعات اکانت برای ارسال کپی شد');
  };

  const columns: ColumnsType<OpenVPNClientInfo> = [
    {
      title: 'کاربر و مشخصات احراز',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record) => (
        <Space direction="vertical" size={2}>
          <Space size={6}>
            <Text strong style={{ color: '#00e5ff', fontSize: 14 }}>
              {name}
            </Text>
            <Button
              type="text"
              size="small"
              icon={<CopyOutlined />}
              title="کپی نام کاربری"
              onClick={() => {
                navigator.clipboard.writeText(name);
                messageApi.success('نام کاربری کپی شد');
              }}
            />
          </Space>
          {record.password ? (
            <Space size={4} align="center">
              <KeyOutlined style={{ fontSize: 11, color: '#f59e0b' }} />
              <Text type="secondary" style={{ fontSize: 11 }}>
                رمز:
              </Text>
              <Text code style={{ fontSize: 11, letterSpacing: '0.5px' }}>
                {showPasswords[record.name] ? record.password : '••••••••'}
              </Text>
              <Button
                type="text"
                size="small"
                style={{ padding: '0 4px', height: 20 }}
                icon={
                  showPasswords[record.name] ? (
                    <EyeInvisibleOutlined style={{ fontSize: 11 }} />
                  ) : (
                    <EyeOutlined style={{ fontSize: 11 }} />
                  )
                }
                onClick={() => toggleShowPassword(record.name)}
              />
              <Button
                type="text"
                size="small"
                style={{ padding: '0 4px', height: 20 }}
                icon={<CopyOutlined style={{ fontSize: 11 }} />}
                onClick={() => {
                  navigator.clipboard.writeText(record.password || '');
                  messageApi.success('رمز عبور کپی شد');
                }}
              />
            </Space>
          ) : (
            <Text type="secondary" style={{ fontSize: 11, fontStyle: 'italic' }}>
              احراز با گواهی (بدون پسورد)
            </Text>
          )}
          <Text type="secondary" style={{ fontSize: 10 }}>
            {record.created_at || '—'}
          </Text>
        </Space>
      ),
    },
    {
      title: 'وضعیت حساب',
      key: 'enable',
      render: (_, record) => (
        <Space direction="vertical" size={4} align="center">
          <Switch
            size="small"
            checked={record.enable}
            onChange={(checked) => handleToggleEnable(record, checked)}
          />
          {record.enable ? (
            <Text style={{ fontSize: 11, color: '#10b981' }}>فعال</Text>
          ) : (
            <Text style={{ fontSize: 11, color: '#ef4444' }}>مسدود</Text>
          )}
        </Space>
      ),
    },
    {
      title: 'وضعیت اتصال',
      key: 'status',
      render: (_, record) =>
        record.is_connected ? (
          <Space direction="vertical" size={2}>
            <div className="openvpn-online-badge">
              <span className="openvpn-pulse-dot" />
              <span>آنلاین ({record.current_connections})</span>
            </div>
            {record.connected_ips && record.connected_ips.length > 0 && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                {record.connected_ips.join(', ')}
              </Text>
            )}
          </Space>
        ) : (
          <div className="openvpn-offline-badge">
            <span
              style={{
                width: 7,
                height: 7,
                borderRadius: '50%',
                backgroundColor: '#64748b',
                display: 'inline-block',
              }}
            />
            <span>آفلاین</span>
          </div>
        ),
    },
    {
      title: 'مصرف حجم و سقف مجاز',
      key: 'traffic',
      render: (_, record) => {
        const isUnlimited = record.total_gb <= 0;
        return (
          <div style={{ minWidth: 140 }}>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                fontSize: 12,
                marginBottom: 2,
              }}
            >
              <Text strong>{record.traffic_used_str}</Text>
              <Text type="secondary">{record.traffic_limit_str}</Text>
            </div>
            {!isUnlimited ? (
              <Progress
                percent={Math.round(record.traffic_percent)}
                size="small"
                status={
                  record.is_traffic_exceeded
                    ? 'exception'
                    : record.traffic_percent > 85
                      ? 'normal'
                      : 'active'
                }
                strokeColor={
                  record.is_traffic_exceeded
                    ? '#ef4444'
                    : record.traffic_percent > 85
                      ? '#f59e0b'
                      : '#00e5ff'
                }
              />
            ) : (
              <span className="openvpn-unlimited-tag">نامحدود</span>
            )}
            {record.is_traffic_exceeded && (
              <Tag color="error" style={{ fontSize: 10, marginTop: 2 }}>
                حجم تمام شده
              </Tag>
            )}
          </div>
        );
      },
    },
    {
      title: 'تاریخ انقضا',
      key: 'expiry',
      render: (_, record) => {
        if (record.expiry_time <= 0) {
          return <span className="openvpn-unlimited-tag">نامحدود</span>;
        }
        const nowMs = Date.now();
        const diffDays = Math.ceil((record.expiry_time - nowMs) / (1000 * 60 * 60 * 24));
        const formatted = dayjs(record.expiry_time).format('YYYY-MM-DD');

        if (record.is_expired) {
          return (
            <Space direction="vertical" size={2}>
              <Tag color="error">منقضی شده</Tag>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {formatted}
              </Text>
            </Space>
          );
        }

        return (
          <Space direction="vertical" size={2}>
            <Tag color={diffDays <= 3 ? 'warning' : 'success'}>{diffDays} روز باقیمانده</Tag>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {formatted}
            </Text>
          </Space>
        );
      },
    },
    {
      title: 'محدودیت IP',
      key: 'limit_ip',
      render: (_, record) =>
        record.limit_ip > 0 ? (
          <Tag color="purple">{record.limit_ip} دستگاه</Tag>
        ) : (
          <span className="openvpn-unlimited-tag">نامحدود</span>
        ),
    },
    {
      title: 'عملیات',
      key: 'actions',
      render: (_, record) => {
        return (
          <Space wrap size="small">
            <Button
              size="small"
              type="primary"
              icon={<DownloadOutlined />}
              className="openvpn-btn-action-download"
              onClick={() => triggerDownload(record.name, activeHost, activePort, false)}
            >
              دانلود فایل .ovpn
            </Button>

            <Tooltip title="مشاهده کانفیگ، تنظیمات IP و پورت، و کپی جزئیات">
              <Button
                size="small"
                icon={<EyeOutlined />}
                onClick={() => openConfigModal(record)}
              >
                کانفیگ
              </Button>
            </Tooltip>

            <Tooltip title="ویرایش اطلاعات و پسورد">
              <Button size="small" icon={<EditOutlined />} onClick={() => openEditModal(record)} />
            </Tooltip>

            <Tooltip title="ریست ترافیک مصرفی">
              <Popconfirm
                title={`آیا از ریست ترافیک '${record.name}' مطمئن هستید؟`}
                okText="بله، ریست کن"
                cancelText="انصراف"
                onConfirm={() => handleResetTraffic(record.name)}
              >
                <Button size="small" icon={<UndoOutlined />} />
              </Popconfirm>
            </Tooltip>

            {record.is_connected && (
              <Tooltip title="قطع اتصال فعال کاربر">
                <Popconfirm
                  title={`اتصال کاربر '${record.name}' قطع شود؟`}
                  okText="قطع اتصال"
                  cancelText="انصراف"
                  onConfirm={() => handleKillSession(record.name)}
                >
                  <Button size="small" danger icon={<StopOutlined />} />
                </Popconfirm>
              </Tooltip>
            )}

            <Popconfirm
              title={`آیا از حذف کاربر '${record.name}' مطمئن هستید؟`}
              description="گواهی باطل شده و کاربر از دیتابیس حذف خواهد شد."
              okText="بله، حذف کن"
              cancelText="انصراف"
              okButtonProps={{ danger: true }}
              onConfirm={() => handleDeleteClient(record.name)}
            >
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {contextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            {/* Cyber Hero Banner */}
            <div className="openvpn-hero">
              <div className="openvpn-hero-left">
                <Title level={2} className="openvpn-hero-title">
                  <SafetyOutlined
                    style={{
                      fontSize: 28,
                      color: '#00f2fe',
                      filter: 'drop-shadow(0 0 10px rgba(0, 242, 254, 0.7))',
                    }}
                  />
                  <span className="openvpn-hero-title-text">مدیریت OpenVPN</span>
                  <Tag
                    style={{
                      marginLeft: 8,
                      fontSize: 12,
                      padding: '2px 10px',
                      borderRadius: 12,
                      border: status?.running
                        ? '1px solid rgba(16, 185, 129, 0.4)'
                        : '1px solid rgba(239, 68, 68, 0.4)',
                      background: status?.running
                        ? 'rgba(16, 185, 129, 0.15)'
                        : 'rgba(239, 68, 68, 0.15)',
                      color: status?.running ? '#10b981' : '#ef4444',
                    }}
                  >
                    {status?.running ? 'هسته آنلاین' : 'هسته متوقف'}
                  </Tag>
                </Title>
                <Text className="openvpn-hero-subtitle">
                  مدیریت کاربران، نام کاربری و رمز عبور، کنترل ترافیک، محدودیت آی‌پی و صدور کانفیگ
                  تانل‌شده
                </Text>
              </div>
              <div className="openvpn-hero-actions">
                <Space wrap>
                  <Button icon={<ReloadOutlined spin={loading} />} onClick={fetchStatus}>
                    بروزرسانی
                  </Button>
                  <Button
                    icon={<SyncOutlined spin={restarting} />}
                    loading={restarting}
                    onClick={handleRestart}
                  >
                    ری‌استارت هسته
                  </Button>
                  <Button
                    type="primary"
                    className="openvpn-btn-add"
                    icon={<PlusOutlined />}
                    onClick={() => {
                      addForm.resetFields();
                      addForm.setFieldsValue({
                        totalGB: 0,
                        limitIp: 0,
                        password: generateRandomPassword(),
                      });
                      setIsAddModalOpen(true);
                    }}
                  >
                    ساخت کاربر جدید
                  </Button>
                </Space>
              </div>
            </div>

            {/* Stat Cards */}
            <Row gutter={[16, 16]} style={{ marginBottom: 20 }}>
              <Col xs={24} sm={12} md={6}>
                <Card className="openvpn-card openvpn-stat-card">
                  <Text type="secondary">وضعیت هسته OpenVPN</Text>
                  <div
                    className="openvpn-stat-value"
                    style={{
                      color: status?.running ? '#10b981' : '#ef4444',
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                    }}
                  >
                    <span
                      className="openvpn-pulse-dot"
                      style={{
                        width: 10,
                        height: 10,
                        backgroundColor: status?.running ? '#10b981' : '#ef4444',
                        boxShadow: status?.running ? '0 0 10px #10b981' : '0 0 10px #ef4444',
                      }}
                    />
                    <span>{status?.running ? 'آنلاین و فعال' : 'متوقف'}</span>
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={12} md={6}>
                <Card className="openvpn-card openvpn-stat-card">
                  <Text type="secondary">پورت لیسن هسته سرور</Text>
                  <div className="openvpn-stat-value">
                    {status?.port || 2083}{' '}
                    <span style={{ fontSize: 14, fontWeight: 500, color: '#94a3b8' }}>
                      {status?.proto?.toUpperCase() || 'TCP'}
                    </span>
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={12} md={6}>
                <Card className="openvpn-card openvpn-stat-card">
                  <Text type="secondary">کاربران متصل آنلاین</Text>
                  <div
                    className="openvpn-stat-value"
                    style={{
                      background: 'linear-gradient(135deg, #10b981 0%, #34d399 100%)',
                      WebkitBackgroundClip: 'text',
                      WebkitTextFillColor: 'transparent',
                    }}
                  >
                    {status?.online_clients || 0}
                  </div>
                </Card>
              </Col>
              <Col xs={24} sm={12} md={6}>
                <Card className="openvpn-card openvpn-stat-card">
                  <Text type="secondary">کل کاربران</Text>
                  <div
                    className="openvpn-stat-value"
                    style={{
                      background: 'linear-gradient(135deg, #e2e8f0 0%, #94a3b8 100%)',
                      WebkitBackgroundClip: 'text',
                      WebkitTextFillColor: 'transparent',
                    }}
                  >
                    {status?.total_clients || 0}
                  </div>
                </Card>
              </Col>
            </Row>

            {/* Dedicated Connection & Tunnel Endpoint Box */}
            <Card
              className="openvpn-card"
              style={{ marginBottom: 20 }}
              title={
                <Space align="center">
                  <SettingOutlined style={{ color: '#00e5ff' }} />
                  <span>تنظیمات آدرس، دامنه و پورت تانل برای کانفیگ‌های خروجی</span>
                  <span dir="ltr" className="openvpn-ovpn-tag">
                    .ovpn
                  </span>
                </Space>
              }
            >
              <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                آدرس و پورتی که می‌خواهید در خط <Text code>remote</Text> کانفیگ‌ها قرار بگیرد را اینجا
                مشخص کنید. می‌توانید از آی‌پی سرور ایران، پورت تانل‌شده، یا دامنه دلخواه خود استفاده
                نمایید:
              </Paragraph>

              <Row gutter={[16, 16]} align="middle">
                <Col xs={24} sm={14} md={10}>
                  <Text strong style={{ display: 'block', marginBottom: 6 }}>
                    آدرس ریموت (دامنه، ساب‌دامین یا آی‌پی تانل/سرور)
                  </Text>
                  <Input
                    dir="ltr"
                    style={{ direction: 'ltr', textAlign: 'left' }}
                    prefix={<GlobalOutlined style={{ color: '#00e5ff' }} />}
                    value={endpointHost}
                    placeholder="vpn.domain.com / 37.202.234.149"
                    onChange={(e) => handleHostChange(e.target.value)}
                    allowClear
                  />
                </Col>
                <Col xs={24} sm={10} md={6}>
                  <Text strong style={{ display: 'block', marginBottom: 6 }}>
                    پورت ریموت (پورت تانل‌شده یا پورت سرور)
                  </Text>
                  <Input
                    dir="ltr"
                    inputMode="numeric"
                    style={{ width: '100%', direction: 'ltr', textAlign: 'left' }}
                    prefix={<ThunderboltOutlined style={{ color: '#f59e0b' }} />}
                    value={portInput}
                    placeholder="مثلاً 8443 یا 443"
                    onChange={(e) => handlePortInputChange(e.target.value)}
                    allowClear
                  />
                </Col>
                <Col xs={24} md={8}>
                  <Text strong style={{ display: 'block', marginBottom: 6 }}>
                    انتخاب سریع پیش‌فرض‌ها
                  </Text>
                  <div className="openvpn-presets-bar">
                    <Button
                      size="small"
                      className="openvpn-preset-chip"
                      onClick={() => {
                        if (status?.direct_ip) handleHostChange(status.direct_ip);
                        setPresetPort(status?.port || 2083);
                      }}
                    >
                      ⚡ مستقیم ({status?.direct_ip || '91.107.165.105'}:2083)
                    </Button>
                    {status?.iran_ip && (
                      <Button
                        size="small"
                        className="openvpn-preset-chip"
                        onClick={() => {
                          handleHostChange(status.iran_ip);
                          setPresetPort(8443);
                        }}
                      >
                        🇮🇷 سرور ایران ({status.iran_ip}:8443)
                      </Button>
                    )}
                    <Button
                      size="small"
                      className="openvpn-preset-chip"
                      onClick={() => setPresetPort(443)}
                    >
                      پورت 443
                    </Button>
                    <Button
                      size="small"
                      className="openvpn-preset-chip"
                      onClick={() => setPresetPort(8443)}
                    >
                      پورت 8443
                    </Button>
                  </div>
                </Col>
              </Row>

              <div
                style={{
                  marginTop: 16,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  flexWrap: 'wrap',
                  gap: 12,
                }}
              >
                <div>
                  <Tag color="cyan" style={{ padding: '6px 12px', fontSize: 13 }}>
                    دستور ریموت فعال:{' '}
                    <strong style={{ letterSpacing: '0.5px' }}>
                      remote {activeHost} {activePort}
                    </strong>
                  </Tag>
                  <Text type="secondary" style={{ fontSize: 12, marginRight: 8 }}>
                    (تمام کانفیگ‌های خروجی با این آدرس و پورت تولید خواهند شد)
                  </Text>
                </div>
                <Button
                  type="primary"
                  icon={<SaveOutlined />}
                  loading={savingSettings}
                  onClick={handleSaveSettings}
                  style={{
                    background: 'linear-gradient(135deg, #00e5ff 0%, #7928ca 100%)',
                    border: 'none',
                    fontWeight: 600,
                    boxShadow: '0 0 12px rgba(0, 229, 255, 0.35)',
                  }}
                >
                  ذخیره دائمی تنظیمات در پنل
                </Button>
              </div>
            </Card>

            {/* Clients Table (Desktop) & Cards (Mobile) */}
            <Card
              className="openvpn-card"
              title="لیست کلاینت‌ها و کاربران OpenVPN"
              extra={
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => setIsAddModalOpen(true)}
                  style={{
                    background: 'linear-gradient(135deg, #00f2fe 0%, #38bdf8 100%)',
                    border: 'none',
                    color: '#000',
                    fontWeight: 600,
                  }}
                >
                  افزودن کاربر
                </Button>
              }
            >
              {/* Desktop Table View */}
              <div className="openvpn-desktop-table">
                <Table
                  dataSource={status?.clients || []}
                  columns={columns}
                  rowKey="name"
                  loading={loading}
                  pagination={{ pageSize: 15 }}
                  scroll={{ x: 800 }}
                  locale={{ emptyText: 'هیچ کلاینتی هنوز ساخته نشده است' }}
                />
              </div>

              {/* Mobile Cards View */}
              <div className="openvpn-mobile-cards">
                {(!status?.clients || status.clients.length === 0) ? (
                  <Empty description="هیچ کلاینتی هنوز ساخته نشده است" />
                ) : (
                  status.clients.map((client) => {
                    const isUnlimited = client.total_gb <= 0;
                    return (
                      <div key={client.name} className="openvpn-mobile-client-card">
                        <div className="openvpn-mobile-client-header">
                          <div className="openvpn-mobile-client-user">
                            <Avatar
                              icon={<UserOutlined />}
                              style={{ backgroundColor: '#00f2fe', color: '#000', fontWeight: 'bold' }}
                            />
                            <div>
                              <span className="openvpn-mobile-username">{client.name}</span>
                              <div style={{ marginTop: 2 }}>
                                {client.is_connected ? (
                                  <Tag color="success">🟢 آنلاین ({client.current_connections})</Tag>
                                ) : (
                                  <Tag color="default">⚫ آفلاین</Tag>
                                )}
                                {client.password && (
                                  <Tag color="processing" style={{ fontSize: 10 }}>🔑 دارای رمز</Tag>
                                )}
                              </div>
                            </div>
                          </div>
                          <Space direction="vertical" size={2} align="end">
                            <Switch
                              size="small"
                              checked={client.enable}
                              onChange={(checked) => handleToggleEnable(client, checked)}
                            />
                            <Text style={{ fontSize: 11, color: client.enable ? '#10b981' : '#ef4444' }}>
                              {client.enable ? 'فعال' : 'مسدود'}
                            </Text>
                          </Space>
                        </div>

                        <div className="openvpn-mobile-client-stats">
                          <div className="openvpn-mobile-stat-row">
                            <span className="stat-label">📊 مصرف حجم:</span>
                            <span className="stat-val">
                              {client.traffic_used_str} / {isUnlimited ? 'نامحدود' : client.traffic_limit_str}
                            </span>
                          </div>
                          {!isUnlimited && (
                            <Progress
                              percent={Math.round(client.traffic_percent)}
                              size="small"
                              status={client.is_traffic_exceeded ? 'exception' : 'active'}
                              strokeColor={client.is_traffic_exceeded ? '#ef4444' : '#00f2fe'}
                            />
                          )}
                          <div className="openvpn-mobile-stat-row" style={{ marginTop: 4 }}>
                            <span className="stat-label">⏳ انقضا:</span>
                            <span className="stat-val">
                              {client.expiry_time <= 0
                                ? 'نامحدود'
                                : client.is_expired
                                  ? 'منقضی شده'
                                  : dayjs(client.expiry_time).format('YYYY-MM-DD')}
                            </span>
                          </div>
                        </div>

                        {/* Direct Download Button (Mobile Hero Action) */}
                        <Button
                          type="primary"
                          block
                          size="large"
                          icon={<DownloadOutlined />}
                          className="openvpn-mobile-btn-download"
                          onClick={() => triggerDownload(client.name, activeHost, activePort, false)}
                        >
                          ⬇️ دانلود مستقیم فایل کانفیگ .ovpn
                        </Button>

                        {/* Secondary Actions on Mobile */}
                        <div className="openvpn-mobile-actions-secondary">
                          <Button
                            icon={<EyeOutlined />}
                            className="openvpn-mobile-btn-sub"
                            onClick={() => openConfigModal(client)}
                          >
                            مشاهده / QR
                          </Button>
                          <Button
                            icon={<EditOutlined />}
                            className="openvpn-mobile-btn-sub"
                            onClick={() => openEditModal(client)}
                          >
                            ویرایش
                          </Button>
                          <Popconfirm
                            title="حذف کلاینت OpenVPN"
                            description={`آیا از حذف ${client.name} مطمئن هستید؟`}
                            onConfirm={() => handleDeleteClient(client.name)}
                            okText="بله"
                            cancelText="خیر"
                            okButtonProps={{ danger: true }}
                          >
                            <Button
                              danger
                              icon={<DeleteOutlined />}
                              className="openvpn-mobile-btn-sub"
                            >
                              حذف
                            </Button>
                          </Popconfirm>
                        </div>
                      </div>
                    );
                  })
                )}
              </div>
            </Card>

            {/* Add Client Modal */}
            <Modal
              title={
                <Space>
                  <UserOutlined style={{ color: '#00e5ff' }} />
                  <span>ساخت کاربر جدید OpenVPN</span>
                </Space>
              }
              open={isAddModalOpen}
              onCancel={() => setIsAddModalOpen(false)}
              footer={null}
              destroyOnClose
              width={620}
            >
              <Form
                form={addForm}
                layout="vertical"
                onFinish={handleAddClient}
                initialValues={{ totalGB: 0, limitIp: 0 }}
              >
                <Form.Item
                  name="name"
                  label="نام کاربری (حروف انگلیسی بدون فاصله)"
                  rules={[
                    { required: true, message: 'لطفاً نام کاربر را وارد کنید' },
                    {
                      pattern: /^[a-zA-Z0-9_-]{1,32}$/,
                      message: 'فقط حروف انگلیسی، عدد، خط فاصله و زیرخط مجاز است',
                    },
                  ]}
                >
                  <Input placeholder="مثال: user1" autoFocus />
                </Form.Item>

                {/* Password */}
                <Form.Item
                  name="password"
                  label={
                    <Space>
                      <KeyOutlined style={{ color: '#f59e0b' }} />
                      <span>رمز عبور (Password)</span>
                    </Space>
                  }
                >
                  <Space.Compact style={{ width: '100%' }}>
                    <Input placeholder="رمز عبور کاربر (برای احراز هویت با پسورد)" />
                    <Button
                      onClick={() => addForm.setFieldsValue({ password: generateRandomPassword() })}
                    >
                      تولید رمز تصادفی
                    </Button>
                  </Space.Compact>
                </Form.Item>

                {/* Traffic Limit (GB) */}
                <Form.Item
                  name="totalGB"
                  label={
                    <Space>
                      <DashboardOutlined style={{ color: '#00e5ff' }} />
                      <span>محدودیت حجم کل (گیگابایت - 0 به معنی نامحدود است)</span>
                    </Space>
                  }
                >
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="0 = نامحدود" />
                </Form.Item>
                <div style={{ marginTop: -14, marginBottom: 16 }}>
                  <Space wrap size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      انتخاب سریع حجم:
                    </Text>
                    {[0, 10, 20, 30, 50, 100].map((gb) => (
                      <Button
                        key={gb}
                        size="small"
                        onClick={() => addForm.setFieldsValue({ totalGB: gb })}
                      >
                        {gb === 0 ? 'نامحدود' : `${gb} GB`}
                      </Button>
                    ))}
                  </Space>
                </div>

                {/* Expiry Date */}
                <Form.Item
                  name="expiryDate"
                  label={
                    <Space>
                      <ClockCircleOutlined style={{ color: '#10b981' }} />
                      <span>تاریخ انقضا (خالی = نامحدود)</span>
                    </Space>
                  }
                >
                  <DatePicker
                    showTime
                    style={{ width: '100%' }}
                    placeholder="انتخاب تاریخ و ساعت انقضا"
                  />
                </Form.Item>
                <div style={{ marginTop: -14, marginBottom: 16 }}>
                  <Space wrap size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      انتخاب سریع زمان:
                    </Text>
                    <Button
                      size="small"
                      onClick={() => addForm.setFieldsValue({ expiryDate: null })}
                    >
                      نامحدود
                    </Button>
                    <Button
                      size="small"
                      onClick={() => addForm.setFieldsValue({ expiryDate: dayjs().add(15, 'day') })}
                    >
                      15 روز
                    </Button>
                    <Button
                      size="small"
                      onClick={() => addForm.setFieldsValue({ expiryDate: dayjs().add(30, 'day') })}
                    >
                      1 ماه (30 روز)
                    </Button>
                    <Button
                      size="small"
                      onClick={() => addForm.setFieldsValue({ expiryDate: dayjs().add(60, 'day') })}
                    >
                      2 ماه
                    </Button>
                    <Button
                      size="small"
                      onClick={() => addForm.setFieldsValue({ expiryDate: dayjs().add(90, 'day') })}
                    >
                      3 ماه
                    </Button>
                  </Space>
                </div>

                {/* IP Limit */}
                <Form.Item
                  name="limitIp"
                  label={
                    <Space>
                      <GlobalOutlined style={{ color: '#f59e0b' }} />
                      <span>محدودیت تعداد آی‌پی همزمان (0 = نامحدود)</span>
                    </Space>
                  }
                >
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="0 = نامحدود" />
                </Form.Item>
                <div style={{ marginTop: -14, marginBottom: 24 }}>
                  <Space wrap size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      پیش‌فرض سریع:
                    </Text>
                    {[0, 1, 2, 3, 5].map((ip) => (
                      <Button
                        key={ip}
                        size="small"
                        onClick={() => addForm.setFieldsValue({ limitIp: ip })}
                      >
                        {ip === 0 ? 'نامحدود' : `${ip} دستگاه`}
                      </Button>
                    ))}
                  </Space>
                </div>

                <div style={{ textAlign: 'left' }}>
                  <Space>
                    <Button onClick={() => setIsAddModalOpen(false)}>انصراف</Button>
                    <Button
                      type="primary"
                      htmlType="submit"
                      loading={addingClient}
                      className="openvpn-btn-add"
                    >
                      ساخت کاربر
                    </Button>
                  </Space>
                </div>
              </Form>
            </Modal>

            {/* Edit Client Modal */}
            <Modal
              title={
                <Space>
                  <EditOutlined style={{ color: '#00e5ff' }} />
                  <span>
                    ویرایش کاربر:{' '}
                    <strong style={{ color: '#00e5ff' }}>{editingClient?.name}</strong>
                  </span>
                </Space>
              }
              open={isEditModalOpen}
              onCancel={() => setIsEditModalOpen(false)}
              footer={null}
              destroyOnClose
              width={620}
            >
              <Form form={editForm} layout="vertical" onFinish={handleSaveEdit}>
                {/* Password */}
                <Form.Item
                  name="password"
                  label={
                    <Space>
                      <KeyOutlined style={{ color: '#f59e0b' }} />
                      <span>رمز عبور (Password)</span>
                    </Space>
                  }
                >
                  <Space.Compact style={{ width: '100%' }}>
                    <Input placeholder="رمز عبور کاربر" />
                    <Button
                      onClick={() =>
                        editForm.setFieldsValue({ password: generateRandomPassword() })
                      }
                    >
                      تولید رمز جدید
                    </Button>
                  </Space.Compact>
                </Form.Item>

                {/* Enable Toggle */}
                <Form.Item name="enable" label="وضعیت فعال بودن اکانت" valuePropName="checked">
                  <Switch checkedChildren="فعال" unCheckedChildren="مسدود" />
                </Form.Item>

                {/* Total GB */}
                <Form.Item
                  name="totalGB"
                  label={
                    <Space>
                      <DashboardOutlined style={{ color: '#00e5ff' }} />
                      <span>سقف حجم کل (گیگابایت - 0 به معنی نامحدود است)</span>
                    </Space>
                  }
                >
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="0 = نامحدود" />
                </Form.Item>
                <div style={{ marginTop: -14, marginBottom: 16 }}>
                  <Space wrap size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      انتخاب سریع حجم:
                    </Text>
                    {[0, 10, 20, 30, 50, 100].map((gb) => (
                      <Button
                        key={gb}
                        size="small"
                        onClick={() => editForm.setFieldsValue({ totalGB: gb })}
                      >
                        {gb === 0 ? 'نامحدود' : `${gb} GB`}
                      </Button>
                    ))}
                  </Space>
                </div>

                {/* Expiry Date */}
                <Form.Item
                  name="expiryDate"
                  label={
                    <Space>
                      <ClockCircleOutlined style={{ color: '#10b981' }} />
                      <span>تاریخ انقضا (خالی = نامحدود)</span>
                    </Space>
                  }
                >
                  <DatePicker
                    showTime
                    style={{ width: '100%' }}
                    placeholder="انتخاب تاریخ و ساعت انقضا"
                  />
                </Form.Item>
                <div style={{ marginTop: -14, marginBottom: 16 }}>
                  <Space wrap size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      انتخاب سریع زمان:
                    </Text>
                    <Button
                      size="small"
                      onClick={() => editForm.setFieldsValue({ expiryDate: null })}
                    >
                      نامحدود
                    </Button>
                    <Button
                      size="small"
                      onClick={() =>
                        editForm.setFieldsValue({ expiryDate: dayjs().add(15, 'day') })
                      }
                    >
                      15 روز
                    </Button>
                    <Button
                      size="small"
                      onClick={() =>
                        editForm.setFieldsValue({ expiryDate: dayjs().add(30, 'day') })
                      }
                    >
                      1 ماه
                    </Button>
                    <Button
                      size="small"
                      onClick={() =>
                        editForm.setFieldsValue({ expiryDate: dayjs().add(60, 'day') })
                      }
                    >
                      2 ماه
                    </Button>
                    <Button
                      size="small"
                      onClick={() =>
                        editForm.setFieldsValue({ expiryDate: dayjs().add(90, 'day') })
                      }
                    >
                      3 ماه
                    </Button>
                  </Space>
                </div>

                {/* IP Limit */}
                <Form.Item
                  name="limitIp"
                  label={
                    <Space>
                      <GlobalOutlined style={{ color: '#f59e0b' }} />
                      <span>محدودیت تعداد آی‌پی همزمان (0 = نامحدود)</span>
                    </Space>
                  }
                >
                  <InputNumber style={{ width: '100%' }} min={0} placeholder="0 = نامحدود" />
                </Form.Item>
                <div style={{ marginTop: -14, marginBottom: 24 }}>
                  <Space wrap size="small">
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      پیش‌فرض سریع:
                    </Text>
                    {[0, 1, 2, 3, 5].map((ip) => (
                      <Button
                        key={ip}
                        size="small"
                        onClick={() => editForm.setFieldsValue({ limitIp: ip })}
                      >
                        {ip === 0 ? 'نامحدود' : `${ip} دستگاه`}
                      </Button>
                    ))}
                  </Space>
                </div>

                <div style={{ textAlign: 'left' }}>
                  <Space>
                    <Button onClick={() => setIsEditModalOpen(false)}>انصراف</Button>
                    <Button
                      type="primary"
                      htmlType="submit"
                      loading={savingEdit}
                      className="openvpn-btn-add"
                    >
                      ذخیره تغییرات
                    </Button>
                  </Space>
                </div>
              </Form>
            </Modal>

            {/* Config & Download Modal */}
            <Modal
              title={
                <Space>
                  <DownloadOutlined style={{ color: '#00e5ff' }} />
                  <span>
                    دریافت کانفیگ کاربر:{' '}
                    <strong style={{ color: '#00e5ff' }}>{configModal.clientName}</strong>
                  </span>
                </Space>
              }
              open={configModal.open}
              onCancel={() => setConfigModal((prev) => ({ ...prev, open: false }))}
              width={720}
              footer={[
                <Button key="share" icon={<ShareAltOutlined />} onClick={copyAccountDetails}>
                  کپی مشخصات اکانت
                </Button>,
                <Button
                  key="copy"
                  icon={<CopyOutlined />}
                  onClick={() =>
                    copyConfigText(
                      configModal.clientName,
                      configModal.host,
                      parseInt(configModal.portStr, 10) || activePort,
                      configModal.inlineAuth,
                    )
                  }
                >
                  کپی محتوای .ovpn
                </Button>,
                <Button
                  key="download"
                  type="primary"
                  icon={<DownloadOutlined />}
                  className="openvpn-btn-download"
                  onClick={() =>
                    triggerDownload(
                      configModal.clientName,
                      configModal.host,
                      parseInt(configModal.portStr, 10) || activePort,
                      configModal.inlineAuth,
                    )
                  }
                >
                  دانلود فایل .ovpn
                </Button>,
                <Button
                  key="close"
                  onClick={() => setConfigModal((prev) => ({ ...prev, open: false }))}
                >
                  بستن
                </Button>,
              ]}
            >
              {/* Account Credentials Card */}
              <Card
                size="small"
                style={{
                  background: 'rgba(0, 229, 255, 0.05)',
                  borderColor: 'rgba(0, 229, 255, 0.25)',
                  marginBottom: 16,
                }}
              >
                <Row gutter={[16, 8]} align="middle">
                  <Col xs={24} sm={12}>
                    <Space align="center">
                      <UserOutlined style={{ color: '#00e5ff' }} />
                      <Text type="secondary">نام کاربری (Username): </Text>
                      <Text strong style={{ color: '#00e5ff' }}>
                        {configModal.clientName}
                      </Text>
                      <Button
                        type="text"
                        size="small"
                        icon={<CopyOutlined />}
                        onClick={() => {
                          navigator.clipboard.writeText(configModal.clientName);
                          messageApi.success('نام کاربری کپی شد');
                        }}
                      />
                    </Space>
                  </Col>
                  <Col xs={24} sm={12}>
                    {configModal.password ? (
                      <Space align="center">
                        <KeyOutlined style={{ color: '#f59e0b' }} />
                        <Text type="secondary">رمز عبور (Password): </Text>
                        <Text code>{configModal.password}</Text>
                        <Button
                          type="text"
                          size="small"
                          icon={<CopyOutlined />}
                          onClick={() => {
                            navigator.clipboard.writeText(configModal.password);
                            messageApi.success('رمز عبور کپی شد');
                          }}
                        />
                      </Space>
                    ) : (
                      <Text type="secondary" style={{ fontStyle: 'italic' }}>
                        بدون رمز عبور (فقط گواهی)
                      </Text>
                    )}
                  </Col>
                </Row>
              </Card>

              <div style={{ marginBottom: 16 }}>
                <Paragraph type="secondary" style={{ marginBottom: 10 }}>
                  می‌توانید برای این کاربر آدرس ریموت (دامنه یا آی‌پی تانل) و پورت اتصال را تغییر
                  دهید:
                </Paragraph>
                <Row gutter={[12, 12]} align="middle">
                  <Col xs={24} sm={14}>
                    <Text strong style={{ display: 'block', marginBottom: 4 }}>
                      آدرس ریموت (دامنه یا آی‌پی):
                    </Text>
                    <Input
                      value={configModal.host}
                      placeholder="دامنه یا آی‌پی سرور"
                      onChange={(e) => {
                        const h = e.target.value.trim();
                        setConfigModal((prev) => ({ ...prev, host: h }));
                      }}
                    />
                  </Col>
                  <Col xs={16} sm={6}>
                    <Text strong style={{ display: 'block', marginBottom: 4 }}>
                      پورت تانل/سرور:
                    </Text>
                    <Input
                      dir="ltr"
                      inputMode="numeric"
                      style={{ width: '100%', direction: 'ltr', textAlign: 'left' }}
                      value={configModal.portStr}
                      placeholder="مثلاً 8443 یا 443"
                      onChange={(e) => {
                        const val = e.target.value.replace(/[^0-9]/g, '');
                        setConfigModal((prev) => ({ ...prev, portStr: val }));
                      }}
                      allowClear
                    />
                  </Col>
                  <Col xs={8} sm={4}>
                    <Button
                      style={{ marginTop: 22, width: '100%' }}
                      icon={<EyeOutlined />}
                      loading={configModal.loadingPreview}
                      onClick={() =>
                        refreshModalPreview(
                          configModal.host,
                          configModal.portStr,
                          configModal.inlineAuth,
                        )
                      }
                    >
                      اعمال
                    </Button>
                  </Col>
                </Row>
              </div>

              {configModal.password && (
                <div style={{ marginBottom: 12 }}>
                  <Checkbox
                    checked={configModal.inlineAuth}
                    onChange={(e) => {
                      const checked = e.target.checked;
                      setConfigModal((prev) => ({ ...prev, inlineAuth: checked }));
                      refreshModalPreview(configModal.host, configModal.portStr, checked);
                    }}
                  >
                    <Text strong style={{ color: '#00e5ff' }}>
                      قرار دادن نام کاربری و رمز عبور داخل فایل .ovpn (اتصال سریع و خودکار)
                    </Text>
                  </Checkbox>
                  <div style={{ fontSize: 12, color: '#94a3b8', marginRight: 24, marginTop: 2 }}>
                    در صورت فعال بودن این گزینه، رمز عبور مستقیماً در تگ &lt;auth-user-pass&gt; داخل
                    فایل قرار می‌گیرد و نرم‌افزار هنگام اتصال پسورد را نمی‌پرسد.
                  </div>
                </div>
              )}

              <div style={{ marginTop: 12 }}>
                <Text strong style={{ display: 'block', marginBottom: 6 }}>
                  پیش‌نمایش سورس کانفیگ (.ovpn):
                </Text>
                <pre className="openvpn-code-snippet">
                  {configModal.loadingPreview
                    ? 'در حال دریافت پیش‌نمایش...'
                    : configModal.previewContent}
                </pre>
              </div>
            </Modal>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
