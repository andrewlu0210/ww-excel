import { describe, expect, it } from 'vitest';
import { callAt, canPoint, findRefs, refText, toggleAbsolute, wordAt } from './refs';

describe('findRefs', () => {
  it('finds cells, ranges, whole columns/rows and sheet prefixes', () => {
    const text = `=SUM(A1:B3)+$C$5*Sheet2!D1+'My Sheet'!E2+F:G+3:4+"A9"+LOG10(H1)`;
    const refs = findRefs(text).map((r) => [text.slice(r.start, r.end), r.sheet, r.kind]);
    expect(refs).toEqual([
      ['A1:B3', null, 'cell'],
      ['$C$5', null, 'cell'],
      ['Sheet2!D1', 'Sheet2', 'cell'],
      ["'My Sheet'!E2", 'My Sheet', 'cell'],
      ['F:G', null, 'col'],
      ['3:4', null, 'row'],
      ['H1', null, 'cell'],
    ]);
    expect(findRefs('=B3:A1')[0].range).toEqual({ r0: 0, c0: 0, r1: 2, c1: 1 });
  });

  it('ignores non-formulas and names that look like refs', () => {
    expect(findRefs('A1+B2')).toEqual([]);
    expect(findRefs('=LOG10(1)+ABC')).toEqual([]);
    expect(findRefs('=XFE1+A0')).toEqual([]);
  });
});

describe('point mode helpers', () => {
  it('detects where a reference can be inserted', () => {
    expect(canPoint('=', 1)).toBe(true);
    expect(canPoint('=SUM(', 5)).toBe(true);
    expect(canPoint('=A1+ ', 5)).toBe(true);
    expect(canPoint('=A1', 3)).toBe(false);
    expect(canPoint('="a+', 4)).toBe(false);
    expect(canPoint('hello+', 6)).toBe(false);
  });

  it('formats pointed references', () => {
    expect(refText({ row: 0, col: 0 }, { row: 0, col: 0 })).toBe('A1');
    expect(refText({ row: 4, col: 2 }, { row: 1, col: 0 })).toBe('A2:C5');
  });
});

describe('callAt / wordAt', () => {
  it('finds the call and argument under the caret', () => {
    expect(callAt('=SUM(1,2', 8)).toEqual({ name: 'SUM', arg: 1 });
    expect(callAt('=IF(A1>0,ROUND(B1,', 18)).toEqual({ name: 'ROUND', arg: 1 });
    expect(callAt('=IF(A1>0,ROUND(B1,2),', 21)).toEqual({ name: 'IF', arg: 2 });
    expect(callAt('=SUM(1)', 7)).toBeNull();
    expect(callAt('=IF((1+2),', 10)).toEqual({ name: 'IF', arg: 1 });
    expect(callAt('=LEN("a,b"', 10)).toEqual({ name: 'LEN', arg: 0 });
  });

  it('finds a function name being typed', () => {
    expect(wordAt('=su', 3)).toEqual({ start: 1, word: 'su' });
    expect(wordAt('=A1+VLO', 7)).toEqual({ start: 4, word: 'VLO' });
    // Any word is offered; filtering by function names happens later.
    expect(wordAt('=A1', 3)).toEqual({ start: 1, word: 'A1' });
    expect(wordAt('="ab', 4)).toBeNull(); // inside a string
    expect(wordAt('=A1+B', 4)).toBeNull(); // caret not at the end of a word
    expect(wordAt('abc', 3)).toBeNull();
  });
});

describe('toggleAbsolute', () => {
  it('cycles A1 -> $A$1 -> A$1 -> $A1 -> A1', () => {
    let s = { text: '=A1+1', caret: 3 };
    const seen: string[] = [];
    for (let i = 0; i < 4; i++) {
      s = toggleAbsolute(s.text, s.caret)!;
      seen.push(s.text);
    }
    expect(seen).toEqual(['=$A$1+1', '=A$1+1', '=$A1+1', '=A1+1']);
  });

  it('toggles both corners of a range and keeps the sheet', () => {
    expect(toggleAbsolute('=SUM(Sheet2!A1:B2)', 17)).toEqual({ text: '=SUM(Sheet2!$A$1:$B$2)', caret: 21 });
    expect(toggleAbsolute('=1+2', 4)).toBeNull();
  });
});
