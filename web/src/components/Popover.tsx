// Glass popovers, small menus and sheets.
import type { ComponentChildren, CSSProperties, JSX } from 'preact';
import { useEffect, useLayoutEffect, useRef } from 'preact/hooks';
import { t } from '../i18n';
import { useEscape } from '../lib/escape';

interface PopoverProps {
  onClose: () => void;
  class?: string;
  style?: CSSProperties | string;
  label?: string;
  /**
   * What still counts as inside for click-outside: the popover alone, or also its parent
   * (the wrapper that holds the button which opens it, so that button can toggle).
   */
  within?: 'self' | 'parent';
  children: ComponentChildren;
}

const EDGE = 8;

export function Popover({ onClose, class: cls, style, label, within = 'parent', children }: PopoverProps): JSX.Element {
  const ref = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEscape(() => close.current());

  // Keep the popover on screen: shift it sideways when it would cross the viewport edge.
  useLayoutEffect(() => {
    const el = ref.current!;
    el.style.animation = 'none';
    const r = el.getBoundingClientRect();
    el.style.animation = '';
    let shift = 0;
    if (r.right > innerWidth - EDGE) shift = innerWidth - EDGE - r.right;
    if (r.left + shift < EDGE) shift = EDGE - r.left;
    if (shift) el.style.translate = `${Math.round(shift)}px 0`;
    el.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, []);

  useEffect(() => {
    const down = (event: PointerEvent) => {
      const el = ref.current;
      const inside = within === 'parent' ? el?.parentElement : el;
      if (inside && event.target instanceof Node && !inside.contains(event.target)) close.current();
    };
    document.addEventListener('pointerdown', down, true);
    return () => document.removeEventListener('pointerdown', down, true);
  }, [within]);

  return (
    <div ref={ref} class={cls ? `pop ${cls}` : 'pop'} style={style} role="dialog" aria-label={label}>
      {children}
    </div>
  );
}

interface DialogProps {
  onClose: () => void;
  label: string;
  class?: string;
  children: ComponentChildren;
}

/** A centred sheet over a dimmed page. */
export function Dialog({ onClose, label, class: cls, children }: DialogProps): JSX.Element {
  useEscape(onClose);
  return (
    <div class="scrim" onPointerDown={event => { if (event.target === event.currentTarget) onClose(); }}>
      <div class={cls ? `sheet ${cls}` : 'sheet'} role="dialog" aria-modal="true" aria-label={label}>
        {children}
      </div>
    </div>
  );
}

interface ConfirmProps {
  title: string;
  text?: string;
  action: string;
  onConfirm: () => void;
  onClose: () => void;
}

/** Asks before something that cannot be taken back. */
export function Confirm({ title, text, action, onConfirm, onClose }: ConfirmProps): JSX.Element {
  return (
    <Dialog onClose={onClose} label={title} class="ask">
      <b>{title}</b>
      {text && <p>{text}</p>}
      <div class="pa">
        <button class="pbtn" onClick={onClose}>{t('common.cancel')}</button>
        <button class="pbtn danger" onClick={() => { onClose(); onConfirm(); }}>{action}</button>
      </div>
    </Dialog>
  );
}
