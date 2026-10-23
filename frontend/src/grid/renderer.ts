import type { BlockCache } from '../stores/viewportCache';
import { AxisLayout, colName } from './layout';
import { current, isFullCols, isFullRows, isSingleCell, type Range, type Selection } from './selection';

// Wide enough for 7-digit row numbers; Excel-style auto width comes later.
export const HEADER_W = 60;
export const HEADER_H = 22;
const CELL_PAD = 4;

export interface VisibleRange {
  r0: number; // inclusive cell indices
  r1: number;
  c0: number;
  c1: number;
}

interface Theme {
  gridBg: string;
  gridLine: string;
  cellText: string;
  headerBg: string;
  headerText: string;
  headerLine: string;
  headerActiveBg: string;
  headerActiveText: string;
  headerFullBg: string;
  accent: string;
  selectionFill: string;
  font: string;
  headerFont: string;
  refColors: string[];
}

// Reference highlight while editing a formula; color is an index into
// the --ref-N palette.
export interface Highlight {
  range: Range;
  color: number;
  dashed: boolean;
}

export const REF_COLORS = 7;

function readTheme(): Theme {
  const css = getComputedStyle(document.documentElement);
  const v = (name: string) => css.getPropertyValue(name).trim();
  return {
    refColors: Array.from({ length: REF_COLORS }, (_, i) => v(`--ref-${i + 1}`)),
    gridBg: v('--grid-bg'),
    gridLine: v('--grid-line'),
    cellText: v('--cell-text'),
    headerBg: v('--header-bg'),
    headerText: v('--header-text'),
    headerLine: v('--header-line'),
    headerActiveBg: v('--header-active-bg'),
    headerActiveText: v('--header-active-text'),
    headerFullBg: v('--header-full-bg'),
    accent: v('--accent'),
    selectionFill: v('--selection-fill'),
    font: `13px ${v('--cell-font')}`,
    headerFont: `11px ${v('--ui-font')}`,
  };
}

// Shared sizing for both canvas layers.
abstract class Layer {
  protected ctx: CanvasRenderingContext2D;
  protected width = 0;
  protected height = 0;
  protected dpr = 1;

  constructor(protected canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext('2d', { alpha: true });
    if (!ctx) throw new Error('canvas 2d context unavailable');
    this.ctx = ctx;
  }

  resize(width: number, height: number, dpr: number): void {
    this.width = width;
    this.height = height;
    this.dpr = dpr;
    this.canvas.width = Math.round(width * dpr);
    this.canvas.height = Math.round(height * dpr);
    this.canvas.style.width = `${width}px`;
    this.canvas.style.height = `${height}px`;
  }

  // Snap a CSS-pixel coordinate to the center of a device pixel so
  // hairlines stay crisp.
  protected snap(px: number): number {
    return (Math.round(px * this.dpr) + 0.5) / this.dpr;
  }
}

export class GridRenderer extends Layer {
  private theme = readTheme();
  private textWidths = new Map<string, number>();

  constructor(
    canvas: HTMLCanvasElement,
    private rows: AxisLayout,
    private cols: AxisLayout,
    private cache: BlockCache,
  ) {
    super(canvas);
  }

  refreshTheme(): void {
    this.theme = readTheme();
  }

  visibleRange(sx: number, sy: number): VisibleRange {
    const contentW = this.width - HEADER_W;
    const contentH = this.height - HEADER_H;
    return {
      r0: this.rows.indexAt(sy),
      r1: this.rows.indexAt(sy + contentH),
      c0: this.cols.indexAt(sx),
      c1: this.cols.indexAt(sx + contentW),
    };
  }

  // Draws the whole grid layer; returns elapsed milliseconds.
  draw(sx: number, sy: number, sel: Selection): number {
    const t0 = performance.now();
    const { ctx, width: w, height: h, theme: th } = this;
    const vr = this.visibleRange(sx, sy);

    const cache = this.cache;
    cache.ensure({
      r0: Math.floor(vr.r0 / cache.blockRows),
      r1: Math.floor(vr.r1 / cache.blockRows),
      c0: Math.floor(vr.c0 / cache.blockCols),
      c1: Math.floor(vr.c1 / cache.blockCols),
    });

    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.fillStyle = th.gridBg;
    ctx.fillRect(0, 0, w, h);

    // Row/col pixel positions are computed once and shared by all passes.
    const ys = this.positions(this.rows, vr.r0, vr.r1, HEADER_H - sy);
    const xs = this.positions(this.cols, vr.c0, vr.c1, HEADER_W - sx);

    ctx.save();
    ctx.beginPath();
    ctx.rect(HEADER_W, HEADER_H, w - HEADER_W, h - HEADER_H);
    ctx.clip();
    this.drawGridLines(xs, ys);
    this.drawCells(vr, xs, ys);
    ctx.restore();

    this.drawHeaders(vr, xs, ys, sel);
    return performance.now() - t0;
  }

