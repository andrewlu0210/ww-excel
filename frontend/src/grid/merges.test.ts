import { describe, expect, it } from 'vitest';
import { areaOf, expand, snap, stepStart } from './merges';

const merges = [
  { r0: 0, c0: 0, r1: 0, c1: 2 }, // A1:C1
  { r0: 2, c0: 1, r1: 4, c1: 1 }, // B3:B5
];

describe('merges', () => {
  it('snaps covered cells to the anchor', () => {
    expect(snap(merges, { row: 0, col: 2 })).toEqual({ row: 0, col: 0 });
    expect(snap(merges, { row: 1, col: 1 })).toEqual({ row: 1, col: 1 });
    expect(areaOf(merges, { row: 3, col: 1 })).toEqual({ r0: 2, c0: 1, r1: 4, c1: 1 });
  });

  it('moves out of a merge from its far edge', () => {
    expect(stepStart(merges, { row: 0, col: 0 }, 0, 1)).toEqual({ row: 0, col: 2 }); // → D1
    expect(stepStart(merges, { row: 2, col: 1 }, 1, 0)).toEqual({ row: 4, col: 1 }); // ↓ B6
    expect(stepStart(merges, { row: 1, col: 1 }, 1, 0)).toEqual({ row: 1, col: 1 });
  });

  it('expands ranges to whole merges, transitively', () => {
    expect(expand(merges, { r0: 1, c0: 0, r1: 2, c1: 1 })).toEqual({ r0: 1, c0: 0, r1: 4, c1: 1 }); // A2:B3 -> A2:B5
    expect(expand(merges, { r0: 1, c0: 0, r1: 2, c1: 0 })).toEqual({ r0: 1, c0: 0, r1: 2, c1: 0 }); // untouched
    expect(expand(merges, { r0: 0, c0: 1, r1: 2, c1: 1 })).toEqual({ r0: 0, c0: 0, r1: 4, c1: 2 });
  });
});
