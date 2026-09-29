import { useId, useMemo } from 'react';
import type { ReactNode } from 'react';

import './ArcTachometer.css';

export interface ArcTachometerProps {
  /** Progress value, clamped to 0–100. */
  value: number;
  /** Width in px (default 180). */
  width?: number;
  /** Height in px (default 108). */
  height?: number;
  /** Stroke thickness in px (default 11). */
  thickness?: number;
  /** Gradient start colour. */
  from?: string;
  /** Gradient end colour. */
  to?: string;
  /** Unfilled track colour. */
  trackColor?: string;
  /** Adds a soft bloom on the filled arc. */
  glow?: boolean;
  /** Accessible name. */
  ariaLabel?: string;
  /** Secondary subtitle inside the tachometer cradle. */
  subtext?: string;
  /** Centred primary readout (e.g. percentage). */
  children?: ReactNode;
}

export default function ArcTachometer({
  value,
  width = 180,
  height = 108,
  thickness = 11,
  from = '#c084fc',
  to = '#a855f7',
  trackColor = 'rgba(255, 255, 255, 0.08)',
  glow = true,
  ariaLabel,
  subtext,
  children,
}: ArcTachometerProps) {
  const rawId = useId();
  const gradientId = `arc-grad-${rawId.replace(/[^a-zA-Z0-9]/g, '')}`;

  const cx = width / 2;
  // Center Y positioned near the bottom to give a full 180° semi-circle
  const cy = height - 16;
  const radius = cx - thickness - 8;
  const arcLength = Math.PI * radius;

  const clamped = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const filled = (clamped / 100) * arcLength;

  // Arc path from 180° (left) to 0° (right)
  const pathD = `M ${cx - radius} ${cy} A ${radius} ${radius} 0 0 1 ${cx + radius} ${cy}`;

  // Ticks at 0%, 25%, 50%, 75%, 100%
  const ticks = useMemo(() => {
    const r1 = radius + thickness / 2 + 3;
    const r2 = radius + thickness / 2 + 7;
    const angles = [180, 135, 90, 45, 0];
    return angles.map((deg) => {
      const rad = (deg * Math.PI) / 180;
      return {
        x1: cx + r1 * Math.cos(rad),
        y1: cy - r1 * Math.sin(rad),
        x2: cx + r2 * Math.cos(rad),
        y2: cy - r2 * Math.sin(rad),
        major: deg === 90 || deg === 180 || deg === 0,
      };
    });
  }, [cx, cy, radius, thickness]);

  return (
    <div className="arc-tacho-wrap" style={{ width, height }}>
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label={ariaLabel}
        className="arc-tacho-svg"
      >
        <defs>
          <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor={from} />
            <stop offset="100%" stopColor={to} />
          </linearGradient>
        </defs>

        {/* Ticks */}
        <g className="arc-tacho-ticks">
          {ticks.map((t, idx) => (
            <line
              key={idx}
              x1={t.x1}
              y1={t.y1}
              x2={t.x2}
              y2={t.y2}
              stroke="rgba(255, 255, 255, 0.18)"
              strokeWidth={t.major ? 1.5 : 1}
              strokeLinecap="round"
            />
          ))}
        </g>

        {/* Background Track */}
        <path
          d={pathD}
          fill="none"
          stroke={trackColor}
          strokeWidth={thickness}
          strokeLinecap="round"
        />

        {/* Active Arc */}
        {clamped > 0 && (
          <path
            d={pathD}
            fill="none"
            stroke={`url(#${gradientId})`}
            strokeWidth={thickness}
            strokeLinecap="round"
            strokeDasharray={`${filled} ${arcLength}`}
            className={glow ? 'arc-tacho-path arc-tacho-glow' : 'arc-tacho-path'}
            style={
              glow
                ? { filter: `drop-shadow(0 0 8px ${to})` }
                : undefined
            }
          />
        )}
      </svg>

      <div className="arc-tacho-center">
        {children}
        {subtext && <div className="arc-tacho-sub">{subtext}</div>}
      </div>
    </div>
  );
}