  // positions()[i] is the start of index first+i; the extra trailing
  // entry is the end of the last index.
  private positions(axis: AxisLayout, first: number, last: number, origin: number): number[] {
    const out = new Array<number>(last - first + 2);
    let p = origin + axis.offset(first);
    for (let i = first; i <= last; i++) {
      out[i - first] = p;
      p += axis.size(i);
    }
    out[last - first + 1] = p;
    return out;
  }

  private drawGridLines(xs: number[], ys: number[]): void {
    const { ctx, width: w, height: h } = this;
    ctx.beginPath();
    for (let i = 1; i < ys.length; i++) {
      if (ys[i] === ys[i - 1]) continue; // hidden row
      const y = this.snap(ys[i] - 1);
      ctx.moveTo(HEADER_W, y);
      ctx.lineTo(w, y);
    }
    for (let i = 1; i < xs.length; i++) {
      if (xs[i] === xs[i - 1]) continue;
      const x = this.snap(xs[i] - 1);
      ctx.moveTo(x, HEADER_H);
      ctx.lineTo(x, h);
    }
    ctx.strokeStyle = this.theme.gridLine;
    ctx.lineWidth = 1 / this.dpr;
    ctx.stroke();
  }

  private drawCells(vr: VisibleRange, xs: number[], ys: number[]): void {
    const { ctx, cache } = this;
    ctx.font = this.theme.font;
    ctx.fillStyle = this.theme.cellText;
    ctx.textBaseline = 'alphabetic';

    for (let r = vr.r0; r <= vr.r1; r++) {
      const top = ys[r - vr.r0];
      const bottom = ys[r - vr.r0 + 1];
      if (bottom === top) continue;
      const baseline = bottom - 5;

      for (let c = vr.c0; c <= vr.c1; c++) {
        const cell = cache.cell(r, c);
        if (!cell || cell.text === '') continue;
        const left = xs[c - vr.c0];
        const right = xs[c - vr.c0 + 1];
        const cellW = right - left;
        if (cellW === 0) continue;

        const textW = this.measure(cell.text);
        const x = cell.numeric
          ? right - CELL_PAD - textW
          : cell.center
            ? left + (cellW - textW) / 2
            : left + CELL_PAD;
        // Overflow into neighbours comes later; for now clip to the cell.
        if (textW > cellW - CELL_PAD * 2) {
          ctx.save();
          ctx.beginPath();
          ctx.rect(left, top, cellW - 1, bottom - top);
          ctx.clip();
          ctx.fillText(cell.text, x, baseline);
          ctx.restore();
        } else {
          ctx.fillText(cell.text, x, baseline);
        }
      }
    }
  }

  private measure(text: string): number {
    let w = this.textWidths.get(text);
    if (w === undefined) {
      w = this.ctx.measureText(text).width;
      if (this.textWidths.size > 20_000) this.textWidths.clear();
      this.textWidths.set(text, w);
    }
    return w;
  }

  // 0 = not selected, 1 = inside a selection, 2 = whole row/col selected.
  private headerState(sel: Selection, index: number, axis: 'row' | 'col'): number {
    const lim = { rows: this.rows.count, cols: this.cols.count };
    let state = 0;
    for (const r of sel.ranges) {
      const inside = axis === 'col' ? index >= r.c0 && index <= r.c1 : index >= r.r0 && index <= r.r1;
      if (!inside) continue;
      const full = axis === 'col' ? isFullCols(r, lim) : isFullRows(r, lim);
      state = Math.max(state, full ? 2 : 1);
    }
    return state;
  }

