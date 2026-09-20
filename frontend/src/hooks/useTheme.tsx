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

// NOVA X PANEL — "Nova Calm · Umber". A warm, low-saturation palette: flat
// warm-dark surfaces, hairline borders and one terracotta accent. Chosen over
// the earlier violet/blue scheme because that one was tiring to look at.
const UMBER = {
  dark: {
    bg: '#1f1c1a',
    container: '#282423',
    elevated: '#302b29',
    sider: '#231f1d',
    border: 'rgba(255, 255, 255, 0.10)',
    borderSecondary: 'rgba(255, 255, 255, 0.08)',
    text: '#ece6de',
    textSecondary: '#cec3b6',
    textTertiary: '#a2948a',
    accent: '#c08a63',
    accentHover: '#a9764f',
    onAccent: '#1a1512',
  },
  ultra: {
    bg: '#12100f',
    container: '#1a1716',
    elevated: '#211d1b',
    sider: '#171413',
    border: 'rgba(255, 255, 255, 0.09)',
    borderSecondary: 'rgba(255, 255, 255, 0.07)',
    text: '#e8e2da',
    textSecondary: '#c8bdb0',
    textTertiary: '#9c8f85',
    accent: '#c08a63',
    accentHover: '#a9764f',
    onAccent: '#141110',
  },
  light: {
    bg: '#f4f1ee',
    container: '#ffffff',
    elevated: '#ffffff',
    sider: '#ffffff',
    border: 'rgba(60, 45, 35, 0.14)',
    borderSecondary: 'rgba(60, 45, 35, 0.10)',
    text: '#2b2622',
    textSecondary: '#5a5148',
    textTertiary: '#7d7268',
    accent: '#a9764f',
    accentHover: '#8f5f3c',
    onAccent: '#ffffff',
  },
};

const DARK_TOKENS = {
  colorBgBase: UMBER.dark.bg,
  colorBgLayout: UMBER.dark.bg,
  colorBgContainer: UMBER.dark.container,
  colorBgElevated: UMBER.dark.elevated,
  colorBgSpotlight: UMBER.dark.elevated,
  colorPrimary: UMBER.dark.accent,
  colorLink: UMBER.dark.accent,
  colorText: UMBER.dark.text,
  colorTextBase: UMBER.dark.text,
  colorTextSecondary: UMBER.dark.textSecondary,
  colorTextTertiary: UMBER.dark.textTertiary,
  colorTextPlaceholder: UMBER.dark.textTertiary,
  colorBorder: UMBER.dark.border,
  colorBorderSecondary: UMBER.dark.borderSecondary,
  borderRadius: 10,
  boxShadow: 'none',
  boxShadowSecondary: 'none',
};
const ULTRA_DARK_TOKENS = {
  ...DARK_TOKENS,
  colorBgBase: UMBER.ultra.bg,
  colorBgLayout: UMBER.ultra.bg,
  colorBgContainer: UMBER.ultra.container,
  colorBgElevated: UMBER.ultra.elevated,
  colorBgSpotlight: UMBER.ultra.elevated,
  colorText: UMBER.ultra.text,
  colorTextSecondary: UMBER.ultra.textSecondary,
  colorTextTertiary: UMBER.ultra.textTertiary,
  colorTextPlaceholder: UMBER.ultra.textTertiary,
  colorBorder: UMBER.ultra.border,
  colorBorderSecondary: UMBER.ultra.borderSecondary,
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: UMBER.dark.bg,
  headerBg: UMBER.dark.sider,
  headerColor: UMBER.dark.text,
  footerBg: UMBER.dark.bg,
  siderBg: UMBER.dark.sider,
  triggerBg: UMBER.dark.container,
  triggerColor: UMBER.dark.text,
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: UMBER.ultra.bg,
  headerBg: UMBER.ultra.sider,
  headerColor: UMBER.ultra.text,
  footerBg: UMBER.ultra.bg,
  siderBg: UMBER.ultra.sider,
  triggerBg: UMBER.ultra.container,
  triggerColor: UMBER.ultra.text,
};
const DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: UMBER.dark.elevated,
  darkItemColor: UMBER.dark.textSecondary,
  darkItemSelectedBg: UMBER.dark.accent,
  darkItemSelectedColor: UMBER.dark.onAccent,
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: UMBER.ultra.elevated,
  darkItemColor: UMBER.ultra.textSecondary,
  darkItemSelectedBg: UMBER.ultra.accent,
  darkItemSelectedColor: UMBER.ultra.onAccent,
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: UMBER.dark.borderSecondary,
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: UMBER.ultra.borderSecondary,
};
const DARK_TABLE_TOKENS = {
  headerBg: UMBER.dark.elevated,
  headerColor: UMBER.dark.textSecondary,
  borderColor: UMBER.dark.borderSecondary,
  rowHoverBg: 'rgba(192, 138, 99, 0.10)',
  headerSplitColor: 'transparent',
};
const DARK_BUTTON_TOKENS = {
  primaryShadow: 'none',
  defaultShadow: 'none',
  defaultBg: UMBER.dark.elevated,
  defaultBorderColor: UMBER.dark.border,
};
const DARK_FIELD_TOKENS = {
  activeBorderColor: UMBER.dark.accent,
  hoverBorderColor: UMBER.dark.accentHover,
  activeShadow: 'none',
};
const DARK_MODAL_TOKENS = {
  contentBg: UMBER.dark.container,
  headerBg: 'transparent',
  titleColor: UMBER.dark.text,
  boxShadow: 'none',
};
const ULTRA_DARK_MODAL_TOKENS = { ...DARK_MODAL_TOKENS, contentBg: UMBER.ultra.container };
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: UMBER.light.textSecondary,
  colorTextTertiary: UMBER.light.textTertiary,
  colorTextPlaceholder: UMBER.light.textTertiary,
  colorBgBase: UMBER.light.bg,
  colorBgLayout: UMBER.light.bg,
  colorBgContainer: UMBER.light.container,
  colorBgElevated: UMBER.light.elevated,
  colorText: UMBER.light.text,
  colorTextBase: UMBER.light.text,
  colorTextSecondary: UMBER.light.textSecondary,
  colorBorder: UMBER.light.border,
  colorBorderSecondary: UMBER.light.borderSecondary,
  colorError: '#b4483a',
  colorSuccessText: '#3f6b4a',
  colorPrimary: UMBER.light.accent,
  colorLink: UMBER.light.accent,
  borderRadius: 10,
  boxShadow: 'none',
  boxShadowSecondary: 'none',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: UMBER.light.accent,
  colorPrimaryHover: UMBER.light.accentHover,
  colorPrimaryActive: UMBER.light.accentHover,
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
          bodyBg: UMBER.light.bg,
          siderBg: UMBER.light.sider,
          headerBg: UMBER.light.sider,
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
