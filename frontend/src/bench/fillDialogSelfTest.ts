import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

export async function fillDialogSelfTest(sheet: SheetDriver, sheetIndex: number): Promise<string> {
  const lines = ['\n## 單色填滿編輯', '', '| | 項目 |', '| --- | --- |'];
  let passed = 0, total = 0;
  const expect = (name: string, ok: boolean) => { total++; if (ok) passed++; lines.push(`| ${ok ? '✅' : '❌'} | ${name} |`); };
  const step = async () => { await sheet.whenIdle(); await new Promise(r => setTimeout(r, 120)); };
  const modal = () => document.querySelector<HTMLElement>('[role="dialog"][aria-label="儲存格格式"]')!;
  const button = (text: string) => Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.trim() === text)!;
  const open = async () => { sheet.focus(); sheet.pressKey('1', { metaKey: true }); await step(); button('填滿').click(); await step(); };
  const preview = () => modal().querySelector<HTMLElement>('.fill-preview .sample')!;
  const raw = async (r = 64, c = 1) => {
    const style = await sheetApi.cellStyle(sheetIndex, r, c);
    return style ? { ...style, fill: style.fill ?? '' } : null;
  };
  const displayed = async (r: number, c: number) => {
    const data = await sheetApi.getBlocks(sheetIndex, [{ row: Math.floor(r / 64), col: Math.floor(c / 32) }], 0);
    const cell = data.blocks?.[0]?.cells?.find(cell => cell.r === r % 64 && cell.c === c % 32);
    return data.styles?.find(st => st.id === (cell?.st ?? 0))?.fill ?? '';
  };
  const pick = async (color: string) => {
    Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.includes('填滿色彩'))!.click(); await step();
    modal().querySelector<HTMLButtonElement>(`[aria-label="#${color}"]`)!.click(); await step();
  };
  await sheet.goToName('B65');
  sheet.setSelectionForTest({ active: { row: 64, col: 1 }, anchor: { row: 64, col: 1 }, focus: { row: 65, col: 3 }, ranges: [{ r0: 64, c0: 1, r1: 65, c1: 3 }] });
  await sheet.applyFormat({ fill: '00FF00', bold: true, numFmt: '0.0' }); await step();
  await open(); await pick('FFFFFF');
  expect('白色填滿預覽不透明', getComputedStyle(preview()).backgroundColor === 'rgb(255, 255, 255)' && modal().querySelector('.fill-preview')?.textContent?.trim() === '#FFFFFF');
  expect('選色只修改草稿', (await raw())?.fill === '00FF00');
  button('字型').click(); await step(); button('填滿').click(); await step();
  expect('跨分頁保留填滿草稿', getComputedStyle(preview()).backgroundColor === 'rgb(255, 255, 255)');
  button('取消').click(); await step();
  expect('取消白色草稿保留原填色', (await raw())?.fill === '00FF00');
  await open(); await pick('FFFFFF'); button('確定').click(); await step();
  expect('白色填滿正確套用到多格', (await raw())?.fill === 'FFFFFF' && (await raw(65, 3))?.fill === 'FFFFFF');
  await sheet.runMenu('undo'); await step();
  expect('填滿一次復原', (await raw())?.fill === '00FF00');
  await sheet.runMenu('redo'); await step();
  expect('填滿一次重做', (await raw())?.fill === 'FFFFFF');
  await open(); button('無填滿').click(); await step();
  expect('無填滿預覽透明且有明確標示', getComputedStyle(preview()).backgroundColor === 'rgba(0, 0, 0, 0)' && modal().querySelector('.fill-preview')?.textContent?.trim() === '無填滿');
  (document.activeElement ?? modal()).dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); await step();
  expect('Esc 不套用無填滿草稿', !modal() && (await raw())?.fill === 'FFFFFF');
  await open(); button('無填滿').click(); await step(); button('確定').click(); await step();
  expect('移除填滿保留字型與數值格式', (await raw())?.fill === '' && (await raw())?.font.bold === true && (await raw())?.numFmt === '0.0');
  await sheetApi.format(sheetIndex, [{ r0: 64, c0: 2, r1: 64, c1: 2 }], { fill: 'FFFFFF' });

  for (const [r, c, text] of [[69, 7, '項目'], [69, 8, '金額'], [70, 7, 'A'], [70, 8, '12'], [71, 7, 'B'], [71, 8, '34']] as const) await sheetApi.setCell(sheetIndex, r, c, text);
  const created = await sheetApi.createTable(sheetIndex, { r0: 69, c0: 7, r1: 71, c1: 8 }, true, 'TableStyleMedium2');
  if (created.error) throw new Error(created.error);
  await sheet.goToName('H71');
  await sheet.applyFormat({ fill: '00FF00' }); await step();
  await sheetApi.addCondFormat(sheetIndex, [{ r0: 70, c0: 7, r1: 70, c1: 7 }], { ranges: [], type: 'formula', op: '', value1: 'TRUE', value2: '', percent: false, below: false, color: '', fill: 'FFFFFF', bold: false, italic: false, colors: [], stopIfTrue: false }); await step();
  expect('條件格式覆蓋顯示並保留直接填色', await displayed(70, 7) === 'FFFFFF' && (await raw(70, 7))?.fill === '00FF00');
  await open();
  expect('格式視窗讀取底層填色', getComputedStyle(preview()).backgroundColor === 'rgb(0, 255, 0)');
  await pick('FFFFFF'); button('確定').click(); await step();
  expect('可套用與顯示色相同的白色直接填滿', (await raw(70, 7))?.fill === 'FFFFFF');
  await sheetApi.clearCondFormats(sheetIndex, [{ r0: 70, c0: 7, r1: 70, c1: 7 }]); await step();
  expect('直接白色優先於表格交錯列色', await displayed(70, 7) === 'FFFFFF');
  await open(); button('無填滿').click(); await step(); button('確定').click(); await step();
  expect('清除直接填色後恢復表格列色', (await raw(70, 7))?.fill === '' && await displayed(70, 7) === 'D9E1F2');
  await sheetApi.setCell(sheetIndex, 64, 1, '無填滿');
  await sheetApi.setCell(sheetIndex, 64, 2, '白色');
  await sheetApi.setCell(sheetIndex, 64, 3, '單色');
  await sheetApi.format(sheetIndex, [{ r0: 64, c0: 3, r1: 64, c1: 3 }], { fill: 'FFFF00' });
  const setup = await sheetApi.pageSetup(sheetIndex);
  await sheetApi.setPageSetup(sheetIndex, { ...setup, printArea: { r0: 64, c0: 1, r1: 72, c1: 8 }, fitWide: 1, fitTall: 1, headerText: '', footerText: '' });
  const pdf = await benchApi.savePDF('selftest-fill.pdf', sheetIndex, {});
  expect('可輸出實際填滿 PDF', pdf.endsWith('selftest-fill.pdf'));
  for (const extension of ['xlsx', 'wwx']) {
    const path = await benchApi.saveCopy(`selftest-fill.${extension}`);
    await documentApi.openPath(path); await step();
    expect(`${extension} 重開區分無填滿與白色`, (await raw())?.fill === '' && (await raw(64, 2))?.fill === 'FFFFFF' && (await raw(64, 3))?.fill === 'FFFF00');
    expect(`${extension} 重開保留表格與直接格式優先順序`, (await raw(70, 7))?.fill === '' && await displayed(70, 7) === 'D9E1F2');
  }
  lines.splice(2, 0, `- 結果：${passed} / ${total} 通過`, '');
  return lines.join('\n') + '\n';
}
