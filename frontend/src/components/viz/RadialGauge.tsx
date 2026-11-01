import { useId } from 'react';
import type { ReactNode } from 'react';

import './RadialGauge.css';

export interface RadialGaugeProps {
  /** Progress value, clamped to 0–100. */
  value: number;
  /** Rendered size in px (square). */
  size?: number;
  /** Ring stroke width in px. */
  thickness?: number;
  /** Gradient start colour. */
  from?: string;
  /** Gradient end colour. */
  to?: string;
  /** Unfilled track colour; defaults to a translucent grey. */
  trackColor?: string;
  /** Adds a soft bloom on the filled arc. */
  glow?: boolean;
  /** Accessible name for the gauge. */
  ariaLabel?: string;
  /** Centred overlay content (the readout). */
  children?: ReactNode;
}

export default function RadialGauge({
  value,
  size = 148,
  thickness = 11,
  from = '#8b5cf6',
  to = '#4f7cff',
  trackColor = 'rgba(128, 128, 140, 0.22)',
  glow = true,
  ariaLabel,
  children,
}: RadialGaugeProps) {
  const rawId = useId();
  const gradientId = `rg-${rawId.replace(/[^a-zA-Z0-9]/g, '')}`;

  const center = size / 2;
  const radius = (size - thickness) / 2;
  const circumference = 2 * Math.PI * radius;
  const clamped = Math.max(0, Math.min(100, Number.isFinite(value) ? value : 0));
  const filled = (clamped / 100) * circumference;

  return (
    <div className="rg-wrap" style={{ width: size, height: size }}>
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        role="img"
        aria-label={ariaLabel}
        focusable="false"
        className="rg-svg"
      >
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor={from} />
            <stop offset="100%" stopColor={to} />
          </linearGradient>
        </defs>

        <circle
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke={trackColor}
          strokeWidth={thickness}
        />

        <circle
          className={glow ? 'rg-arc rg-arc-glow' : 'rg-arc'}
          cx={center}
          cy={center}
          r={radius}
          fill="none"
          stroke={`url(#${gradientId})`}
          strokeWidth={thickness}
          strokeLinecap="round"
          strokeDasharray={`${filled} ${Math.max(0, circumference - filled)}`}
          transform={`rotate(-90 ${center} ${center})`}
        />
      </svg>

      {children ? <div className="rg-center">{children}</div> : null}
    </div>
  );
}
