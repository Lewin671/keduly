// A scanned QR code opens the app on `#approve=<id>` (this device is asked to let another one in)
// or `#signin=<code>` (this device is offered a session). The link is read once and taken out of
// the address bar at once, so a sign-in code is not left in the history or shared with the page URL.
import { signal } from '@preact/signals';
import { formatRoute, route } from './route';

export type Link = { kind: 'approve'; id: string } | { kind: 'signin'; code: string };

export function parseLink(hash: string): Link | null {
  const match = /^#(approve|signin)=([A-Za-z0-9_-]{1,200})$/.exec(hash);
  if (!match) return null;
  return match[1] === 'approve' ? { kind: 'approve', id: match[2]! } : { kind: 'signin', code: match[2]! };
}

export const link = signal<Link | null>(null);

function read(): void {
  const found = parseLink(location.hash);
  if (!found) return;
  link.value = found;
  history.replaceState(null, '', formatRoute(route.value));
}

/** Call before the route takes over the hash. */
export function initLink(): void {
  read();
  addEventListener('hashchange', read);
}

/** What a QR code holds for each kind of link. */
export const approveUrl = (id: string) => `${location.origin}/#approve=${id}`;
export const signinUrl = (code: string) => `${location.origin}/#signin=${code}`;
