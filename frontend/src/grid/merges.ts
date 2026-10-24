// Merged cells and selection, like Excel: a merged area behaves as one
// cell whose address is its top-left (anchor) cell.
import type { CellPos, Range } from './selection';

export function mergeAt(merges: readonly Range[], pos: CellPos): Range | null {
  for (const m of merges) {
    if (pos.row >= m.r0 && pos.row <= m.r1 && pos.col >= m.c0 && pos.col <= m.c1) return m;
  }
  return null;
}

// The anchor of the merge containing pos, or pos itself.
export function snap(merges: readonly Range[], pos: CellPos): CellPos {
  const m = mergeAt(merges, pos);
  return m ? { row: m.r0, col: m.c0 } : pos;
}

// The area a single cell selection covers.
export function areaOf(merges: readonly Range[], pos: CellPos): Range {
  return mergeAt(merges, pos) ?? { r0: pos.row, c0: pos.col, r1: pos.row, c1: pos.col };
}

// Grows r until it fully contains every merge it touches.
export function expand(merges: readonly Range[], r: Range): Range {
  let out = { ...r };
  for (let changed = true; changed; ) {
    changed = false;
    for (const m of merges) {
      const touches = m.r0 <= out.r1 && m.r1 >= out.r0 && m.c0 <= out.c1 && m.c1 >= out.c0;
      if (!touches) continue;
      const next = {
        r0: Math.min(out.r0, m.r0),
        c0: Math.min(out.c0, m.c0),
        r1: Math.max(out.r1, m.r1),
        c1: Math.max(out.c1, m.c1),
      };
      if (next.r0 !== out.r0 || next.c0 !== out.c0 || next.r1 !== out.r1 || next.c1 !== out.c1) {
        out = next;
        changed = true;
      }
    }
  }
  return out;
}

// Where an arrow move starts from pos: the far edge of its merge in the
// direction of travel, so moving out of a merge skips its covered cells.
export function stepStart(merges: readonly Range[], pos: CellPos, dr: number, dc: number): CellPos {
  const m = mergeAt(merges, pos);
  if (!m) return pos;
  return {
    row: dr > 0 ? m.r1 : dr < 0 ? m.r0 : pos.row,
    col: dc > 0 ? m.c1 : dc < 0 ? m.c0 : pos.col,
  };
}
