import { describe, expect, it } from 'vitest';
import { layoutLanes } from './lanes';

const block = (s: number, e: number) => ({ s, e });

describe('lane layout', () => {
  it('gives a lone block the whole column', () => {
    const a = block(9, 10);
    expect(layoutLanes([a]).get(a)).toEqual({ lane: 0, of: 1 });
  });

  it('keeps back-to-back blocks in one lane', () => {
    const a = block(9, 10);
    const b = block(10, 11);
    const at = layoutLanes([a, b]);
    expect(at.get(a)).toEqual({ lane: 0, of: 1 });
    expect(at.get(b)).toEqual({ lane: 0, of: 1 });
  });

  it('puts two overlapping blocks side by side', () => {
    const a = block(10, 11.5);
    const b = block(10.5, 11.5);
    const at = layoutLanes([b, a]);
    expect(at.get(a)).toEqual({ lane: 0, of: 2 });
    expect(at.get(b)).toEqual({ lane: 1, of: 2 });
  });

  it('uses as many lanes as the busiest moment needs and shares the width across the cluster', () => {
    // The crowded morning of the mockup: four blocks between 10:00 and 11:30.
    const review = block(10, 11.5);
    const interview = block(10, 11);
    const sync = block(10.25, 11.25);
    const call = block(10.5, 11);
    const at = layoutLanes([review, interview, sync, call]);
    expect([review, interview, sync, call].map(x => at.get(x)!.lane)).toEqual([0, 1, 2, 3]);
    expect([review, interview, sync, call].every(x => at.get(x)!.of === 4)).toBe(true);
  });

  it('reuses a lane once the block in it has ended', () => {
    const long = block(9, 12);
    const first = block(9, 10);
    const second = block(10, 11);
    const at = layoutLanes([long, first, second]);
    expect(at.get(long)).toEqual({ lane: 0, of: 2 });
    expect(at.get(first)).toEqual({ lane: 1, of: 2 });
    expect(at.get(second)).toEqual({ lane: 1, of: 2 });
  });

  it('starts a new cluster after a gap, so later blocks are full width again', () => {
    const a = block(9, 10);
    const b = block(9.5, 10.5);
    const later = block(14, 15);
    const at = layoutLanes([a, b, later]);
    expect(at.get(a)!.of).toBe(2);
    expect(at.get(later)).toEqual({ lane: 0, of: 1 });
  });

  it('treats a very short block as tall as it is drawn', () => {
    // 15 minutes are drawn 0.4 hours tall, so a block starting 15 minutes later still collides.
    const pill = block(13, 13.25);
    const next = block(13.25, 13.5);
    const at = layoutLanes([pill, next]);
    expect(at.get(pill)).toEqual({ lane: 0, of: 2 });
    expect(at.get(next)).toEqual({ lane: 1, of: 2 });
  });

  it('places the longer of two blocks with the same start first', () => {
    const short = block(10, 10.5);
    const long = block(10, 12);
    const at = layoutLanes([short, long]);
    expect(at.get(long)!.lane).toBe(0);
    expect(at.get(short)!.lane).toBe(1);
  });

  it('handles an empty list', () => {
    expect(layoutLanes([]).size).toBe(0);
  });
});
