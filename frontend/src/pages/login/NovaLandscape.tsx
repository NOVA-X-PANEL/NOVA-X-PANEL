/**
 * Hero landscape for the login page: a glowing planet sitting on the horizon
 * behind three layers of angular mountains, with soft atmospheric cloud bands.
 *
 * Drawn as one SVG so the depth stack, the planet rim-light and the haze are
 * deterministic and crisp at any width. `preserveAspectRatio` keeps the horizon
 * pinned to the bottom edge as the panel narrows.
 */

interface NovaLandscapeProps {
  idSuffix?: string;
  className?: string;
}

export default function NovaLandscape({ idSuffix = 'a', className }: NovaLandscapeProps) {
  const sky = `nx-sky-${idSuffix}`;
  const planet = `nx-planet-${idSuffix}`;
  const haze = `nx-haze-${idSuffix}`;
  const far = `nx-far-${idSuffix}`;
  const mid = `nx-mid-${idSuffix}`;
  const near = `nx-near-${idSuffix}`;

  return (
    <svg
      className={className}
      viewBox="0 0 1200 460"
      preserveAspectRatio="xMidYMax slice"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient id={sky} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#04070f" />
          <stop offset="55%" stopColor="#071127" />
          <stop offset="100%" stopColor="#0a1c3d" />
        </linearGradient>

        <radialGradient id={planet} cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="#bae6fd" stopOpacity="0.95" />
          <stop offset="35%" stopColor="#38bdf8" stopOpacity="0.55" />
          <stop offset="70%" stopColor="#1d4ed8" stopOpacity="0.22" />
          <stop offset="100%" stopColor="#1d4ed8" stopOpacity="0" />
        </radialGradient>

        <linearGradient id={haze} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#38bdf8" stopOpacity="0" />
          <stop offset="100%" stopColor="#38bdf8" stopOpacity="0.28" />
        </linearGradient>

        <linearGradient id={far} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#2b4b80" />
          <stop offset="100%" stopColor="#16294b" />
        </linearGradient>
        <linearGradient id={mid} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#152a4d" />
          <stop offset="100%" stopColor="#0b1730" />
        </linearGradient>
        <linearGradient id={near} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#0a1226" />
          <stop offset="100%" stopColor="#03060e" />
        </linearGradient>
      </defs>

      {/* sky */}
      <rect x="0" y="0" width="1200" height="460" fill={`url(#${sky})`} />

      {/* planet + rim light on the horizon */}
      <circle cx="600" cy="352" r="230" fill={`url(#${planet})`} />
      <circle
        cx="600"
        cy="352"
        r="118"
        fill="#0b2a5c"
        fillOpacity="0.55"
        stroke="#7dd3fc"
        strokeOpacity="0.45"
        strokeWidth="1.4"
      />

      {/* atmospheric cloud bands */}
      <g fill="#0d1b36" opacity="0.75">
        <ellipse cx="205" cy="150" rx="230" ry="34" />
        <ellipse cx="330" cy="186" rx="180" ry="24" />
        <ellipse cx="975" cy="128" rx="250" ry="30" />
        <ellipse cx="880" cy="176" rx="200" ry="26" />
        <ellipse cx="600" cy="96" rx="300" ry="22" />
      </g>
      <g fill="#16305c" opacity="0.42">
        <ellipse cx="250" cy="205" rx="200" ry="20" />
        <ellipse cx="940" cy="212" rx="220" ry="18" />
      </g>

      {/* horizon haze */}
      <rect x="0" y="300" width="1200" height="70" fill={`url(#${haze})`} />

      {/* far ridge */}
      <path
        d="M0 348 L96 292 L168 326 L262 250 L352 316 L438 272 L520 322 L604 268 L692 320 L780 258 L870 318 L958 276 L1052 330 L1128 292 L1200 340 V460 H0 Z"
        fill={`url(#${far})`}
        opacity="0.9"
      />
      {/* mid ridge */}
      <path
        d="M0 386 L90 340 L186 378 L286 318 L386 372 L486 330 L586 380 L690 326 L790 376 L890 334 L990 382 L1090 342 L1200 388 V460 H0 Z"
        fill={`url(#${mid})`}
      />
      {/* near ridge */}
      <path
        d="M0 428 L110 386 L232 424 L344 366 L462 420 L584 378 L704 424 L826 372 L944 420 L1064 384 L1200 426 V460 H0 Z"
        fill={`url(#${near})`}
      />

      {/* snow-rim highlight on the nearest peaks */}
      <g fill="#7dd3fc" opacity="0.3">
        <path d="M344 366 L354 378 L334 378 Z" />
        <path d="M584 378 L594 390 L574 390 Z" />
        <path d="M826 372 L836 384 L816 384 Z" />
        <path d="M1064 384 L1074 396 L1054 396 Z" />
      </g>
    </svg>
  );
}
