import type { FormatRowDTO, SizeDTO } from '../../bindings/ww-excel/internal/service/models';
import { fontCss } from '../stores/styles';
import { layoutRichText, styledRuns, type RichMeasure } from './richtext';
import { LINE_HEIGHT } from './text';

export function measureFormatRows(rows: FormatRowDTO[], measure: RichMeasure): SizeDTO[] {
  return rows.map(row => {
    let height = row.minimum;
    for (const cell of row.cells ?? []) {
      const runs = cell.runs?.length ? cell.runs : [{ text: cell.text, font: cell.style.font }];
      const lines = layoutRichText(styledRuns(runs, '#000000'), Math.max(1, cell.width - 6), !!cell.style.wrap || cell.text.includes('\n'), measure, fontCss(cell.style.font).px);
      height = Math.max(height, Math.ceil(lines.reduce((sum, line) => sum + line.px * LINE_HEIGHT, 0) + 5));
    }
    return { i: row.row, px: Math.min(546, height), hidden: false };
  });
}
