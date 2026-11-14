import { writable } from 'svelte/store';

/** Shared shell UI state for the module-ported pages (G40-S3). */
export const accountMenuOpen = writable(false);
