import type { BorderLineDTO } from '../../bindings/ww-excel/internal/service/models';
import { styleStore, type StyleRender } from '../stores/styles';
import type { BlockCache, CellView } from '../stores/viewportCache';
import { AxisLayout, colName } from './layout';
import { current, isFullCols, isFullRows, isSingleCell, type Range, type Selection } from './selection';
import { LINE_HEIGHT, TextCache } from './text';

export const HEADER_H = 22;

// The row header widens with the number of digits of the rows on screen,
// like Excel. Shared by the renderers, the grid and the editor layout.
export const metrics = { headerW: 40 };

export function headerWidthFor(lastRow: number): number {
  const digits = String(lastRow + 1).length;
  return Math.max(32, Math.ceil(digits * 7.2 + 14));
}

const CELL_PAD = 3;
const INDENT_PX = 9;
// Text in columns left of the viewport may overflow into it.
const OVERFLOW_LOOKBACK = 8;

export interface VisibleRange {
  r0: number; // inclusive cell indices
  r1: number;
  c0: number;
  c1: number;
}

// Sheet-level formatting the renderer needs besides cells.
export interface SheetLook {
  rowStyles: Map<number, number>;
  colStyles: Map<number, number>;
  merges: Range[];
}

export const emptyLook = (): SheetLook => ({ rowStyles: new Map(), colStyles: new Map(), merges: [] });

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

