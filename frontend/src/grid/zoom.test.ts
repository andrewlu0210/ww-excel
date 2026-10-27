import { describe, expect, it } from 'vitest';
import { fitZoom, normalizeZoom, parseZoom, SheetZooms, ZoomTransform } from './zoom';
import { AxisLayout } from './layout';
import { MAX_SPACER, ScrollAxis } from './scroll';
import { frozenRect } from './renderer';

describe('zoom input', () => {
  it.each([[0, 10], [401, 400], [69.7, 70], [NaN, 100], [Infinity, 100]])('normalizes %s to %s', (value, want) => {
    expect(normalizeZoom(value)).toBe(want);
  });
  it.each([['70%', 70], [' 200 % ', 200], ['0', 10], ['-20', 10], ['.5', 10], ['500', 400], ['99.6', 100], ['', 125], ['abc', 125], ['1e2', 125], ['70%%', 125]])('parses %s as %i', (value, want) => {
    expect(parseZoom(value, 125)).toBe(want);
  });
});

describe.each([10, 70, 100, 200, 400])('coordinates at %i%%', (percent) => {
  const zoom = new ZoomTransform(percent);
  it('round trips a merged range beside frozen panes and hidden cells', () => {
    const rows = new AxisLayout(1_048_576, 20);
    const cols = new AxisLayout(16_384, 64);
    rows.setSize(3, 0);
    cols.setSize(3, 0);
    const range = { r0: 8, r1: 10, c0: 8, c1: 10 };
    const logical = frozenRect(rows, cols, range, { rows: 2, cols: 2 }, 100, 40);
    const screen = zoom.rect(logical);
    for (const field of ['x', 'y', 'w', 'h'] as const) expect(zoom.toLogical(screen[field])).toBeCloseTo(logical[field]);
    expect(cols.indexAt(zoom.toLogical(screen.x) - 40 + 100)).toBe(8);
    expect(rows.indexAt(zoom.toLogical(screen.y) - 22 + 40)).toBe(8);
  });
  it('reaches the last row despite the native scroll spacer limit', () => {
    const axis = new ScrollAxis();
    const viewport = 800;
    const logicalTotal = 22 + 1_048_576 * 20;
    axis.update(zoom.toScreen(logicalTotal), viewport);
    expect(axis.spacer).toBeLessThanOrEqual(MAX_SPACER);
    const last = logicalTotal - zoom.toLogical(viewport);
    expect(zoom.toLogical(axis.toContent(axis.spacer - viewport))).toBeCloseTo(last);
    expect(axis.toScroll(zoom.toScreen(last))).toBeCloseTo(axis.spacer - viewport);
  });
});

describe('per-document sheet zoom', () => {
  it('keeps renamed and reordered sheets independent and clears deleted sheets', () => {
    const zooms = new SheetZooms();
    zooms.set('A', 70);
    zooms.set('B', 200);
    zooms.reconcile(['A', 'B'], ['A', 'C']);
    expect(zooms.get('C')).toBe(200);
    zooms.reconcile(['A', 'C'], ['C', 'A']);
    expect(zooms.get('A')).toBe(70);
    expect(zooms.get('C')).toBe(200);
    zooms.reconcile(['C', 'A'], ['A']);
    expect(zooms.get('C')).toBe(100);
    expect(zooms.get('New')).toBe(100);
    zooms.clear();
    expect(zooms.get('A')).toBe(100);
  });
  it('migrates the zoom when a rename is undone', () => {
    const zooms = new SheetZooms();
    zooms.set('A', 70);
    zooms.reconcile(['A'], ['B']);
    zooms.reconcile(['B'], ['A']);
    expect(zooms.get('A')).toBe(70);
  });
});

describe('fit selection', () => {
  it('uses the limiting dimension and rounds down so borders fit', () => {
    expect(fitZoom(1004, 504, 800, 1000)).toBe(50);
    expect(fitZoom(1004, 504, 2000, 100)).toBe(50);
    expect(fitZoom(1000, 1000, 800, 800)).toBe(124);
  });
  it('limits single-cell and whole-sheet selections to the allowed zoom range', () => {
    expect(fitZoom(1200, 800, 100, 42)).toBe(400);
    expect(fitZoom(1200, 800, 1_048_576, 16_384)).toBe(10);
    expect(fitZoom(1200, 800, 0, 0)).toBe(100);
  });
});
