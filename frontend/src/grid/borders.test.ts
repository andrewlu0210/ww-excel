import { describe, expect, it } from 'vitest';
import { slantedBorderSegments } from './borders';

describe('slanted borders', () => {
  it('draws diagonal strokes within horizontal and vertical edge bounds', () => {
    expect(slantedBorderSegments(10, 20, 18, 20)).toEqual([[10, 21, 14, 19], [16, 21, 18, 19]]);
    expect(slantedBorderSegments(10, 20, 10, 28)).toEqual([[11, 20, 9, 24], [11, 26, 9, 28]]);
    expect(slantedBorderSegments(10, 20, 10, 20)).toEqual([]);
  });
});
