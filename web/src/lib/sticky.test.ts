import { describe, expect, it } from 'vitest';
import { keepInPlace } from './sticky';

const rows = (...ids: string[]) => ids.map(id => ({ id }));
const ids = (list: { id: string }[]) => list.map(x => x.id);
const keepOnly = (...kept: string[]) => (id: string) => (kept.includes(id) ? { id } : undefined);

describe('keeping checked-off items in place', () => {
  it('puts a kept item back where it was', () => {
    expect(ids(keepInPlace(rows('a', 'b', 'c'), rows('a', 'c'), keepOnly('b')))).toEqual(['a', 'b', 'c']);
  });

  it('keeps the first and the last row too', () => {
    expect(ids(keepInPlace(rows('a', 'b', 'c'), rows('b', 'c'), keepOnly('a')))).toEqual(['a', 'b', 'c']);
    expect(ids(keepInPlace(rows('a', 'b', 'c'), rows('a', 'b'), keepOnly('c')))).toEqual(['a', 'b', 'c']);
  });

  it('keeps neighbours that were both checked off in order', () => {
    expect(ids(keepInPlace(rows('a', 'b', 'c', 'd'), rows('a', 'd'), keepOnly('b', 'c')))).toEqual(['a', 'b', 'c', 'd']);
  });

  it('drops rows that are gone for another reason and takes new ones', () => {
    expect(ids(keepInPlace(rows('a', 'b', 'c'), rows('a', 'c', 'n'), keepOnly()))).toEqual(['a', 'c', 'n']);
  });

  it('does not duplicate a row that is still listed', () => {
    expect(ids(keepInPlace(rows('a', 'b'), rows('a', 'b'), keepOnly('b')))).toEqual(['a', 'b']);
  });
});
