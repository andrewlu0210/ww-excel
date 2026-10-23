import { describe, expect, it } from 'vitest';
import {
  addRange,
  current,
  cycleActive,
  extendTo,
  isSingleCell,
  rangeOf,
  selectAll,
  selectCols,
  selectRows,
  single,
} from './selection';

const lim = { rows: 1_048_576, cols: 16_384 };

describe('selection', () => {
  it('extends from the anchor and keeps the active cell', () => {
    const s = extendTo(single({ row: 5, col: 5 }), { row: 2, col: 8 });
    expect(current(s)).toEqual({ r0: 2, c0: 5, r1: 5, c1: 8 });
    expect(s.active).toEqual({ row: 5, col: 5 });
    expect(isSingleCell(s)).toBe(false);
  });

  it('adds ranges with ⌘+click', () => {
    const s = addRange(single({ row: 0, col: 0 }), { row: 3, col: 3 });
    expect(s.ranges).toHaveLength(2);
    expect(s.active).toEqual({ row: 3, col: 3 });
    expect(extendTo(s, { row: 4, col: 4 }).ranges[0]).toEqual(rangeOf({ row: 0, col: 0 }, { row: 0, col: 0 }));
  });

  it('selects whole rows, columns and the sheet', () => {
    const base = single({ row: 9, col: 4 });
    const rows = selectRows(base, 9, 9, lim, false, 4);
    expect(current(rows)).toEqual({ r0: 9, c0: 0, r1: 9, c1: lim.cols - 1 });
    expect(rows.active).toEqual({ row: 9, col: 4 });
    // Extending a row selection keeps full rows.
    expect(current(extendTo(rows, { row: 12, col: 0 })).c1).toBe(0);
    expect(current(selectCols(base, 3, 1, lim))).toEqual({ r0: 0, c0: 1, r1: lim.rows - 1, c1: 3 });
    expect(current(selectAll(lim))).toEqual({ r0: 0, c0: 0, r1: lim.rows - 1, c1: lim.cols - 1 });
  });

  it('cycles Enter down columns and wraps to the next column', () => {
    let s = extendTo(single({ row: 0, col: 0 }), { row: 1, col: 1 });
    const seen: string[] = [];
    for (let i = 0; i < 5; i++) {
      seen.push(`${s.active.row},${s.active.col}`);
      s = cycleActive(s, true, false);
    }
    expect(seen).toEqual(['0,0', '1,0', '0,1', '1,1', '0,0']);
    expect(current(s)).toEqual({ r0: 0, c0: 0, r1: 1, c1: 1 });
  });

  it('cycles Tab across rows, backwards too', () => {
    const s = extendTo(single({ row: 0, col: 0 }), { row: 1, col: 1 });
    expect(cycleActive(s, false, false).active).toEqual({ row: 0, col: 1 });
    expect(cycleActive(s, false, true).active).toEqual({ row: 1, col: 1 });
  });

  it('moves on to the next range', () => {
    const s = addRange(extendTo(single({ row: 0, col: 0 }), { row: 1, col: 0 }), { row: 5, col: 5 });
    // Active is in the second (single-cell) range; Enter wraps to range 0.
    expect(cycleActive(s, true, false).active).toEqual({ row: 0, col: 0 });
  });
});
