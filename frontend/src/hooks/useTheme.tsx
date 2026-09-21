import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function applyDom(isDark: boolean, isUltra: boolean) {
  document.body.classList.remove('dark', 'light');
  document.body.classList.add(isDark ? 'dark' : 'light');
  // Native scrollbars read color-scheme, not the body class.
  document.documentElement.style.colorScheme = isDark ? 'dark' : 'light';
  if (isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  const msg = document.getElementById('message');
  if (msg) {
    msg.classList.remove('dark', 'light');
    msg.classList.add(isDark ? 'dark' : 'light');
  }
}

// module load so the document is in the right theme before React mounts.
const initialDark = readBool(STORAGE_DARK, true);
const initialUltra = readBool(STORAGE_ULTRA, false);
applyDom(initialDark, initialUltra);

// NOVA X PANEL — "Neon Console" (from the NEON X reference). A deep navy canvas
// with neon-glow cards, luminous blue borders and electric accents: blue
// #2563FF, cyan #00BFFF, teal #13D6B0, violet #7140FF, magenta #F13B96.
const NEON = {
  dark: {
    bg: '#030b1d',
    container: '#071c3d',
    elevated: '#081d3d',
    sider: '#040c20',
    border: 'rgba(51,112,215,.35)',
    borderSecondary: 'rgba(70,109,170,.22)',
    text: '#f3f7ff',
    textSecondary: '#a6bbdc',
    textTertiary: '#93a9cf',
    blue: '#2563ff',
    violet: '#7140ff',
    onAccent: '#ffffff',
  },
  ultra: {
    bg: '#02081a',
    container: '#04102a',
    elevated: '#061633',
    sider: '#030a18',
    border: 'rgba(51,112,215,.28)',
    borderSecondary: 'rgba(70,109,170,.18)',
    text: '#eaf1ff',
    textSecondary: '#a2b7da',
    textTertiary: '#8fa5ca',
    blue: '#2563ff',
    violet: '#7140ff',
    onAccent: '#ffffff',
  },
  light: {
    bg: '#f4f7ff',
    container: '#ffffff',
    elevated: '#ffffff',
    sider: '#ffffff',
    border: 'rgba(37,99,255,.24)',
    borderSecondary: 'rgba(60,100,180,.16)',
    text: '#0f1830',
    textSecondary: '#48587a',
    textTertiary: '#5d6d92',
    blue: '#1d4ed8',
    violet: '#6d28d9',
    onAccent: '#ffffff',
  },
};

const DARK_TOKENS = {
  colorBgBase: NEON.dark.bg,
  colorBgLayout: NEON.dark.bg,
  colorBgContainer: NEON.dark.container,
  colorBgElevated: NEON.dark.elevated,
  colorBgSpotlight: NEON.dark.elevated,
  colorPrimary: NEON.dark.blue,
  colorLink: NEON.dark.blue,
  colorText: NEON.dark.text,
  colorTextBase: NEON.dark.text,
  colorTextSecondary: NEON.dark.textSecondary,
  colorTextTertiary: NEON.dark.textTertiary,
  colorTextPlaceholder: NEON.dark.textTertiary,
  colorBorder: NEON.dark.border,
  colorBorderSecondary: NEON.dark.borderSecondary,
  borderRadius: 10,
  boxShadow: 'none',
  boxShadowSecondary: 'none',
};
const ULTRA_DARK_TOKENS = {
  ...DARK_TOKENS,
  colorBgBase: NEON.ultra.bg,
  colorBgLayout: NEON.ultra.bg,
  colorBgContainer: NEON.ultra.container,
  colorBgElevated: NEON.ultra.elevated,
  colorBgSpotlight: NEON.ultra.elevated,
  colorText: NEON.ultra.text,
  colorTextSecondary: NEON.ultra.textSecondary,
  colorTextTertiary: NEON.ultra.textTertiary,
  colorTextPlaceholder: NEON.ultra.textTertiary,
  colorBorder: NEON.ultra.border,
  colorBorderSecondary: NEON.ultra.borderSecondary,
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: NEON.dark.bg,
  headerBg: NEON.dark.sider,
  headerColor: NEON.dark.text,
  footerBg: NEON.dark.bg,
  siderBg: NEON.dark.sider,
  triggerBg: NEON.dark.container,
  triggerColor: NEON.dark.text,
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: NEON.ultra.bg,
  headerBg: NEON.ultra.sider,
  headerColor: NEON.ultra.text,
  footerBg: NEON.ultra.bg,
  siderBg: NEON.ultra.sider,
  triggerBg: NEON.ultra.container,
  triggerColor: NEON.ultra.text,
};
const DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: NEON.dark.elevated,
  darkItemColor: NEON.dark.textSecondary,
  darkItemSelectedBg: NEON.dark.blue,
  darkItemSelectedColor: NEON.dark.onAccent,
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: NEON.ultra.elevated,
  darkItemColor: NEON.ultra.textSecondary,
  darkItemSelectedBg: NEON.ultra.blue,
  darkItemSelectedColor: NEON.ultra.onAccent,
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: NEON.dark.borderSecondary,
  borderRadiusLG: 12,
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: NEON.ultra.borderSecondary,
  borderRadiusLG: 12,
};
const DARK_TABLE_TOKENS = {
  headerBg: NEON.dark.elevated,
  headerColor: NEON.dark.textSecondary,
  borderColor: NEON.dark.borderSecondary,
  rowHoverBg: 'rgba(37,99,255,.16)',
  headerSplitColor: 'transparent',
};
const DARK_BUTTON_TOKENS = {
  primaryShadow: 'none',
  defaultShadow: 'none',
  defaultBg: 'rgba(37,99,255,.10)',
  defaultBorderColor: NEON.dark.border,
};
const DARK_FIELD_TOKENS = {
  activeBorderColor: NEON.dark.blue,
  hoverBorderColor: NEON.dark.violet,
  activeShadow: '0 0 0 3px rgba(37,99,255,.22)',
};
const DARK_MODAL_TOKENS = {
  contentBg: NEON.dark.elevated,
  headerBg: 'transparent',
  titleColor: NEON.dark.text,
  boxShadow: 'none',
};
const ULTRA_DARK_MODAL_TOKENS = { ...DARK_MODAL_TOKENS, contentBg: NEON.ultra.elevated };
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: NEON.light.textSecondary,
  colorTextTertiary: NEON.light.textTertiary,
  colorTextPlaceholder: NEON.light.textTertiary,
  colorBgBase: NEON.light.bg,
  colorBgLayout: NEON.light.bg,
  colorBgContainer: NEON.light.container,
  colorBgElevated: NEON.light.elevated,
  colorText: NEON.light.text,
  colorTextBase: NEON.light.text,
  colorTextSecondary: NEON.light.textSecondary,
  colorBorder: NEON.light.border,
  colorBorderSecondary: NEON.light.borderSecondary,
  colorError: '#b4483a',
  colorSuccessText: '#3f6b4a',
  colorPrimary: NEON.light.blue,
  colorLink: NEON.light.blue,
  borderRadius: 10,
  boxShadow: 'none',
  boxShadowSecondary: 'none',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: NEON.light.blue,
  colorPrimaryHover: NEON.light.violet,
  colorPrimaryActive: NEON.light.violet,
  primaryShadow: 'none',
  defaultShadow: 'none',
};

