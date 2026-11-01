/**
 * NOVA X brand mark — Cyrus the Great / Pasargadae Edition
 * Imperial Persian Gold, Ancient Sandstone, and Royal Persian Turquoise.
 * Depicting the majestic stepped Tomb of Cyrus the Great in Pasargadae
 * crowned with Achaemenid wings and solar radiance.
 */

interface NovaLogoProps {
  /** Rendered size in px (square). */
  size?: number;
  /** Unique suffix so multiple marks on one page don't collide on gradient ids. */
  idSuffix?: string;
  className?: string;
}

export default function NovaLogo({ size = 64, idSuffix = 'a', className }: NovaLogoProps) {
  const gradGold = `cyrus-gold-${idSuffix}`;
  const gradTurquoise = `cyrus-tq-${idSuffix}`;
  const gradStone = `cyrus-stone-${idSuffix}`;
  const glow = `cyrus-glow-${idSuffix}`;

  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 64 64"
      role="img"
      aria-label="NOVA X · CYRUS THE GREAT"
      focusable="false"
    >
      <defs>
        {/* Royal Achaemenid Gold Gradient */}
        <linearGradient id={gradGold} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0%" stopColor="#fff0b3" />
          <stop offset="35%" stopColor="#d4af37" />
          <stop offset="70%" stopColor="#aa7c11" />
          <stop offset="100%" stopColor="#f3e5ab" />
        </linearGradient>

        {/* Persian Turquoise Gradient */}
        <linearGradient id={gradTurquoise} x1="0" y1="1" x2="1" y2="0">
          <stop offset="0%" stopColor="#0077b6" />
          <stop offset="50%" stopColor="#00b4d8" />
          <stop offset="100%" stopColor="#90e0ef" />
        </linearGradient>

        {/* Ancient Sandstone Gradient */}
        <linearGradient id={gradStone} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#e8dfcb" />
          <stop offset="60%" stopColor="#c5b796" />
          <stop offset="100%" stopColor="#8c7a56" />
        </linearGradient>

        <filter id={glow} x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="2.2" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* Radiant Solar Aura / Halo */}
      <circle cx="32" cy="24" r="18" fill={`url(#${gradTurquoise})`} opacity="0.12" filter={`url(#${glow})`} />
      <circle cx="32" cy="24" r="14" fill="none" stroke={`url(#${gradGold})`} strokeWidth="0.8" opacity="0.4" strokeDasharray="2 2" />

      {/* Achaemenid Spread Wings (Faravahar Inspired Wing Silhouette behind Tomb) */}
      <g filter={`url(#${glow})`} opacity="0.85">
        {/* Left Wing */}
        <path
          d="M 22 23 C 16 19, 9 20, 3 24 C 9 27, 16 26, 21 28 Z"
          fill={`url(#${gradGold})`}
        />
        <path
          d="M 22 25 C 15 23, 8 26, 5 31 C 11 32, 17 30, 21 32 Z"
          fill={`url(#${gradGold})`}
          opacity="0.8"
        />
        <path
          d="M 22 28 C 17 28, 11 31, 8 36 C 14 36, 18 34, 21 35 Z"
          fill={`url(#${gradGold})`}
          opacity="0.6"
        />

        {/* Right Wing */}
        <path
          d="M 42 23 C 48 19, 55 20, 61 24 C 55 27, 48 26, 43 28 Z"
          fill={`url(#${gradGold})`}
        />
        <path
          d="M 42 25 C 49 23, 56 26, 59 31 C 53 32, 47 30, 43 32 Z"
          fill={`url(#${gradGold})`}
          opacity="0.8"
        />
        <path
          d="M 42 28 C 47 28, 53 31, 56 36 C 50 36, 46 34, 43 35 Z"
          fill={`url(#${gradGold})`}
          opacity="0.6"
        />
      </g>

      {/* Central Solar Disc / Lotus Gem */}
      <circle cx="32" cy="11" r="3.2" fill={`url(#${gradGold})`} filter={`url(#${glow})`} />
      <circle cx="32" cy="11" r="1.6" fill="#00b4d8" />

      {/* The Majestic Tomb of Cyrus the Great (Pasargadae) */}
      <g id="cyrus-tomb">
        {/* Gabled Triangular Stone Roof */}
        <polygon points="32,16 19,25 45,25" fill={`url(#${gradGold})`} filter={`url(#${glow})`} />
        <polygon points="32,17.5 21,24.5 43,24.5" fill={`url(#${gradStone})`} />
        {/* Roof Pediment Line */}
        <line x1="18" y1="25" x2="46" y2="25" stroke={`url(#${gradGold})`} strokeWidth="1.2" />

        {/* Sacred Chamber */}
        <rect x="21" y="25" width="22" height="9" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.8" />
        {/* Chamber Entrance Doorway */}
        <path d="M 29.5 34 V 28.5 Q 32 27.5 34.5 28.5 V 34 Z" fill="#050c18" stroke={`url(#${gradGold})`} strokeWidth="0.7" />

        {/* 6-Tier Stepped Stone Plinth (پایه‌های پلکانی پاسارگاد) */}
        {/* Tier 6 (Top step) */}
        <rect x="19" y="34" width="26" height="3.2" rx="0.5" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.6" />
        {/* Tier 5 */}
        <rect x="17" y="37.2" width="30" height="3.2" rx="0.5" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.6" />
        {/* Tier 4 */}
        <rect x="15" y="40.4" width="34" height="3.4" rx="0.5" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.6" />
        {/* Tier 3 */}
        <rect x="13" y="43.8" width="38" height="3.6" rx="0.5" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.6" />
        {/* Tier 2 */}
        <rect x="10.5" y="47.4" width="43" height="4.2" rx="0.5" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.7" />
        {/* Tier 1 (Massive foundation step) */}
        <rect x="7" y="51.6" width="50" height="5.2" rx="0.8" fill={`url(#${gradStone})`} stroke={`url(#${gradGold})`} strokeWidth="0.8" />
      </g>

      {/* Decorative Persian Turquoise Jewels at the Base */}
      <circle cx="12" cy="54.2" r="1.3" fill="#00b4d8" />
      <circle cx="52" cy="54.2" r="1.3" fill="#00b4d8" />
      <circle cx="32" cy="54.2" r="1.5" fill={`url(#${gradGold})`} />
    </svg>
  );
}
