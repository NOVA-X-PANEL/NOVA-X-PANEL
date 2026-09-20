// @ts-nocheck — vendored from Heimdall-Panel (upstream pg-ui); keep byte-compatible with upstream.
import { clsx } from 'clsx';
import type { ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
