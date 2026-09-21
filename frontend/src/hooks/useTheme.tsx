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

// NOVA X PANEL — "Nova Aurora". A deep night-sky palette with a blue → violet
// → pink accent, chosen from rendered candidates on a live panel. Effects stay
// soft (see nova-glass.css) so it is pretty without being tiring.
const AURORA = {
  dark: {
    bg: '#0b0e1a',
    container: '#141926',
    elevated: '#1b2132',
    sider: '#0f1422',
    border: 'rgba(255, 255, 255, 0.11)',
    borderSecondary: 'rgba(255, 255, 255, 0.07)',
    text: '#eef1fb',
    textSecondary: '#c8d0e6',
    textTertiary: '#9aa4c0',
    c1: '#6ea8ff',
    c2: '#a78bfa',
    c3: '#f0a6d0',
    onAccent: '#0a0e1c',
  },
  ultra: {
    bg: '#06080f',
    container: '#0e1220',
    elevated: '#141a2c',
    sider: '#0a0e1a',
    border: 'rgba(255, 255, 255, 0.09)',
    borderSecondary: 'rgba(255, 255, 255, 0.06)',
    text: '#e8ecf8',
    textSecondary: '#c2cbe3',
    textTertiary: '#939db8',
    c1: '#6ea8ff',
    c2: '#a78bfa',
    c3: '#f0a6d0',
    onAccent: '#080b16',
  },
  light: {
    bg: '#f5f6fb',
    container: '#ffffff',
    elevated: '#ffffff',
    sider: '#ffffff',
    border: 'rgba(60, 60, 90, 0.16)',
    borderSecondary: 'rgba(60, 60, 90, 0.1)',
    text: '#1e2333',
    textSecondary: '#4c5570',
    textTertiary: '#626c88',
    c1: '#3f6fd8',
    c2: '#6d4fd0',
    c3: '#c2568f',
    onAccent: '#ffffff',
  },
};

const DARK_TOKENS = {
  colorBgBase: AURORA.dark.bg,
  colorBgLayout: AURORA.dark.bg,
  colorBgContainer: AURORA.dark.container,
  colorBgElevated: AURORA.dark.elevated,
  colorBgSpotlight: AURORA.dark.elevated,
  colorPrimary: AURORA.dark.c1,
  colorLink: AURORA.dark.c1,
  colorText: AURORA.dark.text,
  colorTextBase: AURORA.dark.text,
  colorTextSecondary: AURORA.dark.textSecondary,
  colorTextTertiary: AURORA.dark.textTertiary,
  colorTextPlaceholder: AURORA.dark.textTertiary,
  colorBorder: AURORA.dark.border,
  colorBorderSecondary: AURORA.dark.borderSecondary,
  borderRadius: 10,
  boxShadow: 'none',
  boxShadowSecondary: 'none',
};
const ULTRA_DARK_TOKENS = {
  ...DARK_TOKENS,
  colorBgBase: AURORA.ultra.bg,
  colorBgLayout: AURORA.ultra.bg,
  colorBgContainer: AURORA.ultra.container,
  colorBgElevated: AURORA.ultra.elevated,
  colorBgSpotlight: AURORA.ultra.elevated,
  colorText: AURORA.ultra.text,
  colorTextSecondary: AURORA.ultra.textSecondary,
  colorTextTertiary: AURORA.ultra.textTertiary,
  colorTextPlaceholder: AURORA.ultra.textTertiary,
  colorBorder: AURORA.ultra.border,
  colorBorderSecondary: AURORA.ultra.borderSecondary,
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: AURORA.dark.bg,
  headerBg: AURORA.dark.sider,
  headerColor: AURORA.dark.text,
  footerBg: AURORA.dark.bg,
  siderBg: AURORA.dark.sider,
  triggerBg: AURORA.dark.container,
  triggerColor: AURORA.dark.text,
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: AURORA.ultra.bg,
  headerBg: AURORA.ultra.sider,
  headerColor: AURORA.ultra.text,
  footerBg: AURORA.ultra.bg,
  siderBg: AURORA.ultra.sider,
  triggerBg: AURORA.ultra.container,
  triggerColor: AURORA.ultra.text,
};
const DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: AURORA.dark.elevated,
  darkItemColor: AURORA.dark.textSecondary,
  darkItemSelectedBg: AURORA.dark.c1,
  darkItemSelectedColor: AURORA.dark.onAccent,
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: AURORA.ultra.elevated,
  darkItemColor: AURORA.ultra.textSecondary,
  darkItemSelectedBg: AURORA.ultra.c1,
  darkItemSelectedColor: AURORA.ultra.onAccent,
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: AURORA.dark.borderSecondary,
  borderRadiusLG: 18,
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: AURORA.ultra.borderSecondary,
  borderRadiusLG: 18,
};
const DARK_TABLE_TOKENS = {
  headerBg: AURORA.dark.elevated,
  headerColor: AURORA.dark.textSecondary,
  borderColor: AURORA.dark.borderSecondary,
  rowHoverBg: 'rgba(110, 168, 255, 0.12)',
  headerSplitColor: 'transparent',
};
const DARK_BUTTON_TOKENS = {
  primaryShadow: 'none',
  defaultShadow: 'none',
  defaultBg: 'rgba(110, 168, 255, 0.10)',
  defaultBorderColor: AURORA.dark.border,
};
const DARK_FIELD_TOKENS = {
  activeBorderColor: AURORA.dark.c1,
  hoverBorderColor: AURORA.dark.c2,
  activeShadow: '0 0 0 4px rgba(110, 168, 255, 0.2)',
};
const DARK_MODAL_TOKENS = {
  contentBg: AURORA.dark.container,
  headerBg: 'transparent',
  titleColor: AURORA.dark.text,
  boxShadow: 'none',
};
const ULTRA_DARK_MODAL_TOKENS = { ...DARK_MODAL_TOKENS, contentBg: AURORA.ultra.container };
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: AURORA.light.textSecondary,
  colorTextTertiary: AURORA.light.textTertiary,
  colorTextPlaceholder: AURORA.light.textTertiary,
  colorBgBase: AURORA.light.bg,
  colorBgLayout: AURORA.light.bg,
  colorBgContainer: AURORA.light.container,
  colorBgElevated: AURORA.light.elevated,
  colorText: AURORA.light.text,
  colorTextBase: AURORA.light.text,
  colorTextSecondary: AURORA.light.textSecondary,
  colorBorder: AURORA.light.border,
  colorBorderSecondary: AURORA.light.borderSecondary,
  colorError: '#b4483a',
  colorSuccessText: '#3f6b4a',
  colorPrimary: AURORA.light.c1,
  colorLink: AURORA.light.c1,
  borderRadius: 10,
  boxShadow: 'none',
  boxShadowSecondary: 'none',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: AURORA.light.c1,
  colorPrimaryHover: AURORA.light.c2,
  colorPrimaryActive: AURORA.light.c2,
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
          bodyBg: AURORA.light.bg,
          siderBg: AURORA.light.sider,
          headerBg: AURORA.light.sider,
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
