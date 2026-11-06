// Exercise the original overflow formula through the real editor and bindings.
import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

export async function formulaSecuritySelfTest(
  sheet: SheetDriver,
  step: () => Promise<void>,
  expect: (name: string, ok: boolean, detail: string) => void,
): Promise<void> {
  const row = 500, col = 25;
  const info = await sheetApi.info(0);
  const value = async (r: number) => {
    const result = await sheetApi.getBlocks(0, [{ row: Math.floor(r / info.blockRows), col: Math.floor(col / info.blockCols) }], 0);
    // Cell coordinates within a block are local to that block.
    return result.blocks?.[0]?.cells?.find(cell => cell.r === r % info.blockRows && cell.c === col % info.blockCols)?.v ?? '';
  };
  const enter = async (address: string, text: string) => {
    await sheet.goToName(address); await step();
    sheet.typeText(text); await step();
    sheet.pressKey('Enter'); await step();
  };
  const before = await value(row);
  await enter('Z501', '=REPT(REPT("a",16384)&REPT("a",16384),562949953421312)');
  expect('公式安全：REPT 溢位回傳錯誤', await value(row) === '#VALUE!', await value(row));
  expect('公式安全：前端收到錯誤結果', sheet.getGrid()?.cache.cell(row, col)?.text === '#VALUE!', JSON.stringify(sheet.getGrid()?.cache.cell(row, col)));
  const state = await documentApi.state();
  expect('公式安全：錯誤後可查詢文件狀態', state.dirty && state.canUndo, JSON.stringify(state));
  await sheet.runMenu('undo'); await step();
  expect('公式安全：錯誤公式可復原', await value(row) === before, await value(row));
  await sheet.runMenu('redo'); await step();
  expect('公式安全：錯誤公式可重做', await value(row) === '#VALUE!', await value(row));
  await enter('Z502', '=1+2');
  expect('公式安全：錯誤後可繼續編輯', await value(row + 1) === '3', await value(row + 1));
  for (const ext of ['xlsx', 'wwx']) {
    const path = await benchApi.saveCopy(`selftest-formula-errors.${ext}`);
    await documentApi.openPath(path); await step(); await step();
    await sheet.goToName('Z501'); await step();
    expect(`公式安全：${ext} 儲存重開保留錯誤與其他資料`, await value(row) === '#VALUE!' && await value(row + 1) === '3', `${await value(row)} / ${await value(row + 1)}`);
    expect(`公式安全：${ext} 重開後文件狀態正常`, !(await documentApi.state()).dirty && sheet.getGrid()?.cache.cell(row, col)?.text === '#VALUE!', JSON.stringify(await documentApi.state()));
  }
}
