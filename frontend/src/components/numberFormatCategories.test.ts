import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { NUMBER_FORMAT_CATEGORIES, numberFormatCategory } from './numberFormatCategories';
import NumberFormatDialog from './NumberFormatDialog.svelte';

const props = { style: null, selection: [], merges: [], preview: async () => '', onapply: () => {}, onclose: () => {} };

describe('number format categories and types', () => {
  it('groups all date and time variants under one category each', () => {
    expect(NUMBER_FORMAT_CATEGORIES.filter(c => c.label === '日期')).toHaveLength(1);
    expect(NUMBER_FORMAT_CATEGORIES.filter(c => c.label === '時間')).toHaveLength(1);
    expect(NUMBER_FORMAT_CATEGORIES.map(c => c.label)).not.toContain('民國年');
    expect(NUMBER_FORMAT_CATEGORIES.map(c => c.label)).not.toContain('上午／下午');
    expect(NUMBER_FORMAT_CATEGORIES.find(c => c.id === 'date')?.types.map(t => t.code)).toContain('[$-404]gge"年"m"月"d"日"');
    expect(numberFormatCategory('上午/下午 h"時"mm"分"')).toBe('time');
  });
  it('recognizes existing date formats and leaves arbitrary codes custom', () => {
    for (const code of ['yyyy/m/d','yyyy-mm-dd','yyyy"年"m"月"d"日"','[$-404]e/m/d','[$-404]gge"年"m"月"d"日"']) expect(numberFormatCategory(code)).toBe('date');
    expect(numberFormatCategory('"custom" 00000')).toBe('custom');
    expect(numberFormatCategory('')).toBe('general');
  });
  it('renders categories separately from date type choices without duplicating the old labels', () => {
    const { body } = render(NumberFormatDialog, { props: { ...props, style: { id: 1, font: {name:'Arial',size:11}, top:{},bottom:{},left:{},right:{}, numFmt:'[$-404]e/m/d' } } });
    const categories=body.match(/<ul[^>]*aria-label="類別"[^>]*>([\s\S]*?)<\/ul>/)?.[1];
    expect(categories).toBeDefined();
    expect(categories).toContain('data-number-category="date"');
    expect(categories).not.toContain('民國年');expect(categories).not.toContain('日期（中文）');expect(categories).not.toContain('上午／下午');
    expect(body).toContain('aria-label="類型"');expect(body).toContain('101/3/14');expect(body).toContain('民國101年3月14日');
  });
  it('keeps mixed format values blank and does not show the active cell type as common', () => {
    const { body } = render(NumberFormatDialog, { props: {...props,mixed:['numFmt']} });
    expect(body).toContain('placeholder="混合"');
    expect(body).not.toContain('class="number-types"');
  });
});

it('offers the screenshot month/day, Chinese numeral, and weekday formats as date types', () => {
  const types=NUMBER_FORMAT_CATEGORIES.find(category=>category.id==='date')!.types;
  for (const [code,example] of [
    ['m"月"d"日"','3月14日'],
    ['[DBNum1][$-404]yyyy"年"m"月"d"日"','二○一二年三月十四日'],
    ['[DBNum1][$-404]m"月"d"日"','三月十四日'],
    ['[$-404]dddd','星期三'],
    ['[$-404]ddd','週三'],
  ]) {
    expect(types.find(type=>type.code===code)?.example).toBe(example);
    expect(numberFormatCategory(code)).toBe('date');
  }
});
