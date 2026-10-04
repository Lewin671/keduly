import type { JSX } from 'preact';
import { useMemo } from 'preact/hooks';
import { encode } from 'uqr';

/** A QR code, always dark on white so that a camera reads it in either theme. */
export function Qr({ text, label }: { text: string; label: string }): JSX.Element {
  const { size, path } = useMemo(() => {
    const code = encode(text, { ecc: 'M', border: 4 });
    let d = '';
    code.data.forEach((row, y) => row.forEach((dark, x) => { if (dark) d += `M${x} ${y}h1v1h-1z`; }));
    return { size: code.size, path: d };
  }, [text]);
  return <svg class="qr" viewBox={`0 0 ${size} ${size}`} role="img" aria-label={label} shape-rendering="crispEdges"><path d={path} /></svg>;
}
