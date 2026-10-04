// Text that is edited where it stands: a title, a note, a heading.
import type { JSX } from 'preact';
import { useEffect, useLayoutEffect, useRef, useState } from 'preact/hooks';

interface Props {
  value: string;
  onSave: (text: string) => void;
  class?: string;
  placeholder?: string;
  label: string;
  /** Allow line breaks; otherwise Enter saves. */
  multiline?: boolean;
  /** An empty value is saved rather than reverted. */
  allowEmpty?: boolean;
  autoFocus?: boolean;
  /** Called when editing ends, saved or not. */
  onDone?: () => void;
}

export function InlineText({ value, onSave, class: cls, placeholder, label, multiline, allowEmpty, autoFocus, onDone }: Props): JSX.Element {
  const [text, setText] = useState(value);
  const ref = useRef<HTMLTextAreaElement>(null);
  const cancelled = useRef(false);

  useEffect(() => {
    if (document.activeElement !== ref.current) setText(value);
  }, [value]);
  useEffect(() => {
    if (autoFocus) ref.current?.select();
  }, []);
  useLayoutEffect(() => {
    const el = ref.current!;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, [text]);

  const commit = () => {
    const next = multiline ? text.trim() : text.replace(/\s+/g, ' ').trim();
    if (cancelled.current || (!next && !allowEmpty)) setText(value);
    else {
      setText(next);
      if (next !== value) onSave(next);
    }
    cancelled.current = false;
    onDone?.();
  };

  return (
    <textarea
      ref={ref} class={cls ? `ed ${cls}` : 'ed'} rows={1} value={text} placeholder={placeholder} aria-label={label}
      onInput={event => setText(event.currentTarget.value)}
      onBlur={commit}
      onKeyDown={event => {
        if (event.isComposing) return;
        if (event.key === 'Enter' && !(multiline && !event.metaKey && !event.ctrlKey)) {
          event.preventDefault();
          event.currentTarget.blur();
        } else if (event.key === 'Escape') {
          event.stopPropagation();
          cancelled.current = true;
          event.currentTarget.blur();
        }
      }}
    />
  );
}
