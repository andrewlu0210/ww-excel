import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

// Exercise the real App modal and backend through the desktop WebView.
export async function cellFormatDialogSelfTest(sheet: SheetDriver, sheetIndex: number): Promise<string> {
  const lines = ['\n## 儲存格格式視窗', '', '| | 項目 |', '| --- | --- |'];
  let passed = 0, total = 0;
  const expect = (name: string, ok: boolean) => {
    total++;
    if (ok) passed++;
    lines.push(`| ${ok ? '✅' : '❌'} | ${name} |`);
    if (!ok) console.error(`Cell format dialog: ${name}`);
  };
  const step = async () => { await sheet.whenIdle(); await new Promise(r => setTimeout(r, 100)); };
  const modal = () => document.querySelector<HTMLElement>('[role="dialog"][aria-label="儲存格格式"]')!;
  const button = (text: string) => Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.trim() === text)!;
  const tab = async (name: string) => { button(name).click(); await step(); };
  const read = async () => {
    const data = await sheetApi.getBlocks(sheetIndex, [{ row: 0, col: 0 }], 0);
    const cell = data.blocks?.[0]?.cells?.find(c => c.r === 49 && c.c === 0);
    return { value: cell?.v, style: data.styles?.find(s => s.id === (cell?.st ?? 0)) };
  };
  const open = async () => { sheet.focus(); sheet.pressKey('1', { metaKey: true }); await step(); };
  const escape = async () => {
    (document.activeElement ?? modal()).dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await step();
  };
  sheet.pressKey('Escape');
  await sheet.goToName('A50');
  await sheetApi.setCell(sheetIndex, 49, 0, '1234.5');
  await sheet.applyFormat({ numFmt: '', bold: false, italic: false, fill: '', alignH: '', wrap: false });
  await step();
  await open();
  expect('⌘1 開啟五分頁共用視窗', !!modal() && modal().querySelectorAll('[role="tab"]').length === 5);
  const code = modal().querySelector<HTMLInputElement>('input')!;
  code.value = '#,##0.00'; code.dispatchEvent(new Event('input', { bubbles: true }));
  await tab('字型');
  const bold = modal().querySelector<HTMLInputElement>('input[type="checkbox"]')!;
  bold.click();
  await tab('對齊');
  const align = modal().querySelector<HTMLSelectElement>('select')!;
  align.value = 'center'; align.dispatchEvent(new Event('change', { bubbles: true }));
  await tab('填滿');
  const fillButton = Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b => b.textContent?.includes('填滿色彩'))!;
  fillButton.click(); await step();
  const yellow = modal().querySelector<HTMLButtonElement>('[aria-label="#FFFF00"]')!;
  yellow.focus(); yellow.click(); await step();
  expect('色盤選色後焦點回到色彩按鈕', document.activeElement === fillButton);
  expect('色盤選色不會提前提交', !!modal() && !(await read()).style?.fill);
  await tab('數值');
  expect('切換分頁保留格式碼與 Go 預覽', modal().querySelector<HTMLInputElement>('input')?.value === '#,##0.00' && modal().querySelector('.value')?.textContent === '1,234.50');
  expect('草稿尚未修改文件', (await read()).value === '1234.5' && !(await read()).style?.font.bold);
  button('確定').click(); await step();
  const applied = await read();
  expect('確定套用跨分頁草稿', !modal() && applied.value === '1,234.50' && !!applied.style?.font.bold && applied.style.alignH === 'center' && applied.style.fill === 'FFFF00');
  const beforeNoop = await documentApi.state();
  await open(); button('確定').click(); await step();
  expect('未修改直接確定不新增復原紀錄', JSON.stringify(await documentApi.state()) === JSON.stringify(beforeNoop));
  await open(); await tab('字型');
  const size = modal().querySelector<HTMLInputElement>('input[type="number"]')!;
  const oldSize = size.value;
  size.value = ''; size.dispatchEvent(new Event('input', { bubbles: true }));
  await tab('對齊');
  expect('無效字級跨分頁仍阻止提交', button('確定').disabled);
  await tab('字型');
  const restoredSize = modal().querySelector<HTMLInputElement>('input[type="number"]')!;
  restoredSize.value = oldSize; restoredSize.dispatchEvent(new Event('input', { bubbles: true }));
  modal().querySelector<HTMLInputElement>('input[type="checkbox"]')!.click();
  button('取消').click(); await step();
  expect('取消保留文件原格式', !!(await read()).style?.font.bold && !modal());
  await open(); await escape();
  expect('Esc 關閉視窗且不修改文件', !modal() && !!(await read()).style?.font.bold);
  const scroller = document.querySelector<HTMLElement>('.scroller')!;
  const rect = scroller.getBoundingClientRect();
  scroller.dispatchEvent(new MouseEvent('contextmenu', { clientX: rect.left + 60, clientY: rect.top + 60, bubbles: true, cancelable: true }));
  await step();
  Array.from(document.querySelectorAll<HTMLButtonElement>('.context-menu button')).find(b => b.textContent?.includes('儲存格格式'))!.click();
  await step();
  expect('右鍵開啟同一個五分頁視窗', !!modal() && modal().querySelectorAll('[role="tab"]').length === 5);
  await tab('數值');
  button('數值').focus();
  button('數值').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true }));
  await step();
  expect('方向鍵切換分頁並移動焦點', document.activeElement?.textContent === '字型' && document.activeElement?.getAttribute('aria-selected') === 'true');
  const submit = button('確定'); submit.focus();
  submit.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }));
  expect('Tab 焦點循環留在視窗內', modal().contains(document.activeElement) && document.activeElement !== submit);
  const original = await benchApi.windowSize();
  try {
    await benchApi.setWindowSize(500, 360); await step();
    await tab('數值');
    const body = modal().querySelector<HTMLElement>('.scroll-body')!;
    body.scrollTop = body.scrollHeight;
    const bounds = modal().getBoundingClientRect();
    expect('小視窗內容可捲動且確定按鈕留在畫面內', body.scrollHeight > body.clientHeight && body.scrollTop > 0 && bounds.top >= 0 && bounds.bottom <= innerHeight && button('確定').getBoundingClientRect().bottom <= innerHeight);
  } finally { await benchApi.setWindowSize(original.width, original.height); await step(); }
  await escape();
  await sheet.goToName('A50');await sheetApi.setCell(sheetIndex,49,0,'40982');await sheet.applyFormat({numFmt:''});await step();
  await open();
  const category=(id:string)=>modal().querySelector<HTMLButtonElement>(`[data-number-category="${id}"]`)!;
  expect('數值分頁只保留日期與時間分類',modal().querySelectorAll('[data-number-category="date"]').length===1 && modal().querySelectorAll('[data-number-category="time"]').length===1 && !modal().querySelector('.list')?.textContent?.includes('民國年'));
  category('date').click();await step();
  const types=modal().querySelector<HTMLSelectElement>('[aria-label="類型"]')!;
  expect('日期右側類型包含西元與民國格式',types.options.length===10 && Array.from(types.options).some(o=>o.value==='[$-404]e/m/d'));
  types.value='[$-404]gge"年"m"月"d"日"';types.dispatchEvent(new Event('change',{bubbles:true}));await step();
  expect('選擇民國中文類型同步格式碼及預覽',modal().querySelector<HTMLInputElement>('input')?.value==='[$-404]gge"年"m"月"d"日"' && modal().querySelector('.value')?.textContent==='民國101年3月14日');
  await tab('字型');await tab('數值');expect('切換分頁保留日期分類與類型',category('date').getAttribute('aria-pressed')==='true' && modal().querySelector<HTMLSelectElement>('[aria-label="類型"]')?.value==='[$-404]gge"年"m"月"d"日"');
  button('確定').click();await step();expect('日期類型確定後實際套用',(await read()).style?.numFmt==='[$-404]gge"年"m"月"d"日"' && (await read()).value==='民國101年3月14日');
  await open();expect('重新開啟自動選回日期與原民國類型',category('date').getAttribute('aria-pressed')==='true' && modal().querySelector<HTMLSelectElement>('[aria-label="類型"]')?.value==='[$-404]gge"年"m"月"d"日"');
  category('time').click();await step();expect('上午下午類型收在時間分類',Array.from(modal().querySelector<HTMLSelectElement>('[aria-label="類型"]')!.options).some(o=>o.value==='上午/下午 h"時"mm"分"'));button('取消').click();await step();
  expect('取消時間類型草稿保留原日期',(await read()).style?.numFmt==='[$-404]gge"年"m"月"d"日"');
  await open();category('date').click();await step();
  for (const [format,want] of [
    ['m"月"d"日"','3月14日'],
    ['[DBNum1][$-404]yyyy"年"m"月"d"日"','二○一二年三月十四日'],
    ['[DBNum1][$-404]m"月"d"日"','三月十四日'],
    ['[$-404]dddd','星期三'],
    ['[$-404]ddd','週三'],
  ]) {
    const list=modal().querySelector<HTMLSelectElement>('[aria-label="類型"]')!;
    list.value=format;list.dispatchEvent(new Event('change',{bubbles:true}));await step();
    expect(`日期類型預覽：${want}`,modal().querySelector('.value')?.textContent===want);
  }
  button('確定').click();await step();expect('星期類型實際套用為週三',(await read()).value==='週三' && (await read()).style?.numFmt==='[$-404]ddd');
  await open();expect('重開保留星期類型',category('date').getAttribute('aria-pressed')==='true' && modal().querySelector<HTMLSelectElement>('[aria-label="類型"]')?.value==='[$-404]ddd');button('取消').click();await step();
  await sheet.applyFormat({numFmt:'"ID-"00000'});await step();await open();expect('未列出的格式歸自訂且保留原碼',category('custom').getAttribute('aria-pressed')==='true' && modal().querySelector<HTMLInputElement>('input')?.value==='"ID-"00000');
  const noChange=JSON.stringify(await documentApi.state());button('確定').click();await step();expect('僅檢視自訂分類不增加修改紀錄',JSON.stringify(await documentApi.state())===noChange);
  lines.splice(2, 0, `- 結果：${passed} / ${total} 通過`, '');
  return lines.join('\n') + '\n';
}
