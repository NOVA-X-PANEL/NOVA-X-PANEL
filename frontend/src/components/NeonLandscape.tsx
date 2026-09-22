import { createPortal } from 'react-dom';
import landscape from '@/assets/neon-landscape.svg';

/**
 * The reference's neon mountain landscape. Rendered as a fixed layer behind the
 * page so the whole panel shares one background instead of each page painting
 * its own flat colour.
 *
 * `variant`:
 *  - `page`  — a faint wash behind the whole viewport
 *  - `panel` — stronger, for the inside of a card (hero banner, sidebar foot)
 *
 * The `page` variant is portalled to <body> and sits at `z-index: -1`. That
 * matters: rendered in place it lived inside the sidebar, which carries
 * `position: relative; z-index: 210`, so the artwork was painted *over* the entire
 * content column and every panel looked translucent. From <body> at a negative
 * index it paints above the page colour and below all content, which is what an
 * ambience layer is supposed to do.
 */
export default function NeonLandscape({ variant = 'page' }: { variant?: 'page' | 'panel' }) {
  const img = (
    <img
      className={`nc-landscape nc-landscape--${variant}`}
      src={landscape}
      alt=""
      aria-hidden="true"
      draggable={false}
    />
  );

  if (variant === 'page' && typeof document !== 'undefined') {
    return createPortal(img, document.body);
  }

  return img;
}
