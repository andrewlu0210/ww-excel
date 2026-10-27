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
