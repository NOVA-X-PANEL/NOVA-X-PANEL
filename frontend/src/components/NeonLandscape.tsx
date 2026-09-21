import landscape from '@/assets/neon-landscape.svg';

/**
 * The reference's neon mountain landscape. Rendered as a fixed, full-viewport
 * layer behind every page so the whole panel shares one background instead of
 * each page painting its own flat colour.
 *
 * `variant`:
 *  - `page`  — faint, sits behind the whole viewport
 *  - `panel` — stronger, for the inside of a card (hero banner, sidebar foot)
 */
export default function NeonLandscape({ variant = 'page' }: { variant?: 'page' | 'panel' }) {
  return (
    <img
      className={`nc-landscape nc-landscape--${variant}`}
      src={landscape}
      alt=""
      aria-hidden="true"
      draggable={false}
    />
  );
}
