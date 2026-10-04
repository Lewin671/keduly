import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, deleteItem, getCounts, listItems, listItemsUpTo, request, updateItem } from './client';
import type { Item } from './types';

type FetchArgs = [url: string, init: RequestInit];

function mockFetch(...responses: Array<Response | Error>) {
  const fn = vi.fn<(...args: FetchArgs) => Promise<Response>>();
  for (const r of responses) {
    if (r instanceof Error) fn.mockRejectedValueOnce(r);
    else fn.mockResolvedValueOnce(r);
  }
  vi.stubGlobal('fetch', fn);
  return fn;
}

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const caught = async (work: Promise<unknown>): Promise<ApiError> => {
  try {
    await work;
  } catch (err) {
    if (err instanceof ApiError) return err;
    throw err;
  }
  throw new Error('expected the request to fail');
};

afterEach(() => vi.unstubAllGlobals());

describe('requests', () => {
  it('reads with a relative URL, same-origin credentials and no CSRF header', async () => {
    const fetch = mockFetch(json({ counts: { inbox: 2, today: 6, pending: 4 }, revision: 9 }));
    expect(await getCounts()).toEqual({ counts: { inbox: 2, today: 6, pending: 4 }, revision: 9 });
    const [url, init] = fetch.mock.calls[0]!;
    expect(url).toBe('/api/v1/counts');
    expect(init.method).toBe('GET');
    expect(init.credentials).toBe('same-origin');
    expect(init.headers).not.toHaveProperty('X-Keduly-Request');
    expect(init.body).toBeUndefined();
  });

  it('marks every write with the CSRF header and sends JSON', async () => {
    const fetch = mockFetch(json({ item: { id: 'abc' } }));
    expect(await updateItem('abc', { important: true })).toEqual({ id: 'abc' });
    const [url, init] = fetch.mock.calls[0]!;
    expect(url).toBe('/api/v1/items/abc');
    expect(init.method).toBe('PATCH');
    expect(init.headers).toMatchObject({ 'X-Keduly-Request': '1', 'Content-Type': 'application/json' });
    expect(init.body).toBe('{"important":true}');
  });

  it('sends the header on writes without a body, and accepts 204', async () => {
    const fetch = mockFetch(new Response(null, { status: 204 }));
    expect(await deleteItem('abc')).toBeUndefined();
    const [, init] = fetch.mock.calls[0]!;
    expect(init.method).toBe('DELETE');
    expect(init.headers).toMatchObject({ 'X-Keduly-Request': '1' });
    expect(init.headers).not.toHaveProperty('Content-Type');
  });

  it('builds the query, leaving out unset filters and escaping values', async () => {
    const fetch = mockFetch(json({ items: [], next_cursor: null, total: 0 }));
    await listItems({ project_id: 'none', q: 'a b&c', cursor: null, limit: 20 });
    expect(fetch.mock.calls[0]![0]).toBe('/api/v1/items?project_id=none&q=a+b%26c&limit=20');
  });

  it('escapes ids in paths', async () => {
    const fetch = mockFetch(new Response(null, { status: 204 }));
    await deleteItem('a/b');
    expect(fetch.mock.calls[0]![0]).toBe('/api/v1/items/a%2Fb');
  });
});

describe('errors', () => {
  it('carries the status, code and message of an API error', async () => {
    mockFetch(json({ error: { code: 'not_found', message: 'item not found' } }, 404));
    const err = await caught(getCounts());
    expect([err.status, err.code, err.message]).toEqual([404, 'not_found', 'item not found']);
  });

  it('reports a lost session as 401', async () => {
    mockFetch(json({ error: { code: 'unauthenticated', message: 'sign in' } }, 401));
    expect((await caught(getCounts())).status).toBe(401);
  });

  it('copes with an error that is not JSON', async () => {
    mockFetch(new Response('<html>Bad Gateway</html>', { status: 502, statusText: 'Bad Gateway' }));
    const err = await caught(getCounts());
    expect([err.status, err.code]).toEqual([502, 'http']);
  });

  it('copes with JSON that is not the documented error shape', async () => {
    mockFetch(json({ message: 'nope' }, 500));
    expect((await caught(getCounts())).code).toBe('http');
  });

  it('turns a network failure into an ApiError', async () => {
    mockFetch(new TypeError('Failed to fetch'));
    const err = await caught(getCounts());
    expect([err.status, err.code]).toEqual([0, 'network']);
  });

  it('rejects a successful response that is not JSON', async () => {
    mockFetch(new Response('<html></html>', { status: 200 }));
    expect((await caught(request('GET', '/counts'))).code).toBe('bad_response');
  });

  it('treats an empty successful body as no value', async () => {
    mockFetch(new Response('', { status: 200 }));
    expect(await request('POST', '/auth/logout')).toBeUndefined();
  });
});

describe('listing more than one page', () => {
  const items = (from: number, n: number) => Array.from({ length: n }, (_, k) => ({ id: String(from + k) }) as Item);

  it('follows cursors until it has enough', async () => {
    const fetch = mockFetch(
      json({ items: items(0, 200), next_cursor: 'c1', total: 450 }),
      json({ items: items(200, 50), next_cursor: 'c2', total: 450 }),
    );
    const page = await listItemsUpTo({ status: 'done' }, 250);
    expect(page.items).toHaveLength(250);
    expect(page.total).toBe(450);
    expect(page.next_cursor).toBe('c2');
    expect(fetch.mock.calls.map(c => c[0])).toEqual(['/api/v1/items?status=done&limit=200', '/api/v1/items?status=done&limit=50&cursor=c1']);
  });

  it('stops at the last page', async () => {
    const fetch = mockFetch(json({ items: items(0, 3), next_cursor: null, total: 3 }));
    const page = await listItemsUpTo({ project_id: 'p' }, 20);
    expect(page.items).toHaveLength(3);
    expect(page.next_cursor).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
