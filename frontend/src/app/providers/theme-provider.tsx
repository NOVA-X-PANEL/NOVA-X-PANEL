// Compatibility shim: the ported pg-ui components (Heimdall's shadcn layer) read
// the theme through `useTheme()` from `@/app/providers/theme-provider`.
//
// NOVA X PANEL already owns a theme context (`@/hooks/useTheme`) that toggles the
// `dark` / `light` class on <body>, which is exactly the class strategy that
// Tailwind's `darkMode: ['class']` expects. Rather than introduce a second,
// competing theme store, this module re-exports that state in the shape pg-ui
// asks for. Colour themes and a custom radius are not offered by this panel, so
// those fields report their defaults.

import { useCallback, useMemo } from 'react';

import { useTheme as usePanelTheme } from '@/hooks/useTheme';

export type Theme = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';
export type ColorTheme = 'default' | (string & {});

export interface ThemeContextValue {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  resolvedTheme: ResolvedTheme;
  colorTheme: ColorTheme;
  setColorTheme: (theme: ColorTheme) => void;
  radius: number;
  setRadius: (radius: number) => void;
}

const DEFAULT_RADIUS = 8;

export function useTheme(): ThemeContextValue {
  const { isDark, toggleTheme } = usePanelTheme();

  const setTheme = useCallback(
    (next: Theme) => {
      const wantDark = next === 'dark';
      if (wantDark !== isDark) toggleTheme();
    },
    [isDark, toggleTheme],
  );

  return useMemo(() => {
    const resolved: ResolvedTheme = isDark ? 'dark' : 'light';
    return {
      theme: resolved,
      setTheme,
      resolvedTheme: resolved,
      colorTheme: 'default' as ColorTheme,
      setColorTheme: () => {},
      radius: DEFAULT_RADIUS,
      setRadius: () => {},
    };
  }, [isDark, setTheme]);
}
