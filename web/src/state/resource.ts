// Fetching for a screen: keeps what it has while it refetches, and refetches when the shared
// version changes.
import { useEffect, useRef, useState } from 'preact/hooks';
import { reportError, version } from './store';

let inflight = 0;
const settled = new Set<() => void>();

/** Runs `fn` each time the last pending screen request finishes. */
export const onSettled = (fn: () => void) => { settled.add(fn); };

/** Counts a request as pending, so optimistic state can wait for fresh data to arrive. */
export function track<T>(work: Promise<T>): Promise<T> {
  inflight++;
  return work.finally(() => {
    if (--inflight === 0) for (const fn of settled) fn();
  });
}

export interface Resource<T> {
  /** `undefined` until the first load for this key finishes. */
  data: T | undefined;
  /** The first load failed and there is nothing to show. */
  failed: boolean;
  reload: () => void;
  /** Replaces the data locally, e.g. after loading one more page. */
  set: (data: T) => void;
}

export function useResource<T>(key: string, fetcher: () => Promise<T>): Resource<T> {
  const [state, setState] = useState<{ key: string; data?: T; failed: boolean }>({ key, failed: false });
  const [nonce, setNonce] = useState(0);
  const latest = useRef(fetcher);
  latest.current = fetcher;
  const v = version.value;

  useEffect(() => {
    let live = true;
    track(latest.current()).then(
      data => { if (live) setState({ key, data, failed: false }); },
      err => {
        if (!live) return;
        reportError(err);
        setState(old => (old.key === key && old.data !== undefined ? old : { key, failed: true }));
      },
    );
    return () => { live = false; };
  }, [key, v, nonce]);

  const current = state.key === key ? state : { key, data: undefined, failed: false };
  return {
    data: current.data,
    failed: current.failed,
    reload: () => setNonce(n => n + 1),
    set: data => setState({ key, data, failed: false }),
  };
}
