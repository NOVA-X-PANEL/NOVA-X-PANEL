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

// NOVA X PANEL accent. A violet primary gives every page — buttons, tabs,
// switches, selected rows, focus rings — the same brand colour as the Nova
// Dark surfaces without touching component-level styles.
const NOVA_PRIMARY_LIGHT = '#7c3aed';
const NOVA_PRIMARY_DARK = '#8b5cf6';
const NOVA_CANVAS = '#06060b';
const NOVA_CANVAS_ULTRA = '#000000';
const NOVA_CONTAINER = '#0e0e16';
const NOVA_CONTAINER_ULTRA = '#0a0a0f';
const NOVA_ELEVATED = '#14141c';
const NOVA_ELEVATED_ULTRA = '#101016';
const NOVA_SIDER = '#0b0b12';
const NOVA_SIDER_ULTRA = '#050508';
const NOVA_HAIRLINE = 'rgba(255, 255, 255, 0.08)';
const NOVA_HAIRLINE_ULTRA = 'rgba(255, 255, 255, 0.07)';

const DARK_TOKENS = {
  colorBgBase: NOVA_CANVAS,
  colorBgLayout: NOVA_CANVAS,
  colorBgContainer: NOVA_CONTAINER,
  colorBgElevated: NOVA_ELEVATED,
  colorBgSpotlight: NOVA_ELEVATED,
  colorPrimary: NOVA_PRIMARY_DARK,
  colorLink: NOVA_PRIMARY_DARK,
  colorText: '#eef1f8',
  colorTextSecondary: '#a6b1c7',
  colorTextTertiary: '#6d7890',
  colorBorder: NOVA_HAIRLINE,
  colorBorderSecondary: NOVA_HAIRLINE,
  controlOutline: 'rgba(139, 92, 246, 0.22)',
  borderRadius: 10,
  borderRadiusLG: 16,
  borderRadiusSM: 8,
};
const ULTRA_DARK_TOKENS = {
  colorBgBase: NOVA_CANVAS_ULTRA,
  colorBgLayout: NOVA_CANVAS_ULTRA,
  colorBgContainer: NOVA_CONTAINER_ULTRA,
  colorBgElevated: NOVA_ELEVATED_ULTRA,
  colorBgSpotlight: NOVA_ELEVATED_ULTRA,
  colorPrimary: NOVA_PRIMARY_DARK,
  colorLink: NOVA_PRIMARY_DARK,
  colorText: '#eef1f8',
  colorTextSecondary: '#a6b1c7',
  colorTextTertiary: '#6d7890',
  colorBorder: NOVA_HAIRLINE_ULTRA,
  colorBorderSecondary: NOVA_HAIRLINE_ULTRA,
  controlOutline: 'rgba(139, 92, 246, 0.22)',
  borderRadius: 10,
  borderRadiusLG: 16,
  borderRadiusSM: 8,
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: NOVA_CANVAS,
  headerBg: NOVA_SIDER,
  headerColor: '#ffffff',
  footerBg: NOVA_CANVAS,
  siderBg: NOVA_SIDER,
  triggerBg: NOVA_CONTAINER,
  triggerColor: '#ffffff',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: NOVA_CANVAS_ULTRA,
  headerBg: NOVA_SIDER_ULTRA,
  headerColor: '#ffffff',
  footerBg: NOVA_CANVAS_ULTRA,
  siderBg: NOVA_SIDER_ULTRA,
  triggerBg: NOVA_CONTAINER_ULTRA,
  triggerColor: '#ffffff',
};
const DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: NOVA_ELEVATED,
  darkItemSelectedBg: 'transparent',
  darkItemHoverBg: 'transparent',
  darkItemColor: '#a6b1c7',
  darkItemSelectedColor: '#ffffff',
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'transparent',
  darkPopupBg: NOVA_ELEVATED_ULTRA,
  darkItemSelectedBg: 'transparent',
  darkItemHoverBg: 'transparent',
  darkItemColor: '#a6b1c7',
  darkItemSelectedColor: '#ffffff',
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: NOVA_HAIRLINE,
  borderRadiusLG: 16,
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: NOVA_HAIRLINE_ULTRA,
  borderRadiusLG: 16,
};
const DARK_MODAL_TOKENS = {
  contentBg: NOVA_CONTAINER,
  headerBg: 'transparent',
  titleColor: '#eef1f8',
};
const ULTRA_DARK_MODAL_TOKENS = {
  contentBg: NOVA_CONTAINER_ULTRA,
  headerBg: 'transparent',
  titleColor: '#eef1f8',
};
const DARK_TABLE_TOKENS = {
  headerBg: 'rgba(255, 255, 255, 0.025)',
  headerColor: '#6d7890',
  rowHoverBg: 'rgba(139, 92, 246, 0.12)',
  borderColor: NOVA_HAIRLINE,
  headerSplitColor: 'transparent',
};
const DARK_BUTTON_TOKENS = {
  primaryShadow: 'none',
  defaultBg: 'rgba(255, 255, 255, 0.05)',
  defaultBorderColor: NOVA_HAIRLINE,
};
const DARK_INPUT_TOKENS = {
  activeBorderColor: NOVA_PRIMARY_DARK,
  hoverBorderColor: 'rgba(139, 92, 246, 0.45)',
  activeShadow: '0 0 0 3px rgba(139, 92, 246, 0.22)',
};
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: 'rgba(0, 0, 0, 0.58)',
  colorTextTertiary: 'rgba(0, 0, 0, 0.58)',
  colorTextPlaceholder: '#767676',
  colorError: '#cf1322',
  colorErrorText: '#cf1322',
  colorSuccessText: '#237804',
  colorPrimary: NOVA_PRIMARY_LIGHT,
  colorLink: NOVA_PRIMARY_LIGHT,
  borderRadius: 10,
  borderRadiusLG: 16,
  borderRadiusSM: 8,
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: NOVA_PRIMARY_LIGHT,
  colorPrimaryHover: '#8b5cf6',
  colorPrimaryActive: '#6d28d9',
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
      Input: DARK_INPUT_TOKENS,
      Select: DARK_INPUT_TOKENS,
      InputNumber: DARK_INPUT_TOKENS,
      DatePicker: DARK_INPUT_TOKENS,
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
