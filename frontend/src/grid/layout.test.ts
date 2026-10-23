import { describe, expect, it } from 'vitest';
import { AxisLayout, cellAddress, colName } from './layout';
import { MAX_SPACER, ScrollAxis } from './scroll';

describe('colName', () => {
  it.each([
    [0, 'A'],
    [25, 'Z'],
    [26, 'AA'],
    [51, 'AZ'],
    [52, 'BA'],
    [701, 'ZZ'],
    [702, 'AAA'],
    [16383, 'XFD'],
  ])('%i -> %s', (i, want) => {
    expect(colName(i)).toBe(want);
  });

  it('formats addresses', () => {
    expect(cellAddress(0, 0)).toBe('A1');
    expect(cellAddress(1_048_575, 16_383)).toBe('XFD1048576');
  });
});

describe('AxisLayout', () => {
  it('uses default sizes', () => {
    const a = new AxisLayout(1_048_576, 20);
    expect(a.offset(10)).toBe(200);
    expect(a.total).toBe(20_971_520);
    expect(a.indexAt(0)).toBe(0);
    expect(a.indexAt(19.9)).toBe(0);
    expect(a.indexAt(20)).toBe(1);
    expect(a.indexAt(1e12)).toBe(1_048_575);
  });

  it('tracks custom sizes', () => {
    const a = new AxisLayout(100, 20);
    a.setSize(2, 50);
    expect(a.offset(3)).toBe(90);
    expect(a.indexAt(60)).toBe(2);
    expect(a.indexAt(90)).toBe(3);
    a.setSize(2, 20);
    expect(a.offset(3)).toBe(60);
    expect(a.size(2)).toBe(20);
  });

  it('skips hidden entries', () => {
    const a = new AxisLayout(10, 10);
    a.setSize(1, 0);
    a.setSize(2, 0);
    expect(a.offset(3)).toBe(10);
    expect(a.indexAt(10)).toBe(3);
    a.setSize(0, 0);
    expect(a.indexAt(0)).toBe(3);
  });
});

describe('ScrollAxis', () => {
  it('is 1:1 below the cap', () => {
    const s = new ScrollAxis();
    s.update(20_971_520, 800);
    expect(s.spacer).toBe(20_971_520);
    expect(s.toContent(1000)).toBe(1000);
  });

  it('scales above the cap and maps the end exactly', () => {
    const s = new ScrollAxis();
    const content = MAX_SPACER * 2;
    s.update(content, 800);
    expect(s.spacer).toBe(MAX_SPACER);
    expect(s.toContent(MAX_SPACER - 800)).toBeCloseTo(content - 800);
    expect(s.toScroll(s.toContent(12345))).toBeCloseTo(12345);
  });
});
