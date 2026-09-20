import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ConfigProvider, Menu, Popover, Spin, message } from 'antd';
import {
  EyeInvisibleOutlined,
  EyeOutlined,
  GlobalOutlined,
  KeyOutlined,
  LockOutlined,
  MoonFilled,
  MoonOutlined,
  RocketOutlined,
  SafetyCertificateOutlined,
  SunOutlined,
  TranslationOutlined,
  UserOutlined,
} from '@ant-design/icons';

import { HttpUtil, LanguageManager } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import { LoginFormSchema, type LoginFormValues } from '@/schemas/login';
import NovaLogo from './NovaLogo';
import NovaLandscape from './NovaLandscape';
import './LoginPage.css';

const basePath = window.X_UI_BASE_PATH || '';
const REMEMBER_KEY = 'nova-login-remember';

/**
 * Fallback copy for the login screen. The panel ships 13 locales, so a key may
 * be missing when a new design lands; these are the values the Persian design
 * specifies and they double as defaults for every other language.
 */
const COPY = {
  slogan: 'مدیریت آسان، اتصال بدون مرز',
  cardSubtitle: 'برای ادامه وارد حساب کاربری خود شوید',
  rememberMe: 'مرا به خاطر بسپار',
  forgotPassword: 'رمز عبور را فراموش کرده‌اید؟',
  or: 'یا',
  telegramLogin: 'ورود با تلگرام',
  telegramUnavailable: 'ورود با تلگرام روی این پنل فعال نشده است',
  credit: 'طراحی و توسعه اختصاصی',
  alwaysOnline: 'همیشه آنلاین',
} as const;

type LoginForm = LoginFormValues;

/** Telegram mark, inline so the button needs no icon-font import. */
function TelegramIcon({ className }: { className?: string }) {
  return (
    <svg className={className} width="17" height="17" viewBox="0 0 24 24" aria-hidden="true">
      <path
        fill="currentColor"
        d="M21.94 4.6 18.9 19.2c-.23 1.02-.84 1.27-1.7.79l-4.7-3.46-2.27 2.18c-.25.25-.46.46-.94.46l.33-4.75 8.65-7.81c.38-.33-.08-.52-.58-.19L6.4 12.9l-4.6-1.44c-1-.31-1.02-1 .21-1.48l17.95-6.92c.83-.3 1.56.2 1.98 1.54Z"
      />
    </svg>
  );
}

const FEATURES = [
  {
    key: 'speed',
    icon: <RocketOutlined />,
    titleKey: 'pages.login.features.speed.title',
    descKey: 'pages.login.features.speed.desc',
    titleDefault: 'High Speed',
    descDefault: 'Fast & Stable',
  },
  {
    key: 'secure',
    icon: <SafetyCertificateOutlined />,
    titleKey: 'pages.login.features.secure.title',
    descKey: 'pages.login.features.secure.desc',
    titleDefault: 'Secure',
    descDefault: 'Your Privacy',
  },
  {
    key: 'global',
    icon: <GlobalOutlined />,
    titleKey: 'pages.login.features.global.title',
    descKey: 'pages.login.features.global.desc',
    titleDefault: 'Global',
    descDefault: 'Worldwide Access',
  },
] as const;

