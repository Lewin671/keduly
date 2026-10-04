// Escape closes the topmost thing that is open: a popover before the card under it, a card
// before the sheet it sits in.
import { useEffect, useRef } from 'preact/hooks';

const stack: Array<{ close: () => void }> = [];

if (typeof document !== 'undefined') {
  document.addEventListener('keydown', event => {
    if (event.key !== 'Escape' || !stack.length) return;
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
