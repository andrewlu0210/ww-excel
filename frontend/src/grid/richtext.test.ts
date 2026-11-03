import { describe, expect, it } from 'vitest';
import { layoutRichText, type StyledRun } from './richtext';

const run = (text: string, color = '#000000', px = 10): StyledRun => ({ text, color, font: `${px}px test`, px });
const measure = (_font: string, text: string) => [...text].length * 10;
const text = (lines: ReturnType<typeof layoutRichText>) => lines.map((l) => l.runs.map((r) => r.text).join(''));

describe('rich text layout', () => {
  it('preserves blank lines and blue run colors through CJK wrapping', () => {
    const lines = layoutRichText([run('關鍵系統\n\n'), run('指標庫：系統保障', '#00B0F0')], 50, true, measure, 10);
    expect(text(lines)).toEqual(['關鍵系統', '', '指標庫：系', '統保障']);
    expect(lines[2].runs[0].color).toBe('#00B0F0');
    expect(lines[3].runs[0].color).toBe('#00B0F0');
  });
  it('keeps different colors within one line', () => {
    const lines = layoutRichText([run('黑字'), run('藍字', '#00B0F0')], 100, true, measure, 10);
    expect(lines[0].runs.map((r) => r.color)).toEqual(['#000000', '#00B0F0']);
    expect(lines[0].width).toBe(40);
  });
  it('wraps Latin words and retains indentation when it fits', () => {
    expect(text(layoutRichText([run('hello big world')], 90, true, measure, 10))).toEqual(['hello big', 'world']);
    expect(text(layoutRichText([run('   標題')], 100, true, measure, 10))).toEqual(['   標題']);
  });
  it('uses the largest font on each line and flattens unwrapped breaks', () => {
    const lines = layoutRichText([run('a', '#000', 20), run('\nb')], 100, false, measure, 10);
    expect(text(lines)).toEqual(['a b']);
    expect(lines[0].px).toBe(20);
  });
});

it('uses each shape paragraph font and the end font for blank paragraphs', () => {
  const lines = layoutRichText([
    run('Heading','#fff',22),run('\n','#fff',14),run('\n','#fff',24),
    run('Body text','#fff',18),run('\n\n','#fff',18),run('Link','#fff',22),
  ],300,true,measure,0);
  expect(text(lines)).toEqual(['Heading','','Body text','','Link']);
  expect(lines.map(l=>l.px)).toEqual([22,24,18,18,22]);
  // Cell layout keeps its minimum font size.
  expect(layoutRichText([run('body','#fff',10)],100,true,measure,20)[0].px).toBe(20);
});

it('retains the font of a trailing empty paragraph', () => {
  const lines=layoutRichText([run('body','#fff',18),run('\n','#fff',18),run('','#fff',12)],100,true,measure,0);
  expect(lines.map(l=>l.px)).toEqual([18,12]);
});

it('keeps closing CJK punctuation off the start of shape lines', () => {
  const runs=[run('使用者皆適用。依照步調學習。')];
  expect(text(layoutRichText(runs,60,true,measure,0,{hangingPunctuation:true}))).toEqual(['使用者皆適用。','依照步調學習。']);
  expect(text(layoutRichText(runs,60,true,measure,0))).toEqual(['使用者皆適用','。依照步調學','習。']);
});

it('does not insert an extra line when a paragraph ends with an overflowing space', () => {
  expect(text(layoutRichText([run('abcdef '),run('\n\nLink')],60,true,measure,0))).toEqual(['abcdef ','','Link']);
  expect(text(layoutRichText([run('hello big world')],90,true,measure,0))).toEqual(['hello big','world']);
});
