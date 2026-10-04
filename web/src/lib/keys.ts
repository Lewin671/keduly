// Keyboard rules shared by everything that opens and closes. Escape closes the topmost thing
// that is open: a popover before the card under it.
import { useEffect, useRef } from 'preact/hooks';

const stack: Array<{ close: () => void }> = [];

if (typeof document !== 'undefined') {
  document.addEventListener('keydown', event => {
    if (event.key !== 'Escape' || !stack.length || isComposing(event)) return;
    event.preventDefault();
    stack[stack.length - 1]!.close();
  });
}

export function useEscape(close: () => void): void {
  const entry = useRef({ close });
  entry.current.close = close;
  useEffect(() => {
    const mine = entry.current;
    stack.push(mine);
    return () => { stack.splice(stack.indexOf(mine), 1); };
  }, []);
}

/**
 * Whether a key press belongs to an input method that is still composing text, e.g. Enter
 * confirming a Chinese candidate. Safari reports that Enter after the composition has ended,
 * recognisable only by its legacy key code.
 */
export function isComposing(event: KeyboardEvent): boolean {
  return event.isComposing || event.keyCode === 229;
}

/** For a form's keydown: Enter that only confirms composed text must not submit the form. */
export function keepComposing(event: KeyboardEvent): void {
  if (event.key === 'Enter' && isComposing(event)) event.preventDefault();
}