  private drawHeaders(vr: VisibleRange, xs: number[], ys: number[], sel: Selection): void {
    const { ctx, width: w, height: h, theme: th } = this;

    ctx.fillStyle = th.headerBg;
    ctx.fillRect(0, 0, w, HEADER_H);
    ctx.fillRect(0, 0, HEADER_W, h);

    ctx.font = th.headerFont;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';

    const colStates = new Array<number>(vr.c1 - vr.c0 + 1);
    const rowStates = new Array<number>(vr.r1 - vr.r0 + 1);

    // Column header
    ctx.save();
    ctx.beginPath();
    ctx.rect(HEADER_W, 0, w - HEADER_W, HEADER_H);
    ctx.clip();
    for (let c = vr.c0; c <= vr.c1; c++) {
      const left = xs[c - vr.c0];
      const right = xs[c - vr.c0 + 1];
      if (right === left) continue;
      const state = (colStates[c - vr.c0] = this.headerState(sel, c, 'col'));
      if (state > 0) {
        ctx.fillStyle = state === 2 ? th.headerFullBg : th.headerActiveBg;
        ctx.fillRect(left, 0, right - left, HEADER_H);
      }
      ctx.fillStyle = state > 0 ? th.headerActiveText : th.headerText;
      ctx.fillText(colName(c), (left + right) / 2, HEADER_H / 2 + 1);
    }
    ctx.restore();

    // Row header
    ctx.save();
    ctx.beginPath();
    ctx.rect(0, HEADER_H, HEADER_W, h - HEADER_H);
    ctx.clip();
    for (let r = vr.r0; r <= vr.r1; r++) {
      const top = ys[r - vr.r0];
      const bottom = ys[r - vr.r0 + 1];
      if (bottom === top) continue;
      const state = (rowStates[r - vr.r0] = this.headerState(sel, r, 'row'));
      if (state > 0) {
        ctx.fillStyle = state === 2 ? th.headerFullBg : th.headerActiveBg;
        ctx.fillRect(0, top, HEADER_W, bottom - top);
      }
      ctx.fillStyle = state > 0 ? th.headerActiveText : th.headerText;
      ctx.fillText(String(r + 1), HEADER_W / 2, (top + bottom) / 2);
    }
    ctx.restore();
    ctx.textAlign = 'start';

    // Header separators
    ctx.beginPath();
    for (let i = 1; i < xs.length; i++) {
      if (xs[i] < HEADER_W) continue;
      const x = this.snap(xs[i] - 1);
      ctx.moveTo(x, 0);
      ctx.lineTo(x, HEADER_H);
    }
    for (let i = 1; i < ys.length; i++) {
      if (ys[i] < HEADER_H) continue;
      const y = this.snap(ys[i] - 1);
      ctx.moveTo(0, y);
      ctx.lineTo(HEADER_W, y);
    }
    const hy = this.snap(HEADER_H - 1);
    const hx = this.snap(HEADER_W - 1);
    ctx.moveTo(0, hy);
    ctx.lineTo(w, hy);
    ctx.moveTo(hx, 0);
    ctx.lineTo(hx, h);
    ctx.strokeStyle = th.headerLine;
    ctx.lineWidth = 1 / this.dpr;
    ctx.stroke();

    // Accent edge on selected headers, as in Excel
    ctx.fillStyle = th.accent;
    ctx.save();
    ctx.beginPath();
    ctx.rect(HEADER_W, 0, w - HEADER_W, HEADER_H);
    ctx.clip();
    for (let i = 0; i < colStates.length; i++) {
      if (colStates[i] > 0) ctx.fillRect(xs[i], HEADER_H - 2, xs[i + 1] - xs[i], 2);
    }
    ctx.restore();
    ctx.save();
    ctx.beginPath();
    ctx.rect(0, HEADER_H, HEADER_W, h - HEADER_H);
    ctx.clip();
    for (let i = 0; i < rowStates.length; i++) {
      if (rowStates[i] > 0) ctx.fillRect(HEADER_W - 2, ys[i], 2, ys[i + 1] - ys[i]);
    }
    ctx.restore();
  }
}

