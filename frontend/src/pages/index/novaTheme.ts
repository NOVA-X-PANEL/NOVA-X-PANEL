/**
 * NOVA X PANEL overview palette — the Nova Aurora accent (blue → violet →
 * pink). Kept in one place so the gauges, charts and glow all agree with the
 * panel theme (nova-glass.css / useTheme.tsx).
 */

export const NOVA_BLUE = '#6ea8ff';
export const NOVA_VIOLET = '#a78bfa';
export const NOVA_PINK = '#f0a6d0';

/** The aurora accent as a CSS gradient (used by the hero and accents). */
export const NOVA_GRADIENT = `linear-gradient(135deg, ${NOVA_BLUE} 0%, ${NOVA_VIOLET} 55%, ${NOVA_PINK} 100%)`;

/* Back-compat aliases: the overview charts referenced these names. */
export const NOVA_CYAN = NOVA_PINK;
