// Selection model, Excel style. Pure functions over immutable values.
//
// - active: the cell that receives input (white cell inside the range)
// - anchor/focus: the current range is rangeOf(anchor, focus); Shift+move
//   moves focus while anchor and active stay put
// - ranges: every selected range; the last one is the current range
//   (⌘+click adds ranges)

export interface CellPos {
  row: number;
  col: number;
}

export interface Range {
  r0: number; // inclusive, r0 <= r1, c0 <= c1
  c0: number;
  r1: number;
  c1: number;
}

export interface Selection {
  active: CellPos;
  anchor: CellPos;
  focus: CellPos;
  ranges: Range[];
}

export interface Limits {
  rows: number;
  cols: number;
}

export function rangeOf(a: CellPos, b: CellPos): Range {
  return {
    r0: Math.min(a.row, b.row),
    c0: Math.min(a.col, b.col),
    r1: Math.max(a.row, b.row),
    c1: Math.max(a.col, b.col),
  };
}

export function clampPos(p: CellPos, lim: Limits): CellPos {
  return {
    row: Math.max(0, Math.min(p.row, lim.rows - 1)),
    col: Math.max(0, Math.min(p.col, lim.cols - 1)),
  };
}

export function single(pos: CellPos): Selection {
  return { active: pos, anchor: pos, focus: pos, ranges: [rangeOf(pos, pos)] };
}

export function current(sel: Selection): Range {
  return sel.ranges[sel.ranges.length - 1];
}

export function isSingleCell(sel: Selection): boolean {
  const r = current(sel);
  return sel.ranges.length === 1 && r.r0 === r.r1 && r.c0 === r.c1;
}

// Shift+click / Shift+arrow: replace the current range.
export function extendTo(sel: Selection, focus: CellPos): Selection {
  return { ...sel, focus, ranges: [...sel.ranges.slice(0, -1), rangeOf(sel.anchor, focus)] };
}

// ⌘+click: start an additional range.
export function addRange(sel: Selection, pos: CellPos): Selection {
  return { active: pos, anchor: pos, focus: pos, ranges: [...sel.ranges, rangeOf(pos, pos)] };
}

// Whole rows (row header click / Shift+Space). activeCol keeps the
// current column for Shift+Space; header clicks use column A.
export function selectRows(sel: Selection, from: number, to: number, lim: Limits, add = false, activeCol = 0): Selection {
  const r: Range = { r0: Math.min(from, to), c0: 0, r1: Math.max(from, to), c1: lim.cols - 1 };
  return {
    active: { row: from, col: activeCol },
    anchor: { row: from, col: 0 },
    focus: { row: to, col: lim.cols - 1 },
    ranges: add ? [...sel.ranges, r] : [r],
  };
}

export function selectCols(sel: Selection, from: number, to: number, lim: Limits, add = false, activeRow = 0): Selection {
  const r: Range = { r0: 0, c0: Math.min(from, to), r1: lim.rows - 1, c1: Math.max(from, to) };
  return {
    active: { row: activeRow, col: from },
    anchor: { row: 0, col: from },
    focus: { row: lim.rows - 1, col: to },
    ranges: add ? [...sel.ranges, r] : [r],
  };
}

export function selectAll(lim: Limits): Selection {
  return {
    active: { row: 0, col: 0 },
    anchor: { row: 0, col: 0 },
    focus: { row: lim.rows - 1, col: lim.cols - 1 },
    ranges: [{ r0: 0, c0: 0, r1: lim.rows - 1, c1: lim.cols - 1 }],
  };
}

export function fromRange(r: Range): Selection {
  const anchor = { row: r.r0, col: r.c0 };
  return { active: anchor, anchor, focus: { row: r.r1, col: r.c1 }, ranges: [r] };
}

export function contains(r: Range, row: number, col: number): boolean {
  return row >= r.r0 && row <= r.r1 && col >= r.c0 && col <= r.c1;
}

export function isFullRows(r: Range, lim: Limits): boolean {
  return r.c0 === 0 && r.c1 === lim.cols - 1;
}

export function isFullCols(r: Range, lim: Limits): boolean {
  return r.r0 === 0 && r.r1 === lim.rows - 1;
}

// Enter / Tab inside a multi-cell selection: move the active cell
// without collapsing the selection, wrapping within each range and then
// on to the next range (Excel behavior). byRow: Enter moves down columns
// first (false = Tab moves across rows first).
export function cycleActive(sel: Selection, byRow: boolean, backward: boolean): Selection {
  let idx = sel.ranges.findIndex((r) => contains(r, sel.active.row, sel.active.col));
  if (idx < 0) idx = sel.ranges.length - 1;
  const r = sel.ranges[idx];
  let { row, col } = sel.active;
  const step = backward ? -1 : 1;

  if (byRow) {
    row += step;
    if (row > r.r1 || row < r.r0) {
      row = backward ? r.r1 : r.r0;
      col += step;
    }
    if (col > r.c1 || col < r.c0) return jumpRange(sel, idx, step);
  } else {
    col += step;
    if (col > r.c1 || col < r.c0) {
      col = backward ? r.c1 : r.c0;
      row += step;
    }
    if (row > r.r1 || row < r.r0) return jumpRange(sel, idx, step);
  }
  return { ...sel, active: { row, col } };
}

function jumpRange(sel: Selection, idx: number, step: number): Selection {
  const n = sel.ranges.length;
  const next = sel.ranges[(idx + step + n) % n];
  const active = step > 0 ? { row: next.r0, col: next.c0 } : { row: next.r1, col: next.c1 };
  return { ...sel, active };
}
