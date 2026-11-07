import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

// Cover logical merged cells and sparse axis formats through the real modal.
export async function formatAcceptanceSelfTest(sheet: SheetDriver, sheetIndex: number): Promise<string> {
  const lines = ['\n## 格式相容性桌面驗收', '', '| | 項目 |', '| --- | --- |'];
  let passed = 0, total = 0;
  const expect = (name: string, ok: boolean) => { total++; if (ok) passed++; lines.push(`| ${ok ? '✅' : '❌'} | ${name} |`); };
  const step = async () => { await sheet.whenIdle(); await new Promise(r => setTimeout(r, 120)); };
  const modal = () => document.querySelector<HTMLElement>('[role="dialog"][aria-label="儲存格格式"]')!;
  const button = (text: string) => Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.trim() === text)!;
  const select = async (r0: number, c0: number, r1 = r0, c1 = c0) => {
    sheet.setSelectionForTest({ active: { row: r0, col: c0 }, anchor: { row: r0, col: c0 }, focus: { row: r1, col: c1 }, ranges: [{ r0, c0, r1, c1 }] }); await step();
  };
  const raw = (row: number, col: number) => sheetApi.cellStyle(sheetIndex, row, col);
  const fill = async (color: string) => {
    sheet.focus(); sheet.pressKey('1', { metaKey: true }); await step();
    button('填滿').click(); await step();
    Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.includes('填滿色彩'))!.click(); await step();
    modal().querySelector<HTMLButtonElement>(`[aria-label="#${color}"]`)!.click(); await step();
    button('確定').click(); await step();
  };
  try {
    await sheet.goToName('J141'); await sheetApi.setCell(sheetIndex, 140, 9, 'merged acceptance');
    await select(140, 9, 140, 11); await sheet.mergeCells(false); await step();
    const before = (await raw(140, 9))?.fill ?? '';
    await select(140, 10); await fill('FFC000');
    expect('合併區內部單格開啟視窗後更新合併錨點', (await raw(140, 9))?.fill === 'FFC000');
    await sheet.runMenu('undo'); await step();
    expect('合併格式一次復原保留合併與內容', ((await raw(140, 9))?.fill ?? '') === before && (await sheetApi.layout(sheetIndex, 0)).merges?.some(m => m.r0 === 140 && m.c0 === 9) === true);
    await sheet.runMenu('redo'); await step();
    expect('合併格式一次重做', (await raw(140, 9))?.fill === 'FFC000');

    await sheetApi.format(sheetIndex, [{ r0: 0, c0: 14, r1: 1048575, c1: 14 }], { fontName: 'Arial', italic: true });
    await select(0, 14, 1048575, 14); await fill('FFFF00');
    expect('整欄格式套用至工作表最後一列', (await raw(1048575, 14))?.fill === 'FFFF00');
    expect('整欄填滿保留繼承字型', (await raw(900000, 14))?.font.name === 'Arial' && (await raw(900000, 14))?.font.italic === true);
    await sheet.runMenu('undo'); await step();
    expect('整欄格式一次復原', (await raw(1048575, 14))?.fill !== 'FFFF00');
    await sheet.runMenu('redo'); await step();

    await select(145, 0, 145, 16383); await fill('00B050');
    expect('整列格式套用至工作表最後一欄', (await raw(145, 16383))?.fill === '00B050');
    expect('列欄交點遵循列格式優先', (await raw(145, 14))?.fill === '00B050');
    for (const extension of ['xlsx', 'wwx']) {
      const path = await benchApi.saveCopy(`selftest-format-acceptance.${extension}`); await documentApi.openPath(path); await step();
      expect(`${extension} 重開保留合併、整列整欄與繼承`, (await raw(140, 9))?.fill === 'FFC000' && (await raw(1048575, 14))?.fill === 'FFFF00' && (await raw(145, 16383))?.fill === '00B050' && (await raw(900000, 14))?.font.italic === true);
    }
  } catch (error) {
    expect('驗收流程完成', false);
    lines.push(`| ❌ | ${String(error).replaceAll('|', '/').replaceAll('\n', ' ')} |`);
  }
  lines.splice(2, 0, `- 結果：${passed} / ${total} 通過`, '');
  return lines.join('\n') + '\n';
}
