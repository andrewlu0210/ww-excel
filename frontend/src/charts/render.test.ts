import { describe, expect, it } from 'vitest';
import { formatTick, niceScale } from './render';

describe('niceScale', () => {
  it('rounds bounds to a 1-2-2.5-5 step and starts at zero', () => {
    expect(niceScale(70, 150)).toEqual({ min: 0, max: 150, step: 50 });
    expect(niceScale(0, 1)).toEqual({ min: 0, max: 1, step: 0.2 });
    expect(niceScale(-30, 80)).toEqual({ min: -50, max: 100, step: 25 });
    expect(niceScale(5, 5)).toEqual({ min: 0, max: 5, step: 1 });
  });
  it('keeps a raised baseline for clustered values', () => {
    const s = niceScale(1000, 1100);
    expect(s.min).toBe(1000);
    expect(s.max).toBe(1100);
  });
});

describe('formatTick', () => {
  it('groups thousands and keeps the step precision', () => {
    expect(formatTick(12500, 2500)).toBe('12,500');
    expect(formatTick(0.4, 0.2)).toBe('0.4');
  });
});
