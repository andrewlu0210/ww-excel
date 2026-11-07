import { benchApi, documentApi, sheetApi, type FormatPatch } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

export async function mixedFormatSelfTest(sheet: SheetDriver, sheetIndex: number): Promise<string> {
  const lines = ['\n## 多選混合格式與一次復原', '', '| | 項目 |', '| --- | --- |'];
  let passed = 0, total = 0;
  const expect = (name: string, ok: boolean) => { total++; if (ok) passed++; lines.push(`| ${ok ? '✅' : '❌'} | ${name} |`); };
  const step = async () => { await sheet.whenIdle(); await new Promise(r => setTimeout(r, 140)); };
  const modal = () => document.querySelector<HTMLElement>('[role="dialog"][aria-label="儲存格格式"]')!;
  const button = (text: string) => Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.trim() === text)!;
  const tab = async (name: string) => { button(name).click(); await step(); };
  const open = async () => { sheet.focus(); sheet.pressKey('1', { metaKey: true }); await step(); };
  const raw = async (row: number, col: number) => { const style = await sheetApi.cellStyle(sheetIndex, row, col); return style ? { ...style, fill: style.fill ?? '', numFmt: style.numFmt ?? '' } : null; };
  const version = async () => (await sheetApi.getBlocks(sheetIndex, [], 0)).version;
  const height = async (row: number) => { const l = await sheetApi.layout(sheetIndex, 0); return l.rows?.find(r => r.i === row)?.px ?? l.defaultRow; };
  const select = async (row: number, col: number, endRow = row, endCol = col) => {
    await sheet.goToName(`${String.fromCharCode(65 + col)}${row + 1}`);
    sheet.setSelectionForTest({ active: { row, col }, anchor: { row, col }, focus: { row: endRow, col: endCol }, ranges: [{ r0: row, c0: col, r1: endRow, c1: endCol }] }); await step();
  };
  const set = async (row: number, col: number, patch: FormatPatch) => { const error = await sheetApi.format(sheetIndex, [{ r0: row, c0: col, r1: row, c1: col }], patch); if (error) throw new Error(error); };
  const pickFill = async (color: string) => {
    await tab('填滿');
    Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.includes('填滿色彩'))!.click(); await step();
    const swatch = modal().querySelector<HTMLButtonElement>(`[aria-label="#${color}"]`);
    if (!swatch) throw new Error(`Missing palette swatch #${color}`);
    swatch.click(); await step();
  };
  const inputSize = () => modal().querySelector<HTMLInputElement>('input[type="number"]')!;
  const setSize = async (value: number) => { inputSize().value = String(value); inputSize().dispatchEvent(new Event('input', { bubbles: true })); await step(); };

  try {
  await sheetApi.setCell(sheetIndex, 119, 1, 'first'); await sheetApi.setCell(sheetIndex, 119, 2, 'second');
  await set(119, 1, { fontName: 'Arial', fontSize: 11, bold: true, fill: 'FF0000', numFmt: '0.00', borders: { top: { style: 'double', color: 'FF0000' } } });
  await set(119, 2, { fontName: 'Calibri', fontSize: 14, bold: false, fill: '0000FF', numFmt: '0%', borders: { top: { style: 'dotted', color: '0000FF' } } });
  await select(119, 1, 119, 2);
  const originalA = await raw(119, 1), originalB = await raw(119, 2);
  const initialVersion = await version();
  await open();
  expect('數值格式混合時不顯示作用中儲存格的格式碼', modal().querySelector<HTMLInputElement>('input')?.value === '' && modal().querySelector<HTMLInputElement>('input')?.placeholder === '混合');
  await tab('字型');
  expect('混合字型與字級顯示混合狀態', modal().querySelector<HTMLSelectElement>('select')?.value === '__mixed__' && inputSize().value === '' && inputSize().placeholder === '混合');
  expect('混合粗體顯示中間狀態', modal().querySelector<HTMLInputElement>('input[type="checkbox"]')?.indeterminate === true);
  await tab('框線');
  expect('混合框線保留未修改狀態', modal().querySelector('[data-edge="top"] span')?.textContent === '混合');
  await tab('填滿');
  expect('混合填滿不顯示單格代表色', modal().querySelector('.fill-preview .caption')?.textContent === '混合');
  button('確定').click(); await step();
  expect('未修改確定不增加修改或復原紀錄', await version() === initialVersion);

  await open(); await pickFill('FFFF00'); button('重設變更').click(); await step();
  expect('重設草稿恢復混合填滿', modal().querySelector('.fill-preview .caption')?.textContent === '混合');
  button('確定').click(); await step();
  expect('重設後確定不修改文件', await version() === initialVersion);
  await open(); await pickFill('FFFF00'); button('確定').click(); await step();
  const a = await raw(119, 1), b = await raw(119, 2);
  expect('僅改填滿保留混合字型與數值格式', a?.fill === 'FFFF00' && b?.fill === 'FFFF00' && a?.font.name === originalA?.font.name && b?.font.size === originalB?.font.size && a?.numFmt === originalA?.numFmt && b?.numFmt === originalB?.numFmt);
  expect('僅改填滿保留各格原框線', JSON.stringify(a?.top) === JSON.stringify(originalA?.top) && JSON.stringify(b?.top) === JSON.stringify(originalB?.top));
  await sheet.runMenu('undo'); await step();
  expect('填滿一次復原保留原先混合值', (await raw(119, 1))?.fill === 'FF0000' && (await raw(119, 2))?.fill === '0000FF');
  await sheet.runMenu('redo'); await step();
  const sameVersion = await version();
  await open(); await pickFill('FFFFFF'); await pickFill('FFFF00'); button('確定').click(); await step();
  expect('共同值修改後改回原值不產生紀錄', await version() === sameVersion);

  await set(119, 1, { fill: '' }); await step();
  await open(); await tab('填滿'); button('無填滿').click(); await step(); button('確定').click(); await step();
  expect('作用中儲存格已無填滿仍可清除其他格填色', (await raw(119, 1))?.fill === '' && (await raw(119, 2))?.fill === '');
  const beforeCombinedA = await raw(119, 1), beforeCombinedB = await raw(119, 2), beforeHeight = await height(119);
  await open(); await tab('字型');
  const fonts = modal().querySelector<HTMLSelectElement>('select')!; fonts.value = 'Arial'; fonts.dispatchEvent(new Event('change', { bubbles: true }));
  await setSize(36);
  modal().querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); await step();
  await tab('對齊'); modal().querySelector<HTMLInputElement>('input[type="checkbox"]')!.click(); await step();
  await pickFill('00B050'); button('確定').click(); await step();
  expect('跨分頁同時套用字型字級粗體換行與填滿', (await raw(119, 1))?.font.size === 36 && (await raw(119, 2))?.font.name === 'Arial' && (await raw(119, 2))?.font.bold === true && (await raw(119, 1))?.wrap === true && (await raw(119, 2))?.fill === '00B050');
  expect('大字級與換行產生自動列高', await height(119) > beforeHeight);
  await sheet.runMenu('undo'); await step();
  expect('一次復原同時還原格式與自動列高', JSON.stringify(await raw(119, 1)) === JSON.stringify(beforeCombinedA) && JSON.stringify(await raw(119, 2)) === JSON.stringify(beforeCombinedB) && await height(119) === beforeHeight);
  await sheet.runMenu('redo'); await step();
  expect('一次重做同時還原格式與自動列高', (await raw(119, 1))?.font.size === 36 && await height(119) > beforeHeight);

  await sheetApi.setCell(sheetIndex, 120, 1, 'manual'); await sheetApi.setSizes(sheetIndex, true, [120], 25);
  await select(120, 1); await open(); await tab('字型'); await setSize(72); await pickFill('FFFF00'); button('確定').click(); await step();
  expect('跨分頁格式不覆寫手動列高', (await raw(120, 1))?.font.size === 72 && await height(120) === 25);
  await sheet.runMenu('undo'); await step();
  expect('手動列高在復原後仍保留', (await raw(120, 1))?.font.size === 11 && await height(120) === 25);

  await set(123, 1, { fill: 'FF0000' }); await set(124, 3, { fill: '0000FF' }); await set(123, 2, { fill: 'FF00FF' });
  await select(123, 1);
  sheet.setSelectionForTest({ active: { row: 123, col: 1 }, anchor: { row: 123, col: 1 }, focus: { row: 124, col: 3 }, ranges: [{ r0: 123, c0: 1, r1: 123, c1: 1 }, { r0: 124, c0: 3, r1: 124, c1: 3 }] }); await step();
  await open(); await pickFill('FFFF00'); button('確定').click(); await step();
  expect('不連續多選只更新所選範圍', (await raw(123, 1))?.fill === 'FFFF00' && (await raw(124, 3))?.fill === 'FFFF00' && (await raw(123, 2))?.fill === 'FF00FF');
  await sheet.runMenu('undo'); await step();
  expect('不連續選取一次復原', (await raw(123, 1))?.fill === 'FF0000' && (await raw(124, 3))?.fill === '0000FF');
  const beforeCancel = await version(); await open(); await pickFill('FFFFFF'); button('取消').click(); await step();
  expect('取消混合草稿不增加修改紀錄', await version() === beforeCancel);
  for (const extension of ['xlsx', 'wwx']) {
    const path = await benchApi.saveCopy(`selftest-mixed.${extension}`); await documentApi.openPath(path); await step();
    expect(`${extension} 重開保留混合格式與合併列高提交結果`, (await raw(119, 1))?.font.size === 36 && (await raw(119, 2))?.fill === '00B050' && await height(119) > beforeHeight && (await raw(123, 1))?.fill === 'FF0000' && (await raw(124, 3))?.fill === '0000FF');
  }
  } catch (error) {
    expect('驗收流程完成', false);
    lines.push(`| ❌ | ${String(error).replaceAll('|', '/').replaceAll('\n', ' ')} |`);
  }
  lines.splice(2, 0, `- 結果：${passed} / ${total} 通過`, '');
  return lines.join('\n') + '\n';
}