// hashed:false drops the `:where(.css-<hash>)` wrapper antd puts around every
// rule. It costs nothing in specificity — `:where()` contributes zero, so the
// panel's own `.ant-*` overrides still win — and it removes roughly 5,700
// wrappers, 16% of the generated stylesheet, from what the browser has to parse.
//
// cssVar.key pins the CSS-variable scope. Every panel page mounts its own
// ConfigProvider (there is no root one), and without a fixed key each mints a
// fresh useId-derived scope, so navigating re-serialises and re-injects the whole
// token block under a new class instead of reusing the one already in the head.
const SHARED_STYLE_CONFIG = {
  hashed: false,
  cssVar: { key: 'xui' },
} as const;

export function buildAntdThemeConfig(isDark: boolean, isUltra: boolean): ThemeConfig {
  if (!isDark) {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: LIGHT_CONTRAST_TOKENS,
      components: {
        Layout: {
          bodyBg: NEON.light.bg,
          siderBg: NEON.light.sider,
          headerBg: NEON.light.sider,
        },
        Statistic: STATISTIC_TOKENS,
        Button: LIGHT_BUTTON_TOKENS,
      },
    };
  }
  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: antdTheme.darkAlgorithm,
    token: isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Modal: isUltra ? ULTRA_DARK_MODAL_TOKENS : DARK_MODAL_TOKENS,
      Table: DARK_TABLE_TOKENS,
      Button: DARK_BUTTON_TOKENS,
      Input: DARK_FIELD_TOKENS,
      Select: DARK_FIELD_TOKENS,
      InputNumber: DARK_FIELD_TOKENS,
      DatePicker: DARK_FIELD_TOKENS,
      Statistic: STATISTIC_TOKENS,
    },
  };
}

export function pauseAnimationsUntilLeave(elementId: string): void {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

interface ThemeContextValue {
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [isDark, setIsDark] = useState<boolean>(initialDark);
  const [isUltra, setIsUltra] = useState<boolean>(initialUltra);

  useLayoutEffect(() => {
    applyDom(isDark, isUltra);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [isDark, isUltra]);

  const toggleTheme = useCallback(() => setIsDark((v) => !v), []);
  const toggleUltra = useCallback(() => setIsUltra((v) => !v), []);

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(isDark, isUltra), [isDark, isUltra]);

  const value = useMemo<ThemeContextValue>(
    () => ({ isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig }),
    [isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
