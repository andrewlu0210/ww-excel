// Exercises the toolbar and the same pointer-selection path as real painting.
import { documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

export async function formatPainterSelfTest(
  sheet: SheetDriver,
  step: () => Promise<void>,
  expect: (name: string, ok: boolean, detail: string) => void,
): Promise<void> {
  const button = () => document.querySelector<HTMLButtonElement>('.format-painter')!;
  const select = async (row: number, col: number) => {
    sheet.setSelectionForTest({ active: { row, col }, anchor: { row, col }, focus: { row, col }, ranges: [{ r0: row, c0: col, r1: row, c1: col }] });
    await step();
  };
  const style = async (row: number, col: number, index = 0) => {
    const blocks = await sheetApi.getBlocks(index, [{ row: Math.floor(row / 64), col: Math.floor(col / 32) }], 0);
    const cell = blocks.blocks?.[0]?.cells?.find((c) => c.r === row % 64 && c.c === col % 32);
    return blocks.styles?.find((s) => s.id === (cell?.st ?? 0));
  };
  const apply = async (row: number, col: number, lastRow = row, lastCol = col) => {
    sheet.clickCell({ row, col });
    if (lastRow !== row || lastCol !== col) sheet.clickCell({ row: lastRow, col: lastCol }, 'drag');
    sheet.clickCell({ row: lastRow, col: lastCol }, 'up');
    await step();
  };
  await sheetApi.setCell(0, 130, 0, '來源');
  await sheetApi.format(0, [{ r0: 130, c0: 0, r1: 130, c1: 0 }], { bold: true, fill: 'FFFF00', fontColor: 'C00000', numFmt: '0.00%', border: 'all', alignH: 'center', label: '格式設定' });
  await sheetApi.setCell(0, 130, 5, '=2+3');
  await step();
  await select(130, 0);
  const before = await documentApi.state();
  button().click();
  await step();
  expect('複製格式：按下刷子待命', button().getAttribute('aria-pressed') === 'true', '');
  const captured = await documentApi.state();
  expect('複製格式：擷取來源不新增復原紀錄', before.undoLabel === captured.undoLabel && before.dirty === captured.dirty, '');
  await apply(130, 5, 131, 6);
  const painted = await style(130, 5);
  const corner = await style(131, 6);
  expect('複製格式：拖曳套用完整格式', !!painted?.font.bold && painted.fill === 'FFFF00' && painted.font.color === 'C00000' && painted.numFmt === '0.00%' && painted.alignH === 'center' && !!corner?.font.bold && !!painted.left.style, JSON.stringify(painted));
  expect('複製格式：保留目標公式', await sheetApi.editText(0, 130, 5) === '=2+3', await sheetApi.editText(0, 130, 5));
  expect('複製格式：單次套用後結束', button().getAttribute('aria-pressed') === 'false', '');
  await sheet.runMenu('undo');
  await step();
  expect('複製格式：一次復原整個範圍', !(await style(130, 5))?.font.bold && !(await style(131, 6))?.font.bold, '');
  await sheet.runMenu('redo');
  await step();
  expect('複製格式：重做', !!(await style(131, 6))?.font.bold, '');

  await select(130, 0);
  button().click();
  await step();
  button().dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
  await step();
  await apply(134, 5);
  await apply(135, 6);
  expect('複製格式：雙擊可連續套用', button().getAttribute('aria-pressed') === 'true' && !!(await style(134, 5))?.font.bold && !!(await style(135, 6))?.font.bold, '');
  sheet.pressKey('Escape');
  await step();
  expect('複製格式：Escape 取消', button().getAttribute('aria-pressed') === 'false', '');

  await sheetApi.format(0, [{ r0: 130, c0: 1, r1: 130, c1: 1 }], { italic: true, fill: '00CCFF', label: '格式設定' });
  sheet.setSelectionForTest({ active: { row: 130, col: 0 }, anchor: { row: 130, col: 0 }, focus: { row: 130, col: 1 }, ranges: [{ r0: 130, c0: 0, r1: 130, c1: 1 }] });
  await step();
  button().click();
  await step();
  await apply(137, 5, 137, 8);
  expect('複製格式：多格來源重複圖樣', (await style(137, 5))?.fill === 'FFFF00' && (await style(137, 6))?.fill === '00CCFF' && (await style(137, 7))?.fill === 'FFFF00' && (await style(137, 8))?.fill === '00CCFF', '');

  const target = await documentApi.addSheet((await documentApi.state()).sheets?.length ?? 1);
  await step();
  await select(130, 0);
  button().click();
  await step();
  const tab = document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[target];
  tab.dispatchEvent(new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true }));
  window.dispatchEvent(new PointerEvent('pointerup'));
  await step();
  await apply(2, 2);
  expect('複製格式：跨工作表套用', !!(await style(2, 2, target))?.font.bold, '');
  document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[0].dispatchEvent(new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true }));
  window.dispatchEvent(new PointerEvent('pointerup'));
  await step();
}
