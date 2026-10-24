import { describe, expect, it } from 'vitest';
import { wrapText } from './text';

// Every character is 10px wide.
const measure = (t: string) => [...t].length * 10;

describe('wrapText', () => {
  it('breaks Latin text at spaces', () => {
    expect(wrapText('hello big world', 90, measure)).toEqual(['hello big', 'world']);
  });

  it('breaks CJK text anywhere', () => {
    expect(wrapText('台北市信義區', 40, measure)).toEqual(['台北市信', '義區']);
  });

  it('keeps explicit line breaks and long words', () => {
    expect(wrapText('a\n\nbcdefghijk', 50, measure)).toEqual(['a', '', 'bcdef', 'ghijk']);
  });

  it('mixes CJK and Latin', () => {
    expect(wrapText('總計 Total amount', 80, measure)).toEqual(['總計 Total', 'amount']);
  });
});
