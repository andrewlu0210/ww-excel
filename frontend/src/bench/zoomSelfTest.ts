// Exercises real Svelte controls and WebView geometry in WW_SELFTEST mode.
import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';
import type { Rect } from '../grid/renderer';
import type { CellPos } from '../grid/selection';

interface GridView {
  cellRect(pos: CellPos): Rect;
  cellAtPoint(x: number, y: number): { pos: CellPos; rect: Rect };
  scrollIntoView(pos: CellPos): void;
  scrollToOrigin(): void;
  topLeft(): CellPos;
  snapshot(): string;
}

export async function zoomSelfTest(
  sheet: SheetDriver,
  step: () => Promise<void>,
  expect: (name: string, ok: boolean, detail: string) => void,
): Promise<void> {
  const slider = () => document.querySelector<HTMLInputElement>('.zoom .slider')!;
  const percentage = () => document.querySelector<HTMLButtonElement>('.zoom .percentage')!;
  const grid = () => sheet.getGrid() as unknown as GridView;
  const setZoom = async (value: number) => {
    slider().value = String(value);
    slider().dispatchEvent(new Event('input', { bubbles: true }));
    await step();
  };
  const inputZoom = async (value: string, key = 'Enter') => {
    percentage().click();
    await step();
    const input = document.querySelector<HTMLInputElement>('.zoom-dialog .custom-percent')!;
    input.focus();
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    if (key === 'Escape') document.querySelector<HTMLDialogElement>('.zoom-dialog')!.dispatchEvent(new Event('cancel', { cancelable: true }));
    else document.querySelector<HTMLFormElement>('.zoom-dialog form')!.requestSubmit();
    await step();
  };
  const sameCell = (a: CellPos, b: CellPos) => a.row === b.row && a.col === b.col;
  const pos = { row: 110, col: 12 };
  const layout = await sheetApi.layout(0, 0);
  await sheetApi.freeze(0, 1, 1);
  await step();
  grid().scrollIntoView(pos);
  await step();
  const base = grid().cellRect(pos);
  const initial = await documentApi.state();

  for (const zoom of [10, 70, 100, 200, 400]) {
    const anchor = grid().topLeft();
    await setZoom(zoom);
    expect(`縮放 ${zoom}%：保留捲動錨點`, sameCell(anchor, grid().topLeft()), JSON.stringify(grid().topLeft()));
    grid().scrollIntoView(pos);
    await step();
    const rc = grid().cellRect(pos);
    expect(`縮放 ${zoom}%：格線尺寸`, Math.abs(rc.w - base.w * zoom / 100) < 0.01 && Math.abs(rc.h - base.h * zoom / 100) < 0.01, JSON.stringify(rc));
    const hit = grid().cellAtPoint(rc.x + rc.w / 2, rc.y + rc.h / 2).pos;
    expect(`縮放 ${zoom}%：命中儲存格`, sameCell(pos, hit), JSON.stringify(hit));
    const scroller = document.querySelector<HTMLElement>('.scroller')!;
    const host = scroller.getBoundingClientRect();
    scroller.dispatchEvent(new MouseEvent('contextmenu', { clientX: host.left + rc.x + rc.w / 2, clientY: host.top + rc.y + rc.h / 2, bubbles: true, cancelable: true }));
    await step();
    expect(`縮放 ${zoom}%：滑鼠座標選格`, sameCell(sheet.getSelection().active, pos), JSON.stringify(sheet.getSelection().active));
    scroller.dispatchEvent(new MouseEvent('dblclick', { clientX: host.left + rc.x + rc.w / 2, clientY: host.top + rc.y + rc.h / 2, bubbles: true }));
    await step();
    expect(`縮放 ${zoom}%：雙擊進入正確儲存格`, sameCell(sheet.getEditing()?.pos ?? { row: -1, col: -1 }, pos), JSON.stringify(sheet.getEditing()));
    const editor = document.querySelector<HTMLTextAreaElement>('.cell-editor')!;
    // Selecting a cell may reveal it and quantize the native scroll offset.
    const editorRect = grid().cellRect(pos);
    expect(`縮放 ${zoom}%：編輯框位置`, Math.abs(parseFloat(editor.style.left) - (editorRect.x - 1)) < 0.01 && Math.abs(parseFloat(editor.style.top) - (editorRect.y - 1)) < 0.01, editor.style.cssText);
    sheet.pressKey('Escape');
    await step();
    expect(`縮放 ${zoom}%：文字大小`, Math.abs(parseFloat(getComputedStyle(editor).fontSize) - 11 * 4 / 3 * zoom / 100) < 0.1, getComputedStyle(editor).fontSize);
    if ([70, 200, 400].includes(zoom)) await benchApi.snapshot(`selftest-zoom-${zoom}`, grid().snapshot());
  }
  const after = await documentApi.state();
  expect('縮放不修改文件或復原紀錄', initial.dirty === after.dirty && initial.undoLabel === after.undoLabel && initial.canUndo === after.canUndo && initial.canRedo === after.canRedo, JSON.stringify(after));

  percentage().click();
  await step();
  expect('點擊百分比開啟縮放視窗', !!document.querySelector<HTMLDialogElement>('.zoom-dialog')?.open, '');
  const preset = document.querySelector<HTMLInputElement>('.zoom-dialog input[value="75"]')!;
  preset.click();
  await step();
  expect('選擇預設比例尚不套用', slider().value === '400', slider().value);
  document.querySelector<HTMLFormElement>('.zoom-dialog form')!.requestSubmit();
  await step();
  expect('確定套用預設比例', slider().value === '75' && !document.querySelector('.zoom-dialog'), slider().value);
  percentage().click();
  await step();
  document.querySelector<HTMLInputElement>('.zoom-dialog input[value="150"]')!.click();
  document.querySelector<HTMLButtonElement>('.zoom-dialog button[type="button"]')!.click();
  await step();
  expect('取消保留原縮放比例', slider().value === '75', slider().value);
  expect('關閉視窗恢復格線焦點', document.activeElement?.classList.contains('cell-editor') ?? false, document.activeElement?.tagName ?? '');

  await inputZoom('70%');
  expect('輸入百分比', slider().value === '70', percentage().textContent ?? '');
  await inputZoom('abc');
  expect('無效百分比保留原值', slider().value === '70' && percentage().textContent === '70%', percentage().textContent ?? '');
  await inputZoom('200', 'Escape');
  expect('Escape 取消百分比', slider().value === '70', percentage().textContent ?? '');
  await inputZoom('999');
  expect('縮放上限', slider().value === '400' && document.querySelectorAll<HTMLButtonElement>('.zoom button')[1].disabled, percentage().textContent ?? '');
  await inputZoom('0');
  expect('縮放下限', slider().value === '10' && document.querySelectorAll<HTMLButtonElement>('.zoom button')[0].disabled, percentage().textContent ?? '');
  await setZoom(100);
  document.querySelectorAll<HTMLButtonElement>('.zoom button')[1].click();
  await step();
  expect('加號增加 10%', slider().value === '110', percentage().textContent ?? '');
  document.querySelectorAll<HTMLButtonElement>('.zoom button')[0].click();
  await step();
  expect('減號減少 10%', slider().value === '100', percentage().textContent ?? '');

  sheet.setSelectionForTest({
    active: { row: 100, col: 8 }, anchor: { row: 100, col: 8 }, focus: { row: 119, col: 19 },
    ranges: [{ r0: 100, c0: 8, r1: 119, c1: 19 }],
  });
  await step();
  percentage().click();
  await step();
  document.querySelector<HTMLInputElement>('.zoom-dialog input[value="fit"]')!.click();
  document.querySelector<HTMLFormElement>('.zoom-dialog form')!.requestSubmit();
  await step();
  const first = grid().cellRect({ row: 100, col: 8 });
  const last = grid().cellRect({ row: 119, col: 19 });
  const scroller = document.querySelector<HTMLElement>('.scroller')!;
  expect('選取範圍最適化容納全部選取範圍', first.x >= 0 && first.y >= 0 && last.x + last.w <= scroller.clientWidth && last.y + last.h <= scroller.clientHeight, JSON.stringify({ first, last, zoom: slider().value }));

  await setZoom(200);
  const chart = sheet.getObjects().find((o) => o.kind === 'chart');
  const chartCanvas = document.querySelector<HTMLCanvasElement>('.object canvas');
  expect('圖表隨比例縮放', !!chart && !!chartCanvas && Math.abs(parseFloat(chartCanvas.style.width) - chart.w * 2) < 0.01, chartCanvas?.style.cssText ?? '');
  await setZoom(70);
  const originalName = (await documentApi.state()).sheets?.[0] ?? '';
  await documentApi.renameSheet(0, '縮放測試');
  await step();
  expect('重新命名保留縮放', slider().value === '70', percentage().textContent ?? '');
  await documentApi.renameSheet(0, originalName);
  await step();
  const added = await documentApi.addSheet((await documentApi.state()).sheets?.length ?? 1);
  await step();
  const selectTab = async (index: number) => {
    document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[index].dispatchEvent(new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true }));
    window.dispatchEvent(new PointerEvent('pointerup'));
    await step();
  };
  await selectTab(added);
  expect('新工作表預設 100%', slider().value === '100', percentage().textContent ?? '');
  await setZoom(200);
  await selectTab(0);
  expect('切回工作表保留 70%', slider().value === '70', percentage().textContent ?? '');
  await selectTab(added);
  expect('各工作表獨立記住比例', slider().value === '200', percentage().textContent ?? '');
  await selectTab(0);
  await documentApi.moveSheet(0, added);
  await step();
  expect('工作表排序保留縮放', slider().value === '70', percentage().textContent ?? '');
  await documentApi.moveSheet(added, 0);
  await step();
  await documentApi.deleteSheet(added);
  await step();
  await sheetApi.freeze(0, layout.freezeRows ?? 0, layout.freezeCols ?? 0);
  await step();
  await setZoom(70);
  const saved = await benchApi.saveCopy('selftest-zoom.xlsx');
  await documentApi.openPath(saved);
  await step();
  await step();
  expect('重新開檔恢復 100%', slider().value === '100', percentage().textContent ?? '');
  sheet.focus();
}
