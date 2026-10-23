import type { BlockCache } from '../stores/viewportCache';
import { AxisLayout, colName } from './layout';

// Wide enough for 7-digit row numbers; Excel-style auto width comes later.
export const HEADER_W = 60;
export const HEADER_H = 22;
const CELL_PAD = 4;

export interface CellPos {
  row: number;
  col: number;
}

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
  accent: string;
  font: string;
  headerFont: string;
}

function readTheme(): Theme {
  const css = getComputedStyle(document.documentElement);
  const v = (name: string) => css.getPropertyValue(name).trim();
  return {
    gridBg: v('--grid-bg'),
    gridLine: v('--grid-line'),
    cellText: v('--cell-text'),
    headerBg: v('--header-bg'),
    headerText: v('--header-text'),
    headerLine: v('--header-line'),
    headerActiveBg: v('--header-active-bg'),
    headerActiveText: v('--header-active-text'),
    accent: v('--accent'),
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
  draw(sx: number, sy: number, active: CellPos): number {
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

    this.drawHeaders(vr, xs, ys, active);
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
        const x = cell.numeric ? right - CELL_PAD - textW : left + CELL_PAD;
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

  private drawHeaders(vr: VisibleRange, xs: number[], ys: number[], active: CellPos): void {
    const { ctx, width: w, height: h, theme: th } = this;

    ctx.fillStyle = th.headerBg;
    ctx.fillRect(0, 0, w, HEADER_H);
    ctx.fillRect(0, 0, HEADER_W, h);

    ctx.font = th.headerFont;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';

    // Column header
    ctx.save();
    ctx.beginPath();
    ctx.rect(HEADER_W, 0, w - HEADER_W, HEADER_H);
    ctx.clip();
    for (let c = vr.c0; c <= vr.c1; c++) {
      const left = xs[c - vr.c0];
      const right = xs[c - vr.c0 + 1];
      if (right === left) continue;
      const isActive = c === active.col;
      if (isActive) {
        ctx.fillStyle = th.headerActiveBg;
        ctx.fillRect(left, 0, right - left, HEADER_H);
      }
      ctx.fillStyle = isActive ? th.headerActiveText : th.headerText;
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
      const isActive = r === active.row;
      if (isActive) {
        ctx.fillStyle = th.headerActiveBg;
        ctx.fillRect(0, top, HEADER_W, bottom - top);
      }
      ctx.fillStyle = isActive ? th.headerActiveText : th.headerText;
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

    // Active header underline, as in Excel
    const activeX = active.col - vr.c0;
    if (activeX >= 0 && activeX < xs.length - 1) {
      ctx.fillStyle = th.accent;
      ctx.fillRect(xs[activeX], HEADER_H - 2, xs[activeX + 1] - xs[activeX], 2);
    }
    const activeY = active.row - vr.r0;
    if (activeY >= 0 && activeY < ys.length - 1) {
      ctx.fillStyle = th.accent;
      ctx.fillRect(HEADER_W - 2, ys[activeY], 2, ys[activeY + 1] - ys[activeY]);
    }
  }
}

// Selection layer: redrawn independently of the grid layer.
export class SelectionRenderer extends Layer {
  private accent = readTheme().accent;

  constructor(
    canvas: HTMLCanvasElement,
    private rows: AxisLayout,
    private cols: AxisLayout,
  ) {
    super(canvas);
  }

  refreshTheme(): void {
    this.accent = readTheme().accent;
  }

  draw(sx: number, sy: number, active: CellPos): void {
    const { ctx, width: w, height: h } = this;
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);

    const cw = this.cols.size(active.col);
    const ch = this.rows.size(active.row);
    if (cw === 0 || ch === 0) return;
    const x = HEADER_W + this.cols.offset(active.col) - sx;
    const y = HEADER_H + this.rows.offset(active.row) - sy;
    if (x > w || y > h || x + cw < HEADER_W || y + ch < HEADER_H) return;

    ctx.save();
    ctx.beginPath();
    ctx.rect(HEADER_W, HEADER_H, w - HEADER_W, h - HEADER_H);
    ctx.clip();
    ctx.strokeStyle = this.accent;
    ctx.lineWidth = 2;
    ctx.strokeRect(x - 1, y - 1, cw, ch);
    // Fill handle
    ctx.fillStyle = this.accent;
    ctx.fillRect(x + cw - 4, y + ch - 4, 6, 6);
    ctx.restore();
  }
}
