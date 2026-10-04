import { describe, expect, it } from 'vitest';
import { groupByMonth } from './done';

const item = (id: string, completed_at: string | null) => ({ id, completed_at });

describe('grouping finished items by month', () => {
  it('makes one group per month, keeping the order given', () => {
    const groups = groupByMonth([
      item('a', '2026-10-12T15:00:00Z'),
      item('b', '2026-10-02T09:00:00Z'),
      item('c', '2026-09-28T09:00:00Z'),
      item('d', '2025-12-31T17:00:00Z'),
    ]);
    expect(groups.map(g => [g.month, g.items.map(i => i.id)])).toEqual([
      ['2026-10', ['a', 'b']],
      ['2026-09', ['c']],
      ['2025-12', ['d']],
    ]);
  });

  it('uses the local month of completion', () => {
    // 02:00 UTC on October 1st is still September 30th in New York.
    const groups = groupByMonth([item('a', '2026-10-01T05:00:00Z'), item('b', '2026-10-01T02:00:00Z')]);
    expect(groups.map(g => g.month)).toEqual(['2026-10', '2026-09']);
  });

  it('keeps an item without a timestamp with its neighbours', () => {
    const groups = groupByMonth([item('a', '2026-10-12T15:00:00Z'), item('b', null)]);
    expect(groups).toHaveLength(1);
    expect(groups[0]!.items).toHaveLength(2);
  });

  it('returns nothing for an empty log', () => {
    expect(groupByMonth([])).toEqual([]);
  });
});
