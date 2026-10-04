// A long list shows its first rows; the rest arrives in batches when asked for.
import { useEffect, useState } from 'preact/hooks';
import * as api from '../../api/client';
import type { Item, ItemFilters } from '../../api/types';
import { track } from '../../state/resource';
import { reportError, version } from '../../state/store';

interface Options {
  /** Rows shown before anything is asked for. */
  first: number;
  /** Rows added per request. */
  step: number;
  /** The first rows and the total, when another request already brought them. */
  initial?: { items: Item[]; total: number };
  /** Set to false to fetch nothing yet. */
  enabled?: boolean;
}

export interface Paged {
  items: Item[] | undefined;
  total: number;
  rest: number;
  /** A further batch is on its way. */
  loading: boolean;
  failed: boolean;
  more: () => void;
  reload: () => void;
}

export function usePaged(filters: ItemFilters, { first, step, initial, enabled = true }: Options): Paged {
  const id = JSON.stringify(filters);
  const [want, setWant] = useState(first);
  const [state, setState] = useState<{ id: string; items: Item[]; total: number } | undefined>(undefined);
  const [failed, setFailed] = useState(false);
  const [nonce, setNonce] = useState(0);
  const v = version.value;
  const covered = initial !== undefined && (want <= initial.items.length || initial.items.length >= initial.total);
  const fetching = enabled && !covered;

  // Refetching asks for as many rows as are shown, so the list does not shrink.
  useEffect(() => {
    if (!fetching) return;
    let live = true;
    track(api.listItemsUpTo(filters, want)).then(
      page => {
        if (!live) return;
        setState({ id, ...page });
        setFailed(false);
      },
      err => {
        if (!live) return;
        reportError(err);
        setFailed(true);
      },
    );
    return () => { live = false; };
  }, [id, want, fetching, v, nonce]);

  const loaded = state?.id === id ? state : undefined;
  // While a bigger batch loads, keep showing the rows already there.
  const data = covered ? initial : (loaded ?? initial);
  const items = data?.items.slice(0, want);
  const total = data?.total ?? 0;
  return {
    items,
    total,
    rest: items ? Math.max(total - items.length, 0) : 0,
    loading: fetching && !failed && (!loaded || loaded.items.length < Math.min(want, loaded.total)),
    failed: failed && !data,
    more: () => setWant(want + step),
    reload: () => setNonce(n => n + 1),
  };
}
