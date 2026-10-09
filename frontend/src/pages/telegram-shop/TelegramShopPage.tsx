import { useState, useEffect, useMemo } from 'react';
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
}

export default function TelegramShopPage() {
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const [form] = Form.useForm();

  const [loading, setLoading] = useState(true);
  const [installing, setInstalling] = useState(false);
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
      message.error(err?.message || 'خطا در واکشی وضعیت تلگرام شاپ');
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
      await HttpUtil.post('/panel/api/telegram-shop/install', values);
      message.success('ربات تلگرام شاپ با موفقیت نصب و فعال شد! 🚀');
      fetchStatus();
    } catch (err: any) {
      message.error(err?.message || 'خطا در نصب ربات تلگرام شاپ');
    } finally {
      setInstalling(false);
    }
  };

  const handleAction = async (action: 'start' | 'stop' | 'restart' | 'uninstall') => {
    try {
      setActionLoading(true);
      await HttpUtil.post('/panel/api/telegram-shop/action', { action });
      if (action === 'uninstall') {
        message.warning('ربات تلگرام شاپ با موفقیت حذف گردید.');
        form.resetFields();
      } else {
        message.success(`دستور ${action} با موفقیت اجرا شد.`);
      }
      fetchStatus();
    } catch (err: any) {
      message.error(err?.message || 'خطا در اجرای عملیات');
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
                    <div dir="rtl" style={{ textAlign: 'right' }}>
                      <Title level={2} className="tgshop-hero-title-text" style={{ margin: 0 }}>
                        <span>تلگرام شاپ</span>
                        <span style={{ fontSize: '0.8em', opacity: 0.85, fontWeight: 500, direction: 'ltr', display: 'inline-block', marginRight: 6 }}>
                          (Telegram Shop)
                        </span>
                      </Title>
                      <Paragraph style={{ margin: 0, color: '#94a3b8', fontSize: 13, marginTop: 4 }}>
                        نصب، مدیریت و حذف آسان ربات فروش و تست خودکار اشتراک مستقیماً از داخل پنل
                      </Paragraph>
                    </div>
                  </Space>
                </Col>
                <Col xs={24} md={8} style={{ textAlign: 'right' }}>
                  <Space wrap>
                    <Button icon={<ReloadOutlined />} onClick={fetchStatus} loading={loading}>
                      بروزرسانی وضعیت
                    </Button>
                    {status?.installed && (
                      <span className={status.running ? 'tgshop-badge-online' : 'tgshop-badge-offline'}>
                        <span className="tgshop-dot" style={{ backgroundColor: status.running ? '#10b981' : '#ef4444' }} />
                        {status.running ? 'ربات فعال و آنلاین' : 'ربات متوقف است'}
                      </span>
                    )}
                  </Space>
                </Col>
              </Row>
            </div>

            {loading ? (
              <div style={{ textAlign: 'center', padding: '60px 0' }}>
                <Spin size="large" />
                <div style={{ marginTop: 12, color: '#94a3b8' }}>در حال بارگذاری وضعیت...</div>
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
                        <span style={{ color: '#fff', fontWeight: 700 }}>نصب و استقرار ربات (One-Click Installer)</span>
                      </Space>
                    }
                  >
                    <Alert
                      type="info"
                      showIcon
                      icon={<InfoCircleOutlined />}
                      style={{ marginBottom: 20, background: 'rgba(6, 182, 212, 0.08)', borderColor: 'rgba(6, 182, 212, 0.2)' }}
                      message="راه‌اندازی کاملاً مستقل و ایزوله"
                      description="با وارد کردن توکن و آیدی ادمین، ربات در قالب یک سرویس پرسرعت لینوکسی مستقر شده و مستقیماً کانفیگ‌ها را در هسته پنل نوا صادر می‌کند."
                    />

                    <Form form={form} layout="vertical" onFinish={handleInstall}>
                      <Form.Item
                        name="bot_token"
                        label={<span style={{ color: '#e2e8f0' }}>توکن ربات تلگرام (Bot Token)</span>}
                        extra="توکن دریافت شده از @BotFather"
                        rules={[{ required: true, message: 'لطفاً توکن ربات تلگرام را وارد کنید' }]}
                      >
                        <Input
                          placeholder="8781465649:AAEEKMK_umrtGoi8..."
                          prefix={<SendOutlined style={{ color: '#06b6d4' }} />}
                          size="large"
                        />
                      </Form.Item>

                      <Form.Item
                        name="admin_chat_id"
                        label={<span style={{ color: '#e2e8f0' }}>چت آیدی عددی ادمین (Admin Chat ID)</span>}
                        extra="آیدی عددی تلگرام شما جهت دریافت اعلانات و سفارشات (دریافت از @userinfobot)"
                        rules={[{ required: true, message: 'لطفاً آیدی عددی ادمین را وارد کنید' }]}
                      >
                        <Input
                          placeholder="842798945"
                          prefix={<UserOutlined style={{ color: '#8b5cf6' }} />}
                          size="large"
                        />
                      </Form.Item>

                      <Button
                        type="primary"
                        htmlType="submit"
                        size="large"
                        block
                        className="tgshop-btn-install"
                        loading={installing}
                        icon={<ThunderboltOutlined />}
                        style={{ marginTop: 12 }}
                      >
                        نصب و راه‌اندازی خودکار تلگرام شاپ
                      </Button>
                    </Form>
                  </Card>
                </Col>

                <Col xs={24} lg={9}>
                  <Card className="tgshop-card" title={<span style={{ color: '#fff' }}>ویژگی‌های سیستم تلگرام شاپ</span>}>
                    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                      <div>
                        <Text strong style={{ color: '#06b6d4' }}>⚡️ بدون تداخل با پنل و وب‌سرور:</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          برخلاف نصب‌های سنتی، هیچ پورت مشترکی اشغال نمی‌شود و روی دامنه‌های فعال یا Nginx تاثیر منفی ندارد.
                        </Paragraph>
                      </div>

                      <div>
                        <Text strong style={{ color: '#10b981' }}>🎁 اکانت تست رایگان ۲۴ ساعته:</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          جلوگیری از دریافت مکرر تست با ثبت آیدی تلگرام در دیتابیس اختصاصی.
                        </Paragraph>
                      </div>

                      <div>
                        <Text strong style={{ color: '#8b5cf6' }}>📊 استعلام لحظه‌ای ترافیک:</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          مشتری با زدن دکمه استعلام، باقیمانده حجم و تاریخ انقضا را مستقیماً از دیتابیس پنل مشاهده می‌کند.
                        </Paragraph>
                      </div>

                      <div>
                        <Text strong style={{ color: '#ec4899' }}>🚀 حذف و نصب مستقیم درون پنل:</Text>
                        <Paragraph style={{ color: '#94a3b8', fontSize: 13, margin: 0 }}>
                          تنظیمات و راه‌اندازی ربات بدون نیاز به دستور ترمینال و تنها از داخل همین صفحه انجام می‌شود.
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
                        title={<span style={{ color: '#94a3b8' }}>ربات تلگرام متصل</span>}
                        value={status.bot_username ? `@${status.bot_username}` : 'متصل'}
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
                          باز کردن ربات در تلگرام ↗
                        </Button>
                      )}
                    </Card>
                  </Col>

                  <Col xs={12} md={4}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>تست‌های رایگان</span>}
                        value={status.total_trials}
                        prefix={<CheckCircleOutlined style={{ color: '#10b981' }} />}
                        valueStyle={{ color: '#10b981' }}
                      />
                    </Card>
                  </Col>

                  <Col xs={12} md={4}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>سفارشات ثبت‌شده</span>}
                        value={status.total_orders}
                        prefix={<ShoppingCartOutlined style={{ color: '#ec4899' }} />}
                        valueStyle={{ color: '#ec4899' }}
                      />
                    </Card>
                  </Col>

                  <Col xs={24} md={8}>
                    <Card className="tgshop-card">
                      <Statistic
                        title={<span style={{ color: '#94a3b8' }}>آیدی ادمین متصل</span>}
                        value={status.admin_chat_id || 'نامشخص'}
                        prefix={<SafetyCertificateOutlined style={{ color: '#8b5cf6' }} />}
                        valueStyle={{ color: '#818cf8', fontSize: 18 }}
                      />
                    </Card>
                  </Col>
                </Row>

                {/* ACTION & CONTROLS */}
                <Card
                  className="tgshop-card"
                  title={<span style={{ color: '#fff' }}>عملیات و مدیریت سرویک تلگرام شاپ</span>}
                  style={{ marginBottom: 24 }}
                >
                  <Space wrap size="middle">
                    <Button
                      icon={<ReloadOutlined />}
                      onClick={() => handleAction('restart')}
                      loading={actionLoading}
                    >
                      راه‌اندازی مجدد سرویس (Restart)
                    </Button>

                    {status.running ? (
                      <Button
                        danger
                        icon={<PauseCircleOutlined />}
                        onClick={() => handleAction('stop')}
                        loading={actionLoading}
                      >
                        توقف موقت ربات (Stop)
                      </Button>
                    ) : (
                      <Button
                        type="primary"
                        icon={<PlayCircleOutlined />}
                        onClick={() => handleAction('start')}
                        loading={actionLoading}
                        style={{ background: '#10b981', borderColor: '#10b981' }}
                      >
                        شروع و فعال‌سازی (Start)
                      </Button>
                    )}

                    <Popconfirm
                      title="حذف کامل تلگرام شاپ"
                      description="آیا از لغو نصب و حذف کامل سرویس ربات تلگرام شاپ اطمینان دارید؟"
                      onConfirm={() => handleAction('uninstall')}
                      okText="بله، حذف کن"
                      cancelText="انصراف"
                      okButtonProps={{ danger: true }}
                    >
                      <Button danger icon={<DeleteOutlined />} loading={actionLoading}>
                        لغو نصب و حذف کامل (Uninstall)
                      </Button>
                    </Popconfirm>
                  </Space>
                </Card>

                {/* CONFIG DETAILS */}
                <Card
                  className="tgshop-card"
                  title={<span style={{ color: '#fff' }}>مشخصات و پیکربندی فعال</span>}
                >
                  <Form form={form} layout="vertical" onFinish={handleInstall}>
                    <Row gutter={16}>
                      <Col xs={24} md={12}>
                        <Form.Item
                          name="bot_token"
                          label={<span style={{ color: '#e2e8f0' }}>توکن ربات تلگرام</span>}
                          rules={[{ required: true }]}
                        >
                          <Input prefix={<SendOutlined />} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12}>
                        <Form.Item
                          name="admin_chat_id"
                          label={<span style={{ color: '#e2e8f0' }}>چت آیدی ادمین</span>}
                          rules={[{ required: true }]}
                        >
                          <Input prefix={<UserOutlined />} />
                        </Form.Item>
                      </Col>
                    </Row>

                    <Button type="primary" htmlType="submit" loading={installing} className="tgshop-btn-install">
                      ذخیره و اعمال تغییرات
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