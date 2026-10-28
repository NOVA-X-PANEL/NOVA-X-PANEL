/**
 * NOVA X brand mark — Halloween Edition
 * Fiery Pumpkin Orange, Harvest Gold, and Haunted Spectral Violet.
 */

interface NovaLogoProps {
  /** Rendered size in px (square). */
  size?: number;
  /** Unique suffix so multiple marks on one page don't collide on gradient ids. */
  idSuffix?: string;
  className?: string;
}

export default function NovaLogo({ size = 64, idSuffix = 'a', className }: NovaLogoProps) {
  const gradOuter = `nova-outer-${idSuffix}`;
  const gradN = `nova-n-${idSuffix}`;
  const glow = `nova-glow-${idSuffix}`;

  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 64 64"
      role="img"
      aria-label="NOVA X HALLOWEEN"
      focusable="false"
    >
      <defs>
        <linearGradient id={gradOuter} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#ff7700" />
          <stop offset="45%" stopColor="#ff5400" />
          <stop offset="100%" stopColor="#9d4edd" />
        </linearGradient>
        <linearGradient id={gradN} x1="0" y1="1" x2="1" y2="0">
          <stop offset="0%" stopColor="#ffe49e" />
          <stop offset="60%" stopColor="#ff9e00" />
          <stop offset="100%" stopColor="#ffffff" />
        </linearGradient>
        <filter id={glow} x="-40%" y="-40%" width="180%" height="180%">
          <feGaussianBlur stdDeviation="2.8" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* bevelled crest */}
      <path
        d="M32 3.2 L57.5 17.8 V46.2 L32 60.8 L6.5 46.2 V17.8 Z"
        fill={`url(#${gradOuter})`}
        opacity="0.25"
      />
      <path
        d="M32 3.2 L57.5 17.8 V46.2 L32 60.8 L6.5 46.2 V17.8 Z"
        fill="none"
        stroke={`url(#${gradOuter})`}
        strokeWidth="2.0"
        strokeLinejoin="round"
      />

      {/* angular N */}
      <path
        d="M21.5 45.5 V18.5 L42.5 45.5 V18.5"
        fill="none"
        stroke={`url(#${gradN})`}
        strokeWidth="6.4"
        strokeLinecap="square"
        strokeLinejoin="miter"
        filter={`url(#${glow})`}
      />

      {/* pumpkin fire accent spark */}
      <circle cx="47.5" cy="20.5" r="3.0" fill="#ff7700" filter={`url(#${glow})`} />
      <circle cx="16.5" cy="43.5" r="2.2" fill="#9d4edd" filter={`url(#${glow})`} />
    </svg>
  );
}