// Rectangle of a cell range in viewport coordinates (may be off screen).
export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export function rangeRect(rows: AxisLayout, cols: AxisLayout, r: Range, sx: number, sy: number): Rect {
  const x0 = metrics.headerW + cols.offset(r.c0) - sx;
  const x1 = metrics.headerW + cols.offset(r.c1 + 1) - sx;
  const y0 = HEADER_H + rows.offset(r.r0) - sy;
  const y1 = HEADER_H + rows.offset(r.r1 + 1) - sy;
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

// Clamped to a margin around the viewport so whole-sheet ranges stay
// small numbers.
function clampRect(rc: Rect, width: number, height: number): Rect {
  const m = 8;
  const x0 = Math.max(rc.x, -m);
  const y0 = Math.max(rc.y, -m);
  const x1 = Math.min(rc.x + rc.w, width + m);
  const y1 = Math.min(rc.y + rc.h, height + m);
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

// Automatic text color on a filled cell: dark on light fills, light on
// dark ones, so fills stay readable in dark mode too.
function contrastText(fill: string): string {
  const n = parseInt(fill.slice(1), 16);
  const lum = (0.299 * ((n >> 16) & 255) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255)) / 255;
  return lum > 0.55 ? '#1f1f1f' : '#f2f2f2';
}

const BORDER_WIDTH: Record<string, number> = { medium: 2, thick: 3, double: 3, mediumDashed: 2, mediumDashDot: 2, mediumDashDotDot: 2 };
const BORDER_DASH: Record<string, number[]> = {
  dashed: [3, 2],
  dotted: [1, 1],
  hair: [1, 1],
  mediumDashed: [6, 2],
  dashDot: [6, 2, 2, 2],
  mediumDashDot: [6, 2, 2, 2],
  dashDotDot: [6, 2, 2, 2, 2, 2],
  mediumDashDotDot: [6, 2, 2, 2, 2, 2],
  slantDashDot: [6, 2, 2, 2],
};

export class GridRenderer extends Layer {
  private theme = readTheme();
  private text: TextCache;
  look: SheetLook = emptyLook();

  constructor(
    canvas: HTMLCanvasElement,
    private rows: AxisLayout,
    private cols: AxisLayout,
    private cache: BlockCache,
  ) {
    super(canvas);
    this.text = new TextCache(this.ctx);
  }

  refreshTheme(): void {
    this.theme = readTheme();
  }

  visibleRange(sx: number, sy: number): VisibleRange {
    const contentW = this.width - metrics.headerW;
    const contentH = this.height - HEADER_H;
    return {
      r0: this.rows.indexAt(sy),
      r1: this.rows.indexAt(sy + contentH),
      c0: this.cols.indexAt(sx),
      c1: this.cols.indexAt(sx + contentW),
    };
  }

  // Style ID that applies at a position: the cell's own, else the row
  // or column style.
  styleAt(row: number, col: number, cell = this.cache.cell(row, col)): number {
    if (cell?.style) return cell.style;
    return this.look.rowStyles.get(row) ?? this.look.colStyles.get(col) ?? 0;
  }

  // Draws the whole grid layer; returns elapsed milliseconds.
  draw(sx: number, sy: number, sel: Selection): number {
    const t0 = performance.now();
    const { ctx, width: w, height: h, theme: th } = this;
    const vr = this.visibleRange(sx, sy);
    const hw = metrics.headerW;

    // Text from columns left of the viewport may overflow into it.
    const first = Math.max(0, vr.c0 - OVERFLOW_LOOKBACK);
    const cache = this.cache;
    const blocks = {
      r0: Math.floor(vr.r0 / cache.blockRows),
      r1: Math.floor(vr.r1 / cache.blockRows),
      c0: Math.floor(first / cache.blockCols),
      c1: Math.floor(vr.c1 / cache.blockCols),
    };
    cache.ensure(blocks);
    const merges = this.look.merges.filter((m) => m.r1 >= vr.r0 && m.r0 <= vr.r1 && m.c1 >= first && m.c0 <= vr.c1);
    for (const m of merges) {
      // The anchor of a merge may sit outside the loaded blocks.
      const br = Math.floor(m.r0 / cache.blockRows);
      const bc = Math.floor(m.c0 / cache.blockCols);
      if (br < blocks.r0 || bc < blocks.c0) cache.ensure({ r0: br, r1: br, c0: bc, c1: bc });
    }

    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.fillStyle = th.gridBg;
    ctx.fillRect(0, 0, w, h);

    // Row/col pixel positions are computed once and shared by all passes.
    const ys = this.positions(this.rows, vr.r0, vr.r1, HEADER_H - sy);
    const xs = this.positions(this.cols, first, vr.c1, hw - sx);
    const cv: VisibleRange = { ...vr, c0: first };

    // Cells covered by merges (other than the anchor) are drawn by the
    // merge.
    const covered = new Set<number>();
    for (const m of merges) {
      for (let r = Math.max(m.r0, vr.r0); r <= Math.min(m.r1, vr.r1); r++) {
        for (let c = Math.max(m.c0, first); c <= Math.min(m.c1, vr.c1); c++) covered.add(r * 16384 + c);
      }
    }

    ctx.save();
    ctx.beginPath();
    ctx.rect(hw, HEADER_H, w - hw, h - HEADER_H);
    ctx.clip();
    this.drawGridLines(xs, ys);
    this.drawFills(cv, xs, ys, covered, merges, sx, sy);
    this.drawBorders(cv, xs, ys, covered, merges, sx, sy);
    this.drawTexts(cv, xs, ys, covered, merges, sx, sy);
    ctx.restore();

    this.drawHeaders(vr, xs.slice(vr.c0 - first), ys, sel);
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
    const hw = metrics.headerW;
    ctx.beginPath();
    for (let i = 1; i < ys.length; i++) {
      if (ys[i] === ys[i - 1]) continue; // hidden row
      const y = this.snap(ys[i] - 1);
      ctx.moveTo(hw, y);
      ctx.lineTo(w, y);
    }
    for (let i = 1; i < xs.length; i++) {
      if (xs[i] === xs[i - 1] || xs[i] < hw) continue;
      const x = this.snap(xs[i] - 1);
      ctx.moveTo(x, HEADER_H);
      ctx.lineTo(x, h);
    }
    ctx.strokeStyle = this.theme.gridLine;
    ctx.lineWidth = 1 / this.dpr;
    ctx.stroke();
  }

  // Fills cover the grid lines, as in Excel. Merged areas are filled
  // with their anchor's fill (or the background, hiding inner lines).
  private drawFills(vr: VisibleRange, xs: number[], ys: number[], covered: Set<number>, merges: Range[], sx: number, sy: number): void {
    const ctx = this.ctx;
    for (let r = vr.r0; r <= vr.r1; r++) {
      const top = ys[r - vr.r0];
      const bottom = ys[r - vr.r0 + 1];
      if (bottom === top) continue;
      for (let c = vr.c0; c <= vr.c1; c++) {
        if (covered.has(r * 16384 + c)) continue;
        const st = styleStore.get(this.styleAt(r, c));
        if (!st?.fill) continue;
        const left = xs[c - vr.c0];
        const right = xs[c - vr.c0 + 1];
        ctx.fillStyle = st.fill;
        ctx.fillRect(left, top, right - left, bottom - top);
      }
    }
    for (const m of merges) {
      const rc = rangeRect(this.rows, this.cols, m, sx, sy);
      const st = styleStore.get(this.styleAt(m.r0, m.c0));
      ctx.fillStyle = st?.fill ?? this.theme.gridBg;
      ctx.fillRect(rc.x, rc.y, rc.w - 1, rc.h - 1);
    }
  }

  private line(b: BorderLineDTO, x0: number, y0: number, x1: number, y1: number): void {
    if (!b.style) return;
    const ctx = this.ctx;
    const width = BORDER_WIDTH[b.style] ?? 1;
    ctx.strokeStyle = b.color ? `#${b.color}` : this.theme.cellText;
    ctx.lineWidth = b.style === 'double' ? 1 : width;
    ctx.setLineDash(BORDER_DASH[b.style] ?? []);
    const horizontal = y0 === y1;
    const draw = (offset: number) => {
      ctx.beginPath();
      if (horizontal) {
        const y = this.snap(y0 + offset);
        ctx.moveTo(x0, y);
        ctx.lineTo(x1, y);
      } else {
        const x = this.snap(x0 + offset);
        ctx.moveTo(x, y0);
        ctx.lineTo(x, y1);
      }
      ctx.stroke();
    };
    if (b.style === 'double') {
      draw(-1);
      draw(1);
    } else {
      draw(0);
    }
    ctx.setLineDash([]);
  }

  private cellBorders(st: StyleRender | undefined, left: number, top: number, right: number, bottom: number): void {
    if (!st) return;
    const d = st.dto;
    // Lines sit on the cell edge (the grid line position).
    this.line(d.top, left - 1, top - 1, right, top - 1);
    this.line(d.bottom, left - 1, bottom - 1, right, bottom - 1);
    this.line(d.left, left - 1, top - 1, left - 1, bottom);
    this.line(d.right, right - 1, top - 1, right - 1, bottom);
  }

  private drawBorders(vr: VisibleRange, xs: number[], ys: number[], covered: Set<number>, merges: Range[], sx: number, sy: number): void {
    for (let r = vr.r0; r <= vr.r1; r++) {
      const top = ys[r - vr.r0];
      const bottom = ys[r - vr.r0 + 1];
      if (bottom === top) continue;
      for (let c = vr.c0; c <= vr.c1; c++) {
        if (covered.has(r * 16384 + c)) continue;
        const id = this.styleAt(r, c);
        if (id === 0) continue;
        const st = styleStore.get(id);
        const d = st?.dto;
        if (!d || !(d.top.style || d.bottom.style || d.left.style || d.right.style)) continue;
        this.cellBorders(st, xs[c - vr.c0], top, xs[c - vr.c0 + 1], bottom);
      }
    }
    for (const m of merges) {
      const rc = rangeRect(this.rows, this.cols, m, sx, sy);
      this.cellBorders(styleStore.get(this.styleAt(m.r0, m.c0)), rc.x, rc.y, rc.x + rc.w, rc.y + rc.h);
    }
  }

  private drawTexts(vr: VisibleRange, xs: number[], ys: number[], covered: Set<number>, merges: Range[], sx: number, sy: number): void {
    for (let r = vr.r0; r <= vr.r1; r++) {
      const top = ys[r - vr.r0];
      const bottom = ys[r - vr.r0 + 1];
      if (bottom === top) continue;
      for (let c = vr.c0; c <= vr.c1; c++) {
        if (covered.has(r * 16384 + c)) continue;
        const cell = this.cache.cell(r, c);
        if (!cell || cell.text === '') continue;
        const left = xs[c - vr.c0];
        const right = xs[c - vr.c0 + 1];
        if (right === left) continue;
        this.cellText(cell, styleStore.get(this.styleAt(r, c, cell)), left, top, right, bottom, (dir) =>
          this.overflowEdge(r, c, dir, vr, xs, covered),
        );
      }
    }
    for (const m of merges) {
      const cell = this.cache.cell(m.r0, m.c0);
      if (!cell || cell.text === '') continue;
      const rc = rangeRect(this.rows, this.cols, m, sx, sy);
      this.cellText(cell, styleStore.get(this.styleAt(m.r0, m.c0, cell)), rc.x, rc.y, rc.x + rc.w, rc.y + rc.h, null);
    }
  }

  // How far text overflowing from (r, c) may extend in direction dir:
  // across empty neighbors, stopping at any cell with content or a merge.
  private overflowEdge(r: number, c: number, dir: 1 | -1, vr: VisibleRange, xs: number[], covered: Set<number>): number {
    let k = c + dir;
    while (k >= vr.c0 && k <= vr.c1) {
      const n = this.cache.cell(r, k);
      if ((n && n.text !== '') || covered.has(r * 16384 + k)) break;
      k += dir;
    }
    return dir > 0 ? xs[Math.min(k, vr.c1 + 1) - vr.c0] : xs[Math.max(k + 1, vr.c0) - vr.c0];
  }

  private cellText(
    cell: CellView,
    st: StyleRender | undefined,
    left: number,
    top: number,
    right: number,
    bottom: number,
    overflow: ((dir: 1 | -1) => number) | null,
  ): void {
    const ctx = this.ctx;
    const d = st?.dto;
    const font = st?.font ?? `14.67px -apple-system, sans-serif`;
    const px = st?.fontPx ?? 14.67;
    const indent = (d?.indent ?? 0) * INDENT_PX;
    const pad = CELL_PAD + indent;
    const cellW = right - left;
    let align = d?.alignH ?? '';
    if (align === '' || align === 'general') align = cell.numeric ? 'right' : cell.center ? 'center' : 'left';
    if (align === 'justify' || align === 'distributed' || align === 'fill') align = 'left';
    if (align === 'centerContinuous') align = 'center';
    const valign = d?.alignV ?? '';
    const lineH = px * LINE_HEIGHT;

    ctx.font = font;
    ctx.fillStyle = cell.color ?? st?.color ?? (st?.fill ? contrastText(st.fill) : this.theme.cellText);
    ctx.textBaseline = 'alphabetic';

    let lines: string[];
    let clipL = left;
    let clipR = right;
    if (d?.wrap) {
      lines = this.text.wrap(font, cell.text, Math.max(1, cellW - pad * 2));
    } else {
      const text = cell.text.replace(/\n/g, ' ');
      const tw = this.text.width(font, text);
      if (tw > cellW - pad * 2) {
        if (cell.numeric) {
          // Numbers never overflow: Excel shows #### instead.
          const hashW = this.text.width(font, '#');
          lines = ['#'.repeat(Math.max(1, Math.floor((cellW - 2) / hashW)))];
          align = 'center';
        } else {
          lines = [text];
          if (overflow) {
            if (align === 'left' || align === 'center') clipR = overflow(1);
            if (align === 'right' || align === 'center') clipL = overflow(-1);
          }
        }
      } else {
        lines = [text];
      }
    }

    const total = lines.length * lineH;
    let y: number;
    if (valign === 'top') y = top + px * 0.98 + 1;
    else if (valign === 'center') y = top + (bottom - top - total) / 2 + px * 0.98;
    else y = bottom - 4 - (lines.length - 1) * lineH;

    const needsClip = lines.length > 1 || clipL !== left || clipR !== right || this.text.width(font, lines[0]) > cellW - pad * 2;
    if (needsClip) {
      ctx.save();
      ctx.beginPath();
      ctx.rect(clipL, top, clipR - clipL - 1, bottom - top - 1);
      ctx.clip();
    }
    for (const line of lines) {
      const tw = this.text.width(font, line);
      const x = align === 'right' ? right - pad - tw : align === 'center' ? left + (cellW - tw) / 2 : left + pad;
      ctx.fillText(line, x, y);
      if (d?.font.underline || d?.font.strike) {
        ctx.fillRect(x, d.font.underline ? y + 2 : y - px * 0.3, tw, Math.max(1, px / 14));
      }
      y += lineH;
    }
    if (needsClip) ctx.restore();
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
    const hw = metrics.headerW;

    ctx.fillStyle = th.headerBg;
    ctx.fillRect(0, 0, w, HEADER_H);
    ctx.fillRect(0, 0, hw, h);

    ctx.font = th.headerFont;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';

    const colStates = new Array<number>(vr.c1 - vr.c0 + 1);
    const rowStates = new Array<number>(vr.r1 - vr.r0 + 1);

    ctx.save();
    ctx.beginPath();
    ctx.rect(hw, 0, w - hw, HEADER_H);
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

    ctx.save();
    ctx.beginPath();
    ctx.rect(0, HEADER_H, hw, h - HEADER_H);
    ctx.clip();
    for (let r = vr.r0; r <= vr.r1; r++) {
      const top = ys[r - vr.r0];
      const bottom = ys[r - vr.r0 + 1];
      if (bottom === top) continue;
      const state = (rowStates[r - vr.r0] = this.headerState(sel, r, 'row'));
      if (state > 0) {
        ctx.fillStyle = state === 2 ? th.headerFullBg : th.headerActiveBg;
        ctx.fillRect(0, top, hw, bottom - top);
      }
      ctx.fillStyle = state > 0 ? th.headerActiveText : th.headerText;
      ctx.fillText(String(r + 1), hw / 2, (top + bottom) / 2);
    }
    ctx.restore();
    ctx.textAlign = 'start';

    ctx.beginPath();
    for (let i = 1; i < xs.length; i++) {
      if (xs[i] < hw) continue;
      const x = this.snap(xs[i] - 1);
      ctx.moveTo(x, 0);
      ctx.lineTo(x, HEADER_H);
    }
    for (let i = 1; i < ys.length; i++) {
      if (ys[i] < HEADER_H) continue;
      const y = this.snap(ys[i] - 1);
      ctx.moveTo(0, y);
      ctx.lineTo(hw, y);
    }
    const hy = this.snap(HEADER_H - 1);
    const hx = this.snap(hw - 1);
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
    ctx.rect(hw, 0, w - hw, HEADER_H);
    ctx.clip();
    for (let i = 0; i < colStates.length; i++) {
      if (colStates[i] > 0) ctx.fillRect(xs[i], HEADER_H - 2, xs[i + 1] - xs[i], 2);
    }
    ctx.restore();
    ctx.save();
    ctx.beginPath();
    ctx.rect(0, HEADER_H, hw, h - HEADER_H);
    ctx.clip();
    for (let i = 0; i < rowStates.length; i++) {
      if (rowStates[i] > 0) ctx.fillRect(hw - 2, ys[i], 2, ys[i + 1] - ys[i]);
    }
    ctx.restore();
  }
}

// Selection layer: redrawn independently of the grid layer.
export class SelectionRenderer extends Layer {
  private theme = readTheme();
  merges: Range[] = [];
  // Column or row being resized: [axis, pixel position] of the guide.
  guide: { vertical: boolean; at: number } | null = null;

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

  private mergeOf(row: number, col: number): Range {
    for (const m of this.merges) {
      if (row >= m.r0 && row <= m.r1 && col >= m.c0 && col <= m.c1) return m;
    }
    return { r0: row, c0: col, r1: row, c1: col };
  }

  draw(sx: number, sy: number, sel: Selection, highlights: readonly Highlight[] = []): void {
    const { ctx, width: w, height: h, theme: th } = this;
    const hw = metrics.headerW;
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);

    ctx.save();
    ctx.beginPath();
    ctx.rect(hw, HEADER_H, w - hw, h - HEADER_H);
    ctx.clip();

    const activeArea = this.mergeOf(sel.active.row, sel.active.col);
    const cur = current(sel);
    const single =
      isSingleCell(sel) ||
      (sel.ranges.length === 1 && cur.r0 === activeArea.r0 && cur.c0 === activeArea.c0 && cur.r1 === activeArea.r1 && cur.c1 === activeArea.c1);
    const rect = (r: Range) => clampRect(rangeRect(this.rows, this.cols, r, sx, sy), w, h);

    if (!single) {
      ctx.fillStyle = th.selectionFill;
      for (const r of sel.ranges) {
        const rc = rect(r);
        if (rc.w > 0 && rc.h > 0) ctx.fillRect(rc.x, rc.y, rc.w, rc.h);
      }
      // The active cell stays unfilled.
      const a = rect(activeArea);
      ctx.clearRect(a.x, a.y, a.w - 1, a.h - 1);
    }

    ctx.strokeStyle = th.accent;
    if (sel.ranges.length > 1) {
      const a = rect(activeArea);
      ctx.lineWidth = 1;
      ctx.strokeRect(a.x - 0.5, a.y - 0.5, a.w, a.h);
    } else {
      const rc = rect(single ? activeArea : cur);
      if (rc.w > 0 && rc.h > 0) {
        ctx.lineWidth = 2;
        ctx.strokeRect(rc.x - 1, rc.y - 1, rc.w, rc.h);
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

    if (this.guide) {
      ctx.fillStyle = th.accent;
      if (this.guide.vertical) ctx.fillRect(this.guide.at - 1, 0, 2, h);
      else ctx.fillRect(0, this.guide.at - 1, w, 2);
    }
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