// Rectangle of a cell range in viewport coordinates, clamped to a margin
// around the viewport so whole-sheet ranges stay small numbers.
export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export function rangeRect(rows: AxisLayout, cols: AxisLayout, r: Range, sx: number, sy: number): Rect {
  const x0 = HEADER_W + cols.offset(r.c0) - sx;
  const x1 = HEADER_W + cols.offset(r.c1 + 1) - sx;
  const y0 = HEADER_H + rows.offset(r.r0) - sy;
  const y1 = HEADER_H + rows.offset(r.r1 + 1) - sy;
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

function clampRect(rc: Rect, width: number, height: number): Rect {
  const m = 8;
  const x0 = Math.max(rc.x, -m);
  const y0 = Math.max(rc.y, -m);
  const x1 = Math.min(rc.x + rc.w, width + m);
  const y1 = Math.min(rc.y + rc.h, height + m);
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

// Selection layer: redrawn independently of the grid layer.
export class SelectionRenderer extends Layer {
  private theme = readTheme();

  constructor(
    canvas: HTMLCanvasElement,
    private rows: AxisLayout,
    private cols: AxisLayout,
  ) {
    super(canvas);
  }

  refreshTheme(): void {
    this.theme = readTheme();
  }

  draw(sx: number, sy: number, sel: Selection, highlights: readonly Highlight[] = []): void {
    const { ctx, width: w, height: h, theme: th } = this;
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);

    ctx.save();
    ctx.beginPath();
    ctx.rect(HEADER_W, HEADER_H, w - HEADER_W, h - HEADER_H);
    ctx.clip();

    const single = isSingleCell(sel);
    const rect = (r: Range) => clampRect(rangeRect(this.rows, this.cols, r, sx, sy), w, h);

    if (!single) {
      ctx.fillStyle = th.selectionFill;
      for (const r of sel.ranges) {
        const rc = rect(r);
        if (rc.w > 0 && rc.h > 0) ctx.fillRect(rc.x, rc.y, rc.w, rc.h);
      }
      // The active cell stays unfilled.
      const a = rect({ r0: sel.active.row, c0: sel.active.col, r1: sel.active.row, c1: sel.active.col });
      ctx.clearRect(a.x, a.y, a.w - 1, a.h - 1);
    }

    ctx.strokeStyle = th.accent;
    if (sel.ranges.length > 1) {
      // Multiple ranges: thin outline on the active cell only.
      const a = rect({ r0: sel.active.row, c0: sel.active.col, r1: sel.active.row, c1: sel.active.col });
      ctx.lineWidth = 1;
      ctx.strokeRect(a.x - 0.5, a.y - 0.5, a.w, a.h);
    } else {
      const rc = rect(current(sel));
      if (rc.w > 0 && rc.h > 0) {
        ctx.lineWidth = 2;
        ctx.strokeRect(rc.x - 1, rc.y - 1, rc.w, rc.h);
        // Fill handle with a background-colored ring
        const fx = rc.x + rc.w - 4;
        const fy = rc.y + rc.h - 4;
        ctx.fillStyle = th.gridBg;
        ctx.fillRect(fx - 1, fy - 1, 8, 8);
        ctx.fillStyle = th.accent;
        ctx.fillRect(fx, fy, 6, 6);
      }
    }
    this.drawHighlights(highlights, rect);
    ctx.restore();
  }

  // Colored boxes for the references in the formula being edited.
  private drawHighlights(highlights: readonly Highlight[], rect: (r: Range) => Rect): void {
    const { ctx, theme: th } = this;
    for (const hl of highlights) {
      const rc = rect(hl.range);
      if (rc.w <= 0 || rc.h <= 0) continue;
      const color = th.refColors[hl.color % th.refColors.length];
      ctx.globalAlpha = 0.12;
      ctx.fillStyle = color;
      ctx.fillRect(rc.x, rc.y, rc.w, rc.h);
      ctx.globalAlpha = 1;
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      ctx.setLineDash(hl.dashed ? [5, 3] : []);
      ctx.strokeRect(rc.x - 0.5, rc.y - 0.5, rc.w - 1, rc.h - 1);
      ctx.setLineDash([]);
      if (!hl.dashed) {
        // Corner handles, as in Excel.
        ctx.fillStyle = color;
        for (const [cx, cy] of [
          [rc.x, rc.y],
          [rc.x + rc.w, rc.y],
          [rc.x, rc.y + rc.h],
          [rc.x + rc.w, rc.y + rc.h],
        ]) {
          ctx.fillRect(cx - 3, cy - 3, 5, 5);
        }
      }
    }
  }
}