export default function LoginPage() {
  const { t, i18n } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();

  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);

  const [ready, setReady] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [twoFactorEnable, setTwoFactorEnable] = useState(false);
  const [telegramEnabled, setTelegramEnabled] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [remember, setRemember] = useState(() => localStorage.getItem(REMEMBER_KEY) === '1');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [lang, setLang] = useState<string>(() => LanguageManager.getLanguage());

  // The panel is RTL for Persian; the layout uses logical properties so the
  // same markup mirrors correctly for LTR languages.
  const langCode = (i18n.resolvedLanguage || i18n.language || '').toLowerCase();
  const isRtl = langCode.startsWith('fa') || langCode.startsWith('ar') || langCode.startsWith('he');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.post('/getLoginOptions');
      if (cancelled) return;
      const opts = (msg.obj ?? {}) as { twoFactorEnable?: boolean; telegramEnabled?: boolean };
      if (msg.success) {
        setTwoFactorEnable(!!opts.twoFactorEnable);
        setTelegramEnabled(!!opts.telegramEnabled);
      } else {
        // Older panels only expose the 2FA probe.
        const fallback = await HttpUtil.post('/getTwoFactorEnable');
        if (!cancelled && fallback.success) setTwoFactorEnable(!!fallback.obj);
      }
      setReady(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const version = window.X_UI_CUR_VER || '';

  const onSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();

      const candidate = {
        username,
        password,
        twoFactorCode: twoFactorEnable ? totpCode : '',
      };
      const parsed = LoginFormSchema.safeParse(candidate);
      if (!parsed.success) {
        const next: Record<string, string> = {};
        for (const issue of parsed.error.issues) {
          const field = String(issue.path[0] ?? '');
          if (field && !next[field]) next[field] = t(issue.message);
        }
        setErrors(next);
        return;
      }
      setErrors({});

      setSubmitting(true);
      try {
        const msg = await HttpUtil.post('/login', parsed.data);
        if (msg.success) {
          localStorage.setItem(REMEMBER_KEY, remember ? '1' : '0');
          window.location.href = basePath + 'panel/';
        } else {
          setErrors({ form: msg.msg || t('somethingWentWrong') });
        }
      } finally {
        setSubmitting(false);
      }
    },
    [username, password, totpCode, twoFactorEnable, remember, t],
  );

  const onLangChange = useCallback((next: string) => {
    setLang(next);
    LanguageManager.setLanguage(next);
  }, []);

  const cycleTheme = useCallback(() => {
    pauseAnimationsUntilLeave('login-theme-cycle');
    if (!isDark) {
      toggleTheme();
      if (isUltra) toggleUltra();
    } else if (!isUltra) {
      toggleUltra();
    } else {
      toggleUltra();
      toggleTheme();
    }
  }, [isDark, isUltra, toggleTheme, toggleUltra]);

  const langMenuItems = useMemo(
    () =>
      (LanguageManager.supportedLanguages as { value: string; name: string; icon: string }[]).map(
        (l) => ({
          key: l.value,
          label: (
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
              <span aria-hidden="true">{l.icon}</span>
              <span>{l.name}</span>
            </span>
          ),
        }),
      ),
    [],
  );

  const themeIcon = !isDark ? <SunOutlined /> : !isUltra ? <MoonOutlined /> : <MoonFilled />;

  const onTelegram = useCallback(() => {
    messageApi.info(t('pages.login.telegramUnavailable'));
  }, [messageApi, t]);

  return (
    <ConfigProvider theme={antdThemeConfig} direction={isRtl ? 'rtl' : 'ltr'}>
      {messageContextHolder}
      <div className="nova-login" dir={isRtl ? 'rtl' : 'ltr'} lang={langCode || 'en'}>
        {/* ---------------- hero ---------------- */}
        <section className="nx-hero">
          <div className="nx-hero__top">
            <div className="nx-brand">
              <NovaLogo size={52} idSuffix="hero" />
              <div className="nx-brand__text">
                <span className="nx-brand__name">NOVA X</span>
                <span className="nx-brand__label">PANEL</span>
              </div>
            </div>

            <h1 className="nx-hero__title">NOVA X PANEL</h1>
            <p className="nx-hero__tagline">More Freedom · More Connection · Nova X</p>
            <p className="nx-hero__slogan">
              {t('pages.login.slogan', { defaultValue: COPY.slogan })}
            </p>
            <div className="nx-hero__rule" />

            <div className="nx-features">
              {FEATURES.map((feature) => (
                <div className="nx-feature" key={feature.key}>
                  <span className="nx-feature__icon">{feature.icon}</span>
                  <div className="nx-feature__copy">
                    <span className="nx-feature__title">
                      {t(feature.titleKey, { defaultValue: feature.titleDefault })}
                    </span>
                    <span className="nx-feature__desc">
                      {t(feature.descKey, { defaultValue: feature.descDefault })}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="nx-hero__art">
            <NovaLandscape idSuffix="hero" />
          </div>
        </section>

        {/* ---------------- auth ---------------- */}
        <section className="nx-auth">
          <div className="nx-auth__glow" />

          <div className="nx-toolbar">
            <button
              type="button"
              id="login-theme-cycle"
              className="nx-toolbar__btn"
              aria-label={t('menu.theme')}
              title={t('menu.theme')}
              onClick={cycleTheme}
            >
              {themeIcon}
            </button>
            <Popover
              rootClassName={isDark ? 'dark' : 'light'}
              placement="bottomRight"
              trigger="click"
              styles={{ content: { padding: 4 } }}
              content={
                <Menu
                  mode="vertical"
                  selectable
                  selectedKeys={[lang]}
                  items={langMenuItems}
                  onClick={({ key }) => onLangChange(key)}
                  style={{ border: 'none', minWidth: 160 }}
                />
              }
            >
              <button
                type="button"
                className="nx-toolbar__btn"
                aria-label={t('pages.settings.language')}
                title={t('pages.settings.language')}
              >
                <TranslationOutlined />
              </button>
            </Popover>
          </div>

          {!ready ? (
            <div className="nx-loading">
              <Spin size="large" />
            </div>
          ) : (
            <div className="nx-card">
              <div className="nx-card__head">
                <NovaLogo size={58} idSuffix="card" />
                <h2 className="nx-card__title">NOVA X PANEL</h2>
                <p className="nx-card__subtitle">
                  {t('pages.login.cardSubtitle', { defaultValue: COPY.cardSubtitle })}
                </p>
              </div>

              <form className="nx-form" onSubmit={onSubmit} noValidate>
                <label className="nx-field">
                  <span className="nx-field__label">{t('username')}</span>
                  <span className={`nx-input${errors.username ? ' nx-input--error' : ''}`}>
                    <UserOutlined className="nx-input__icon" />
                    <input
                      name="username"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      autoComplete="username"
                      placeholder={t('username')}
                      autoFocus
                      aria-invalid={!!errors.username}
                    />
                  </span>
                  {errors.username && <span className="nx-field__error">{errors.username}</span>}
                </label>

                <label className="nx-field">
                  <span className="nx-field__label">{t('password')}</span>
                  <span className={`nx-input${errors.password ? ' nx-input--error' : ''}`}>
                    <LockOutlined className="nx-input__icon" />
                    <input
                      name="password"
                      type={showPassword ? 'text' : 'password'}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      autoComplete="current-password"
                      placeholder={t('password')}
                      aria-invalid={!!errors.password}
                    />
                    <button
                      type="button"
                      className="nx-input__toggle"
                      onClick={() => setShowPassword((v) => !v)}
                      aria-label={t(showPassword ? 'hidePassword' : 'showPassword', {
                        defaultValue: showPassword ? 'Hide password' : 'Show password',
                      })}
                      title={t(showPassword ? 'hidePassword' : 'showPassword', {
                        defaultValue: showPassword ? 'Hide password' : 'Show password',
                      })}
                    >
                      {showPassword ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                    </button>
                  </span>
                  {errors.password && <span className="nx-field__error">{errors.password}</span>}
                </label>

                {twoFactorEnable && (
                  <label className="nx-field">
                    <span className="nx-field__label">{t('twoFactorCode')}</span>
                    <span className={`nx-input${errors.twoFactorCode ? ' nx-input--error' : ''}`}>
                      <KeyOutlined className="nx-input__icon" />
                      <input
                        name="twoFactorCode"
                        value={totpCode}
                        onChange={(e) => setTotpCode(e.target.value)}
                        autoComplete="one-time-code"
                        inputMode="numeric"
                        placeholder={t('twoFactorCode')}
                        aria-invalid={!!errors.twoFactorCode}
                      />
                    </span>
                    {errors.twoFactorCode && (
                      <span className="nx-field__error">{errors.twoFactorCode}</span>
                    )}
                  </label>
                )}

                {errors.form && <span className="nx-field__error">{errors.form}</span>}

                <button type="submit" className="nx-submit" disabled={submitting}>
                  {submitting && <span className="nx-submit__spinner" aria-hidden="true" />}
                  <span>{t('login')}</span>
                  <span aria-hidden="true">←</span>
                </button>

                <div className="nx-form__meta">
                  <label className="nx-check">
                    <input
                      type="checkbox"
                      checked={remember}
                      onChange={(e) => setRemember(e.target.checked)}
                    />
                    <span>{t('pages.login.rememberMe', { defaultValue: COPY.rememberMe })}</span>
                  </label>
                  <button
                    type="button"
                    className="nx-link"
                    onClick={() => messageApi.info(t('pages.login.forgotHint'))}
                  >
                    {t('pages.login.forgotPassword', { defaultValue: COPY.forgotPassword })}
                  </button>
                </div>
              </form>

              <div className="nx-divider">
                <span>{t('pages.login.or', { defaultValue: COPY.or })}</span>
              </div>

              <button
                type="button"
                className="nx-telegram"
                onClick={onTelegram}
                disabled={!telegramEnabled}
                title={
                  telegramEnabled
                    ? undefined
                    : t('pages.login.telegramUnavailable', {
                        defaultValue: COPY.telegramUnavailable,
                      })
                }
              >
                <TelegramIcon className="nx-telegram__icon" />
                <span>{t('pages.login.telegramLogin', { defaultValue: COPY.telegramLogin })}</span>
              </button>
            </div>
          )}

          <footer className="nx-foot">
            <span className="nx-foot__brand">NOVA X PANEL</span>
            {version && <span className="nx-foot__ver">v{version}</span>}
            <span>{t('pages.login.credit', { defaultValue: COPY.credit })}</span>
            <span className="nx-foot__status">
              <span className="nx-foot__dot" aria-hidden="true" />
              {t('pages.login.alwaysOnline', { defaultValue: COPY.alwaysOnline })}
            </span>
          </footer>
        </section>
      </div>
    </ConfigProvider>
  );
}
