import { describe, expect, it } from 'vitest';
import { fillCopy, fillTarget } from './fill';

const src = { r0: 2, c0: 1, r1: 3, c1: 1 };

describe('fillTarget', () => {
  it('extends along the dominant direction', () => {
    expect(fillTarget(src, { row: 9, col: 2 })).toEqual({ r0: 2, c0: 1, r1: 9, c1: 1 });
    expect(fillTarget(src, { row: 3, col: 5 })).toEqual({ r0: 2, c0: 1, r1: 3, c1: 5 });
    expect(fillTarget(src, { row: 0, col: 1 })).toEqual({ r0: 0, c0: 1, r1: 3, c1: 1 });
  });
  it('shrinks when dragged back inside', () => {
    expect(fillTarget(src, { row: 2, col: 1 })).toEqual({ r0: 2, c0: 1, r1: 2, c1: 1 });
  });
});

describe('fillCopy', () => {
  it('uses the row above a single row', () => {
    expect(fillCopy({ r0: 4, c0: 0, r1: 4, c1: 2 }, true)).toEqual({
      src: { r0: 3, c0: 0, r1: 3, c1: 2 },
      dst: { r0: 3, c0: 0, r1: 4, c1: 2 },
    });
    expect(fillCopy({ r0: 0, c0: 0, r1: 0, c1: 0 }, true)).toBeNull();
  });
  it('uses the first column of a block', () => {
    expect(fillCopy({ r0: 0, c0: 2, r1: 5, c1: 4 }, false)?.src).toEqual({ r0: 0, c0: 2, r1: 5, c1: 2 });
  });
});
