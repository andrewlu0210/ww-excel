// Fill handle geometry, pure so it can be unit tested.
import type { CellPos, Range } from './selection';

// The range a fill-handle drag from src to pos covers: src extended
// along the axis the pointer moved further on, or shrunk when the
// pointer is back inside src (the cells left out are cleared).
export function fillTarget(src: Range, pos: CellPos): Range {
  const dr = pos.row > src.r1 ? pos.row - src.r1 : pos.row < src.r0 ? pos.row - src.r0 : 0;
  const dc = pos.col > src.c1 ? pos.col - src.c1 : pos.col < src.c0 ? pos.col - src.c0 : 0;
  if (dr !== 0 && Math.abs(dr) >= Math.abs(dc)) {
    return dr > 0 ? { ...src, r1: pos.row } : { ...src, r0: pos.row };
  }
  if (dc !== 0) return dc > 0 ? { ...src, c1: pos.col } : { ...src, c0: pos.col };
  // Inside: shrink from the bottom (or the right for a single row).
  if (src.r1 > src.r0 && pos.row < src.r1) return { ...src, r1: pos.row };
  if (src.c1 > src.c0 && pos.col < src.c1) return { ...src, c1: pos.col };
  return src;
}

export function sameRange(a: Range, b: Range): boolean {
  return a.r0 === b.r0 && a.c0 === b.c0 && a.r1 === b.r1 && a.c1 === b.c1;
}

// ⌘D / ⌘R: the source is the first row (column) of the selection, or the
// row above (column left of) a single-row selection. Null at the edge.
export function fillCopy(sel: Range, down: boolean): { src: Range; dst: Range } | null {
  if (down) {
    if (sel.r0 === sel.r1) {
      if (sel.r0 === 0) return null;
      return { src: { ...sel, r0: sel.r0 - 1, r1: sel.r0 - 1 }, dst: { ...sel, r0: sel.r0 - 1 } };
    }
    return { src: { ...sel, r1: sel.r0 }, dst: sel };
  }
  if (sel.c0 === sel.c1) {
    if (sel.c0 === 0) return null;
    return { src: { ...sel, c0: sel.c0 - 1, c1: sel.c0 - 1 }, dst: { ...sel, c0: sel.c0 - 1 } };
  }
  return { src: { ...sel, c1: sel.c0 }, dst: sel };
}
