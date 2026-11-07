import { describe, expect, it } from 'vitest';
import { measureFormatRows } from './formatRows';
import type { FormatRowDTO } from '../../bindings/ww-excel/internal/service/models';
const style = (size: number, wrap = false) => ({ id: 0, font: { name: 'Arial', size }, wrap, top: {}, bottom: {}, left: {}, right: {} });
describe('atomic format row measurement', () => {
  it('uses projected fonts, wrapping and the largest unselected cell', () => {
    const rows: FormatRowDTO[] = [{ row: 2, minimum: 20, cells: [{ text: 'ABCD', width: 25, style: style(30, true) }, { text: 'other', width: 100, style: style(50) }] }];
    const result = measureFormatRows(rows, (_, text) => text.length * 10);
    expect(result[0].i).toBe(2);
    expect(result[0].px).toBeGreaterThan(70);
  });
  it('keeps large rich text and clamps excessive wrapping', () => {
    const rows: FormatRowDTO[] = [{ row: 3, minimum: 20, cells: [{ text: 'big', width: 100, style: style(11), runs: [{ text: 'big', font: { name: 'Arial', size: 70 } }] }] }, { row: 4, minimum: 20, cells: [{ text: 'A'.repeat(200), width: 8, style: style(30, true) }] }];
    const result = measureFormatRows(rows, (_, text) => text.length * 10);
    expect(result[0].px).toBeGreaterThan(90);
    expect(result[1].px).toBe(546);
  });
});
