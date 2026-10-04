// The mockup's inline SVG symbols, plus a few drawn in the same style for actions it does not show.
import type { JSX } from 'preact';

export type IconName =
  | 'cal' | 'week' | 'tray' | 'list' | 'checklist' | 'bell' | 'gear' | 'search' | 'plus' | 'pluscircle' | 'left' | 'right'
  | 'term' | 'phone' | 'star' | 'layers' | 'book' | 'flag' | 'clock' | 'moon' | 'grid' | 'person' | 'more' | 'trash' | 'copy';

export function Sprite(): JSX.Element {
  return (
    <svg width="0" height="0" style="position:absolute" aria-hidden="true">
      <symbol id="i-cal" viewBox="0 0 24 24"><rect x="3.5" y="5" width="17" height="15.5" rx="3.5" /><path d="M3.5 10h17M8 3v4M16 3v4" /></symbol>
      <symbol id="i-week" viewBox="0 0 24 24"><rect x="3.5" y="5" width="17" height="15.5" rx="3.5" /><path d="M3.5 10h17M9.2 10v10.5M14.8 10v10.5" /></symbol>
      <symbol id="i-tray" viewBox="0 0 24 24"><path d="M4 13.5l2.4-7.2A2 2 0 018.3 5h7.4a2 2 0 011.9 1.3l2.4 7.2v4a2 2 0 01-2 2H6a2 2 0 01-2-2zM4 13.5h4.5a3.5 3.5 0 007 0H20" /></symbol>
      <symbol id="i-list" viewBox="0 0 24 24"><path d="M9 7h11M9 12h11M9 17h11" /><circle cx="4.8" cy="7" r="1" fill="currentColor" /><circle cx="4.8" cy="12" r="1" fill="currentColor" /><circle cx="4.8" cy="17" r="1" fill="currentColor" /></symbol>
      <symbol id="i-checklist" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5" /><path d="M8.3 12.3l2.6 2.6 4.8-5.2" /></symbol>
      <symbol id="i-bell" viewBox="0 0 24 24"><path d="M6 16.5V11a6 6 0 0112 0v5.5l1.5 2h-15zM10 20.5a2.2 2.2 0 004 0" /></symbol>
      <symbol id="i-gear" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" /><path d="M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M18.4 5.6l-1.8 1.8M7.4 16.6l-1.8 1.8" /></symbol>
      <symbol id="i-search" viewBox="0 0 24 24"><circle cx="10.5" cy="10.5" r="6.5" /><path d="M15.5 15.5L20 20" /></symbol>
      <symbol id="i-plus" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14" /></symbol>
      <symbol id="i-pluscircle" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9.5" fill="currentColor" stroke="none" /><path d="M12 7.5v9M7.5 12h9" stroke="#fff" stroke-width="2" /></symbol>
      <symbol id="i-left" viewBox="0 0 24 24"><path d="M14.5 5.5L8 12l6.5 6.5" /></symbol>
      <symbol id="i-right" viewBox="0 0 24 24"><path d="M9.5 5.5L16 12l-6.5 6.5" /></symbol>
      <symbol id="i-term" viewBox="0 0 24 24"><path d="M5 8l4.5 4L5 16M12.5 16.5H19" /></symbol>
      <symbol id="i-phone" viewBox="0 0 24 24"><rect x="7" y="2.5" width="10" height="19" rx="3" /><path d="M11 5.5h2" /></symbol>
      <symbol id="i-star" viewBox="0 0 24 24"><path d="M12 3.2l2.7 5.5 6 .9-4.4 4.2 1.1 6L12 17l-5.4 2.8 1.1-6-4.4-4.2 6-.9z" fill="currentColor" stroke="none" /></symbol>
      <symbol id="i-layers" viewBox="0 0 24 24"><path d="M12 4l8.5 4.5L12 13 3.5 8.5zM3.5 13L12 17.5 20.5 13" /></symbol>
      <symbol id="i-book" viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="4.5" /><path d="M8.3 12.3l2.6 2.6 4.8-5.2" /></symbol>
      <symbol id="i-flag" viewBox="0 0 24 24"><path d="M6 21V4.5M6 5h11.5l-2.4 3.6L17.5 12H6" /></symbol>
      <symbol id="i-clock" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5V12l3 2" /></symbol>
      <symbol id="i-moon" viewBox="0 0 24 24"><path d="M20 14.5A8.5 8.5 0 019.5 4a8.5 8.5 0 1010.5 10.5z" fill="currentColor" stroke="none" /></symbol>
      <symbol id="i-grid" viewBox="0 0 24 24"><rect x="4" y="4" width="7" height="7" rx="2" /><rect x="13" y="4" width="7" height="7" rx="2" /><rect x="4" y="13" width="7" height="7" rx="2" /><rect x="13" y="13" width="7" height="7" rx="2" /></symbol>
      <symbol id="i-person" viewBox="0 0 24 24"><circle cx="12" cy="8.5" r="3.8" /><path d="M4.5 20a7.5 7.5 0 0115 0" /></symbol>
      <symbol id="i-more" viewBox="0 0 24 24"><circle cx="5.5" cy="12" r="1.4" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none" /><circle cx="18.5" cy="12" r="1.4" fill="currentColor" stroke="none" /></symbol>
      <symbol id="i-trash" viewBox="0 0 24 24"><path d="M4.5 7h15M9.5 7V4.5h5V7M6.5 7l.9 11.2a2 2 0 002 1.8h5.2a2 2 0 002-1.8L17.5 7" /></symbol>
      <symbol id="i-copy" viewBox="0 0 24 24"><rect x="8.5" y="8.5" width="11" height="11.5" rx="2.5" /><path d="M15.5 5.5V5a2 2 0 00-2-2H6.5a2 2 0 00-2 2v8.5a2 2 0 002 2H6" /></symbol>
    </svg>
  );
}

export function Icon({ name, class: cls }: { name: IconName; class?: string }): JSX.Element {
  return <svg class={cls ? `i ${cls}` : 'i'} aria-hidden="true"><use href={`#i-${name}`} /></svg>;
}

/** Progress ring of a project: the filled wedge is the share of finished items. */
export function Pie({ done, open, color }: { done: number; open: number; color: string }): JSX.Element {
  const total = done + open;
  const frac = total ? done / total : 0;
  return (
    <svg class="pie" viewBox="0 0 16 16" style={{ color }} aria-hidden="true">
      <circle cx="8" cy="8" r="6.6" fill="none" stroke="currentColor" stroke-width="1.5" />
      <circle cx="8" cy="8" r="2.4" fill="none" stroke="currentColor" stroke-width="4.8" stroke-dasharray={`${frac * 15.08} 15.08`} transform="rotate(-90 8 8)" />
    </svg>
  );
}
