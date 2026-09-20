/**
 * NOVA X brand mark — an angular, futuristic "N" cut into a bevelled hexagonal
 * crest. Rendered as inline SVG so it inherits the page's sizing and stays crisp
 * at any scale, and so the gradient ids can be namespaced per instance.
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
      aria-label="NOVA X"
      focusable="false"
    >
      <defs>
        <linearGradient id={gradOuter} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#38bdf8" />
          <stop offset="55%" stopColor="#2563eb" />
          <stop offset="100%" stopColor="#7c3aed" />
        </linearGradient>
        <linearGradient id={gradN} x1="0" y1="1" x2="1" y2="0">
          <stop offset="0%" stopColor="#e0f2fe" />
          <stop offset="100%" stopColor="#ffffff" />
        </linearGradient>
        <filter id={glow} x="-40%" y="-40%" width="180%" height="180%">
          <feGaussianBlur stdDeviation="2.4" result="blur" />
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
        opacity="0.22"
      />
      <path
        d="M32 3.2 L57.5 17.8 V46.2 L32 60.8 L6.5 46.2 V17.8 Z"
        fill="none"
        stroke={`url(#${gradOuter})`}
        strokeWidth="1.8"
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

      {/* accent spark */}
      <circle cx="47.5" cy="20.5" r="2.6" fill="#22d3ee" filter={`url(#${glow})`} />
    </svg>
  );
}
