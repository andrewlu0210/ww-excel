// Scripted editing self-test (WW_SELFTEST=1). Drives the real sheet
// controller with simulated key events and typed text, then checks the
// Go model (source of truth) and the frontend cache. Native IME input
// and menu accelerators cannot be simulated here; test those by hand.
import { benchApi, documentApi, sheetApi, type FormatPatch, type PasteMode } from '../bridge/sheet';
import type { MenuAction } from '../input/keymap';
import type { Range } from '../grid/selection';
import { ensureFont } from '../stores/fontLoader';
import { fontCss, styleStore } from '../stores/styles';
import type { CellPos, Selection } from '../grid/selection';
import { cellAddress } from '../grid/layout';
import type { BlockCache } from '../stores/viewportCache';

export interface SheetDriver {
  pressKey(key: string, mods?: Partial<KeyboardEventInit>): void;
  clickCell(pos: CellPos, kind?: 'down' | 'drag' | 'up' | 'dbl'): void;
  focusBar(): void;
  getBarText(): string;
  getAssist(): { suggestions: string[]; hint: string | null; error: string | null; highlights: unknown[] };
  typeText(text: string): void;
  whenIdle(): Promise<void>;
  getSelection(): Selection;
  getEditing(): { mode: string; pos: CellPos } | null;
  setSelectionForTest(sel: Selection): void;
  applyFormat(patch: FormatPatch): Promise<string | null>;
  mergeCells(center: boolean): Promise<string | null>;
  focus(): void;
  getGrid(): { cache: BlockCache } | undefined;
  // v0.5
  runMenu(action: MenuAction, pasteText?: string | null): Promise<void>;
  paste(text: string | null): Promise<void>;
  pasteSpecial(mode: PasteMode, transpose: boolean, skipBlanks: boolean): Promise<void>;
  getCopyRange(): Range | null;
  dragFillHandle(to: CellPos): void;
  // v0.6
  goToName(text: string): Promise<void>;
  // v0.7
  chartImages(): Record<number, string>;
  getObjects(): { id: number; kind: string; row: number; col: number; w: number; h: number }[];
}

interface Check {
  name: string;
  ok: boolean;
  detail: string;
}

const settle = () => new Promise((r) => setTimeout(r, 60));

function addr(p: CellPos): string {
  return cellAddress(p.row, p.col);
}

export async function runSelfTest(sheet: SheetDriver, sheetIndex: number): Promise<{ markdown: string; failed: number }> {
  const checks: Check[] = [];

  const step = async () => {
    await sheet.whenIdle();
    await settle();
    await sheet.whenIdle();
  };

  const key = async (k: string, mods: Partial<KeyboardEventInit> = {}) => {
    sheet.pressKey(k, mods);
    await step();
  };
  const type = async (text: string) => {
    sheet.typeText(text);
    await step();
  };
  const goTo = async (row: number, col: number) => {
    await key('Home', { metaKey: true });
    for (let i = 0; i < row; i++) await key('ArrowDown');
    for (let i = 0; i < col; i++) await key('ArrowRight');
  };

  const expectCell = async (name: string, row: number, col: number, want: string) => {
    const got = await sheetApi.editText(sheetIndex, row, col);
    checks.push({ name, ok: got === want, detail: `${cellAddress(row, col)} = ${JSON.stringify(got)}，預期 ${JSON.stringify(want)}` });
  };
  const expectActive = (name: string, row: number, col: number) => {
    const a = sheet.getSelection().active;
    checks.push({ name, ok: a.row === row && a.col === col, detail: `作用中 ${addr(a)}，預期 ${cellAddress(row, col)}` });
  };
  // JSON.stringify(undefined) is undefined, so coerce details to text.
  const expect = (name: string, ok: boolean, detail: string | undefined) =>
    checks.push({ name, ok, detail: String(detail ?? '') });

  // 1. Typing starts Enter mode; Enter commits and moves down.
  await type('Hello');
  expect('輸入時進入「輸入」模式', sheet.getEditing()?.mode === 'enter', `editing = ${JSON.stringify(sheet.getEditing())}`);
  await key('Enter');
  await expectCell('Enter 提交', 0, 0, 'Hello');
  expectActive('Enter 下移', 1, 0);

  // 2. Tab across, then Enter returns to the starting column.
  await type('123');
  await key('Tab');
  await type('台北');
  await key('Enter');
  await expectCell('數字', 1, 0, '123');
  await expectCell('中文文字', 1, 1, '台北');
  expectActive('Tab 後 Enter 回到起始欄', 2, 0);
  const cached = sheet.getGrid()?.cache.cell(1, 0);
  expect('前端快取收到 diff（數字靠右）', cached?.text === '123' && cached.numeric, `cache(A2) = ${JSON.stringify(cached)}`);

  // 3. F2 edits existing content (Edit mode: arrows move the caret).
  await key('ArrowUp');
  await key('ArrowUp');
  await key('F2');
  expect('F2 進入「編輯」模式', sheet.getEditing()?.mode === 'edit', `editing = ${JSON.stringify(sheet.getEditing())}`);
  // Synthetic keys do not move the caret natively, so it stays at the end.
  await key('ArrowLeft');
  expect('編輯模式方向鍵不提交', sheet.getEditing() !== null, `editing = ${JSON.stringify(sheet.getEditing())}`);
  await type('!');
  await key('Enter');
  const a1 = 'Hello!';
  await expectCell('F2 編輯後提交', 0, 0, a1);

  // 4. Escape cancels.
  await type('zzz');
  await key('Escape');
  await expectCell('Esc 取消編輯', 1, 0, '123');
  expect('Esc 後回到就緒', sheet.getEditing() === null, '');

  // 5. Undo / redo restore content and select the affected cell.
  await key('ArrowDown');
  await key('z', { metaKey: true });
  await expectCell('⌘Z 復原', 0, 0, 'Hello');
  expectActive('復原後選取受影響儲存格', 0, 0);
  await key('z', { metaKey: true, shiftKey: true });
  await expectCell('⇧⌘Z 重做', 0, 0, a1);

  // 6. ⌘Enter fills the whole selection.
  await goTo(4, 0);
  await key('ArrowDown', { shiftKey: true });
  await key('ArrowRight', { shiftKey: true });
  await type('x');
  await key('Enter', { metaKey: true });
  await expectCell('⌘Enter 填滿 A5', 4, 0, 'x');
  await expectCell('⌘Enter 填滿 B6', 5, 1, 'x');
  const sel = sheet.getSelection();
  expect('⌘Enter 保留選取範圍', sel.ranges.length === 1 && sel.ranges[0].r1 === 5 && sel.ranges[0].c1 === 1, JSON.stringify(sel.ranges));

  // 7. Enter cycles inside the selection.
  await key('Enter');
  expectActive('選取範圍內 Enter 循環', 5, 0);

  // 8. Delete clears the selection; undo restores it.
  await key('Backspace');
  await expectCell('Delete 清除', 5, 1, '');
  await key('z', { metaKey: true });
  await expectCell('清除後復原', 5, 1, 'x');

  // 9. ⌘↓ jumps to data edges.
  await goTo(0, 0);
  await key('ArrowDown', { metaKey: true });
  expectActive('⌘↓ 到連續資料尾端', 1, 0);
  await key('ArrowDown', { metaKey: true });
  expectActive('⌘↓ 跳到下一段資料', 4, 0);
  await key('ArrowDown', { metaKey: true, shiftKey: true });
  const ext = sheet.getSelection().ranges[0];
  expect('⇧⌘↓ 延伸選取', ext.r0 === 4 && ext.r1 === 5, JSON.stringify(ext));

  // 10. ⌥Enter inserts a line break.
  await goTo(0, 3);
  await type('line1');
  await key('Enter', { altKey: true });
  await type('line2');
  await key('Enter');
  await expectCell('⌥Enter 換行', 0, 3, 'line1\nline2');

  // 11. In Enter mode an arrow commits and moves.
  await goTo(0, 4);
  await type('7');
  await key('ArrowRight');
  await expectCell('輸入模式方向鍵提交', 0, 4, '7');
  expectActive('輸入模式方向鍵移動', 0, 5);

  // 12. Apostrophe keeps numbers as text.
  await type("'007");
  await key('Enter');
  await expectCell('單引號保留文字', 0, 5, "'007");

  // 13. Whole row / sheet selection.
  await key(' ', { shiftKey: true });
  const row = sheet.getSelection().ranges[0];
  expect('⇧Space 選整列', row.c0 === 0 && row.c1 === 16383 && row.r0 === row.r1, JSON.stringify(row));
  await key('a', { metaKey: true });
  const all = sheet.getSelection().ranges[0];
  expect('⌘A 全選', all.r1 === 1_048_575 && all.c1 === 16_383, JSON.stringify(all));

  // ---- v0.3 formulas ----
  await key('Escape');
  await goTo(7, 0);
  await type('3');
  await key('Enter');
  await type('4');
  await key('Enter'); // A8=3, A9=4, now at A10

  // 14. Autocomplete offers functions alphabetically (like Excel, "=su"
  // starts at SUBSTITUTE); Tab inserts NAME( and shows the hint.
  await type('=su');
  const sug = sheet.getAssist().suggestions;
  expect('函數自動完成', sug[0] === 'SUBSTITUTE' && sug.includes('SUM') && sug.includes('SUMIF'), JSON.stringify(sug));
  await type('m');
  expect('繼續輸入縮小清單', sheet.getAssist().suggestions[0] === 'SUM', JSON.stringify(sheet.getAssist().suggestions));
  await key('Tab');
  expect('Tab 接受建議並顯示參數提示', sheet.getAssist().hint === 'SUM', JSON.stringify(sheet.getAssist()));

  // 15. Point mode: arrows insert a reference, Shift extends it.
  await key('ArrowUp');
  expect('方向鍵插入參照（指向模式）', sheet.getAssist().highlights.length === 1, JSON.stringify(sheet.getAssist().highlights));
  await key('ArrowUp', { shiftKey: true });
  await type(')');
  await key('Enter');
  await expectCell('指向模式產生 =SUM(A8:A9)', 9, 0, '=SUM(A8:A9)');
  expect('公式結果', sheet.getGrid()?.cache.cell(9, 0)?.text === '7', `cache(A10) = ${JSON.stringify(sheet.getGrid()?.cache.cell(9, 0))}`);

  // 16. Editing a precedent recalculates the formula.
  await goTo(7, 0);
  await type('10');
  await key('Enter');
  expect('修改參照儲存格後重算', sheet.getGrid()?.cache.cell(9, 0)?.text === '14', `cache(A10) = ${JSON.stringify(sheet.getGrid()?.cache.cell(9, 0))}`);

  // 17. F4 cycles absolute references.
  await goTo(9, 1);
  await type('=A8');
  await key('F4');
  await key('Enter');
  await expectCell('F4 切換為 $A$8', 9, 1, '=$A$8');

  // 18. Clicking a cell while editing a formula inserts it.
  await goTo(9, 3);
  await type('=');
  sheet.clickCell({ row: 7, col: 0 });
  sheet.clickCell({ row: 7, col: 0 }, 'up');
  await step();
  await type('*2');
  await key('Enter');
  await expectCell('滑鼠點選插入參照', 9, 3, '=A8*2');
  expect('滑鼠點選後的結果', sheet.getGrid()?.cache.cell(9, 3)?.text === '20', JSON.stringify(sheet.getGrid()?.cache.cell(9, 3)));

  // 19. Lower-case input is stored canonically; missing ")" is added.
  await goTo(9, 4);
  await type('=round(a8/3,1');
  await key('Enter');
  await expectCell('公式標準化並自動補上括號', 9, 4, '=ROUND(A8/3,1)');

  // 20. A bad formula keeps the editor open with an explanation.
  await goTo(9, 5);
  await type('=SUM(1;2)');
  await key('Enter');
  expect('公式錯誤時保持編輯並提示', sheet.getEditing() !== null && !!sheet.getAssist().error, JSON.stringify(sheet.getAssist().error));
  await expectCell('錯誤公式不寫入', 9, 5, '');
  await key('Escape');

  // 21. Errors and logicals are centered.
  await goTo(10, 0);
  await type('=1/0');
  await key('Tab');
  await type('=1=1');
  await key('Enter');
  const e1 = sheet.getGrid()?.cache.cell(10, 0);
  const e2 = sheet.getGrid()?.cache.cell(10, 1);
  expect('錯誤值置中', e1?.text === '#DIV/0!' && e1.center, JSON.stringify(e1));
  expect('邏輯值置中', e2?.text === 'TRUE' && e2.center, JSON.stringify(e2));

  // 22. The formula bar shows the formula of the active cell and edits it.
  await goTo(9, 0);
  expect('公式列顯示公式', sheet.getBarText() === '=SUM(A8:A9)', JSON.stringify(sheet.getBarText()));
  sheet.focusBar();
  await step();
  expect('點選公式列進入編輯模式', sheet.getEditing()?.mode === 'edit', JSON.stringify(sheet.getEditing()));
  await key('Escape');

  // 23. Circular references show 0 and are reported.
  await goTo(11, 0);
  await type('=A12+1');
  await key('Enter');
  const st = await documentApi.state();
  expect('循環參照回報', st.circular === 'A12', JSON.stringify(st.circular));
  await expectCell('循環參照公式保留', 11, 0, '=A12+1');
  await key('ArrowUp');
  await key('Backspace');

  await formatTests(sheet, { key, type, goTo, step, expect, expectCell });
  await productivityTests(sheet, { key, type, goTo, step, expect, expectCell });
  await dataTests(sheet, { key, type, goTo, step, expect, expectCell });
  await outputTests(sheet, { key, type, goTo, step, expect, expectCell });

  // Show a formula being edited, with reference boxes, for the snapshot.
  await goTo(12, 2);
  await type('=SUM(');
  sheet.clickCell({ row: 7, col: 0 });
  sheet.clickCell({ row: 8, col: 0 }, 'drag');
  sheet.clickCell({ row: 8, col: 0 }, 'up');
  await step();
  await type(')+B10*');
  await key('ArrowUp');
  await key('ArrowUp');
  await key('ArrowLeft');
  await step();
  return finish(checks);
}

function finish(checks: Check[]): { markdown: string; failed: number } {
  const failed = checks.filter((c) => !c.ok).length;
  const lines = [
    '# Self-test',
    '',
    `- 時間：${new Date().toISOString()}`,
    `- 結果：${checks.length - failed} / ${checks.length} 通過`,
    '',
    '| | 項目 | 細節 |',
    '| --- | --- | --- |',
    ...checks.map((c) => `| ${c.ok ? '✅' : '❌'} | ${c.name} | ${c.detail.replace(/\|/g, '\\|').replace(/\n/g, '\\n')} |`),
  ];
  return { markdown: lines.join('\n') + '\n', failed };
}

// The v0.2 snapshot: a multi-range selection.
export async function multiRangeSnapshot(sheet: SheetDriver): Promise<void> {
  sheet.setSelectionForTest({
    active: { row: 4, col: 0 },
    anchor: { row: 4, col: 0 },
    focus: { row: 5, col: 1 },
    ranges: [
      { r0: 0, c0: 3, r1: 1, c1: 4 },
      { r0: 4, c0: 0, r1: 5, c1: 1 },
    ],
  });
  await sheet.whenIdle();
}

interface Helpers {
  key(k: string, mods?: Partial<KeyboardEventInit>): Promise<void>;
  type(text: string): Promise<void>;
  goTo(row: number, col: number): Promise<void>;
  step(): Promise<void>;
  expect(name: string, ok: boolean, detail: string | undefined): void;
  expectCell(name: string, row: number, col: number, want: string): Promise<void>;
}

const area = (r0: number, c0: number, r1 = r0, c1 = c0): Selection => ({
  active: { row: r0, col: c0 },
  anchor: { row: r0, col: c0 },
  focus: { row: r1, col: c1 },
  ranges: [{ r0, c0, r1, c1 }],
});

// v0.4: formats, sizes, merges, sheets and an xlsx round trip. Values
// and styles are read from Go (GetBlocks), which is the source of truth.
async function formatTests(sheet: SheetDriver, h: Helpers): Promise<void> {
  const { key, type, goTo, step, expect, expectCell } = h;
  const cellAt = async (row: number, col: number, sheetIndex = 0) => {
    const res = await sheetApi.getBlocks(sheetIndex, [{ row: Math.floor(row / 64), col: Math.floor(col / 32) }], 0);
    const styles = new Map((res.styles ?? []).map((s) => [s.id, s]));
    const c = res.blocks?.[0]?.cells?.find((x) => x.r === row % 64 && x.c === col % 32);
    return { v: c?.v ?? '', style: styles.get(c?.st ?? 0) };
  };
  const layout = () => sheetApi.layout(0, 0);

  // 24. Number formats and font styles from the toolbar.
  await goTo(14, 0);
  await type('1234.5');
  await key('Enter');
  sheet.setSelectionForTest(area(14, 0));
  await sheet.applyFormat({ bold: true, numFmt: '#,##0.00', fill: 'FFF2CC', label: '格式' });
  await step();
  let c = await cellAt(14, 0);
  expect('數值格式與粗體、填色', c.v === '1,234.50' && !!c.style?.font.bold && c.style.fill === 'FFF2CC', JSON.stringify(c));
  expect('前端樣式表同步', styleStore.get(sheet.getGrid()?.cache.cell(14, 0)?.style ?? 0)?.dto.font.bold === true, '');

  // 25. Typing detects formats.
  await goTo(14, 1);
  await type('2026/10/23');
  await key('Tab');
  await type('12%');
  await key('Tab');
  await type('$1,234');
  await key('Enter');
  expect('輸入日期自動套用日期格式', (await cellAt(14, 1)).v === '2026/10/23', JSON.stringify(await cellAt(14, 1)));
  expect('輸入百分比', (await cellAt(14, 2)).v === '12%', JSON.stringify(await cellAt(14, 2)));
  expect('輸入貨幣', (await cellAt(14, 3)).v.startsWith('$1,234'), JSON.stringify(await cellAt(14, 3)));
  const widened = ((await layout()).cols ?? []).find((s) => s.i === 1);
  expect('日期放不下時自動加寬欄', (widened?.px ?? 0) > 64, JSON.stringify(widened));

  // 26. Editing keeps the format; Delete keeps it too.
  await goTo(14, 0);
  await type('99');
  await key('Enter');
  c = await cellAt(14, 0);
  expect('修改值時保留格式', c.v === '99.00' && !!c.style?.font.bold, JSON.stringify(c));

  // 27. Outside borders only on the range edges.
  sheet.setSelectionForTest(area(16, 0, 17, 2));
  await sheet.applyFormat({ border: 'outside', label: '框線' });
  await step();
  const corner = await cellAt(16, 0);
  const middle = await cellAt(16, 1);
  expect(
    '外框線只套在邊緣',
    corner.style?.top.style === 'thin' && corner.style.left.style === 'thin' && middle.style?.left.style !== 'thin',
    JSON.stringify({ corner: corner.style, middle: middle.style }),
  );

  // 28. Merge & center; clicking inside selects the whole merge.
  await goTo(19, 0);
  await type('標題');
  await key('Enter');
  sheet.setSelectionForTest(area(19, 0, 19, 2));
  await sheet.mergeCells(true);
  await step();
  const merges = (await layout()).merges ?? [];
  expect('跨欄置中', merges.length === 1 && merges[0].c1 === 2 && (await cellAt(19, 0)).style?.alignH === 'center', JSON.stringify(merges));
  await goTo(19, 0);
  await key('ArrowRight');
  expect('從合併儲存格往右移跳過整個合併', sheet.getSelection().active.col === 3, JSON.stringify(sheet.getSelection().active));

  // 29. Column width and auto row height for larger fonts.
  await sheetApi.setSizes(0, false, [4], 120);
  sheet.setSelectionForTest(area(14, 0));
  await sheet.applyFormat({ fontSize: 20, label: '字型大小' });
  await step();
  const l = await layout();
  const col = (l.cols ?? []).find((s) => s.i === 4);
  const row = (l.rows ?? []).find((s) => s.i === 14);
  expect('調整欄寬', col?.px === 120, JSON.stringify(l.cols));
  expect('放大字型後自動加高列', (row?.px ?? 0) > 20, JSON.stringify(l.rows));

  // 30. A line break turns on wrapping and grows the row.
  await goTo(21, 0);
  await type('第一行');
  await key('Enter', { altKey: true });
  await type('第二行');
  await key('Enter');
  await step();
  const wrapped = await cellAt(21, 0);
  const row22 = ((await layout()).rows ?? []).find((s) => s.i === 21);
  expect('換行自動開啟自動換行', !!wrapped.style?.wrap, JSON.stringify(wrapped));
  expect('換行後自動加高列', (row22?.px ?? 0) > 30, JSON.stringify(row22));

  // 31. Sheets: add, cross-sheet formula, rename, delete, undo.
  await documentApi.addSheet(1);
  await step();
  let st = await documentApi.state();
  expect('新增工作表', st.sheets?.length === 2 && st.sheets[1] === '工作表2', JSON.stringify(st.sheets));
  await sheetApi.setCell(1, 0, 0, '42');
  await goTo(14, 4);
  await type('=工作表2!A1');
  await key('Enter');
  expect('跨工作表公式', (await cellAt(14, 4)).v === '42', JSON.stringify(await cellAt(14, 4)));
  const renameErr = await documentApi.renameSheet(1, '報表');
  await step();
  await expectCell('改名後公式跟著更新', 14, 4, '=報表!A1');
  expect('改名成功', renameErr === null, String(renameErr));
  expect('不合法的名稱被拒絕', (await documentApi.renameSheet(1, 'a/b')) !== null, '');
  await documentApi.deleteSheet(1);
  await step();
  expect('刪除後參照變成 #REF!', (await cellAt(14, 4)).v === '#REF!', JSON.stringify(await cellAt(14, 4)));
  await key('z', { metaKey: true });
  await step();
  st = await documentApi.state();
  expect('復原刪除工作表', st.sheets?.length === 2 && (await cellAt(14, 4)).v === '42', JSON.stringify(st.sheets));

  // 32. xlsx round trip: save, reopen, check values, formats and layout.
  const path = await benchApi.saveCopy('selftest-roundtrip.xlsx');
  await documentApi.openPath(path);
  await step();
  await step();
  c = await cellAt(14, 0);
  const after = await layout();
  st = await documentApi.state();
  expect('xlsx 存讀：數值格式與粗體', c.v === '99.00' && !!c.style?.font.bold && c.style.font.size === 20, JSON.stringify(c));
  expect('xlsx 存讀：合併與欄寬', (after.merges ?? []).length === 1 && (after.cols ?? []).some((s) => s.i === 4 && s.px === 120), JSON.stringify(after.cols));
  expect('xlsx 存讀：工作表與跨表公式', st.sheets?.join(',') === '工作表1,報表' && (await cellAt(14, 4)).v === '42', JSON.stringify(st.sheets));
  expect('xlsx 存讀：日期與換行', (await cellAt(14, 1)).v === '2026/10/23' && !!(await cellAt(21, 0)).style?.wrap, '');
  expect('xlsx 存讀：貨幣格式', (await cellAt(14, 3)).v.startsWith('$1,234'), JSON.stringify(await cellAt(14, 3)));

  // Leave the formatted area on screen for the snapshot.
  sheet.setSelectionForTest(area(14, 0));
  await step();
}

// v0.5: clipboard, fill, insert/delete, hide, freeze, find, sort, filter.
// Runs on sheet 0 of the reopened round-trip file, below row 30.
async function productivityTests(sheet: SheetDriver, h: Helpers): Promise<void> {
  const { key, step, expect, expectCell } = h;
  const set = async (row: number, col: number, text: string) => {
    await sheetApi.setCell(0, row, col, text);
    await step();
  };
  const select = async (r0: number, c0: number, r1 = r0, c1 = c0) => {
    sheet.setSelectionForTest(area(r0, c0, r1, c1));
    await step();
  };
  const shown = async (row: number, col: number) => {
    const res = await sheetApi.getBlocks(0, [{ row: Math.floor(row / 64), col: Math.floor(col / 32) }], 0);
    return res.blocks?.[0]?.cells?.find((x) => x.r === row % 64 && x.c === col % 32)?.v ?? '';
  };
  const layout = () => sheetApi.layout(0, 0);
  const allRows = (r0: number, r1 = r0): Selection => ({
    active: { row: r0, col: 0 },
    anchor: { row: r0, col: 0 },
    focus: { row: r1, col: 16383 },
    ranges: [{ r0, c0: 0, r1, c1: 16383 }],
  });
  const allCols = (c0: number, c1 = c0): Selection => ({
    active: { row: 0, col: c0 },
    anchor: { row: 0, col: c0 },
    focus: { row: 1048575, col: c1 },
    ranges: [{ r0: 0, c0, r1: 1048575, c1 }],
  });

  // 33. ⌘C / ⌘V within the app keeps formulas (adjusted), with ants.
  await set(30, 0, '1');
  await set(30, 1, '=A31*2');
  await select(30, 0, 30, 1);
  await key('c', { metaKey: true });
  expect('⌘C 顯示複製虛線框', JSON.stringify(sheet.getCopyRange()) === JSON.stringify({ r0: 30, c0: 0, r1: 30, c1: 1 }), JSON.stringify(sheet.getCopyRange()));
  await select(32, 0);
  await key('v', { metaKey: true });
  await expectCell('⌘V 貼上公式並調整參照', 32, 1, '=A33*2');
  expect('貼上後選取貼上範圍', JSON.stringify(sheet.getSelection().ranges) === JSON.stringify([{ r0: 32, c0: 0, r1: 32, c1: 1 }]), JSON.stringify(sheet.getSelection().ranges));
  await select(34, 0, 35, 1);
  await sheet.paste(null);
  await step();
  await expectCell('貼到倍數大小的範圍會重複貼上', 35, 1, '=A36*2');
  await key('Escape');
  expect('Esc 結束虛線框', sheet.getCopyRange() === null, '');

  // 34. Text from other apps (Excel TSV) is entered as typed.
  await select(37, 0);
  await sheet.paste('名稱\t數量\n蘋果\t=1+2\n');
  await step();
  await expectCell('外部 TSV 貼上：文字', 37, 0, '名稱');
  await expectCell('外部 TSV 貼上：公式', 38, 1, '=1+2');

  // 35. ⌘X moves cells; formulas that refer to them follow.
  await select(32, 0);
  await key('x', { metaKey: true });
  await select(32, 3);
  await key('v', { metaKey: true });
  await expectCell('剪下貼上移動儲存格', 32, 3, '1');
  await expectCell('剪下後原位置清空', 32, 0, '');
  await expectCell('參照跟著移動', 32, 1, '=D33*2');
  await key('z', { metaKey: true });
  await expectCell('復原剪下貼上', 32, 0, '1');

  // 36. 選擇性貼上: values, transposed.
  await select(30, 0, 30, 1);
  await key('c', { metaKey: true });
  await select(30, 5);
  await sheet.pasteSpecial('values', true, false);
  await step();
  await expectCell('選擇性貼上：值＋轉置（第一格）', 30, 5, '1');
  await expectCell('選擇性貼上：值＋轉置（公式變成值）', 31, 5, '2');
  await key('Escape');

  // 37. Fill handle: numbers continue, lists continue, formulas adjust.
  await set(39, 0, '1');
  await set(40, 0, '3');
  await set(39, 1, '星期一');
  await set(39, 2, '=A40*10');
  await select(39, 0, 40, 0);
  sheet.dragFillHandle({ row: 44, col: 0 });
  await step();
  await expectCell('填滿控點：等差數列', 44, 0, '11');
  await select(39, 1, 39, 2);
  sheet.dragFillHandle({ row: 42, col: 2 });
  await step();
  await expectCell('填滿控點：星期清單', 42, 1, '星期四');
  await expectCell('填滿控點：公式調整參照', 42, 2, '=A43*10');
  await set(39, 3, 'x');
  await select(39, 3, 41, 3);
  await key('d', { metaKey: true });
  await expectCell('⌘D 向下複製', 41, 3, 'x');

  // 38. Insert / delete rows and columns adjust formulas.
  await set(30, 2, '=SUM(A40:A45)');
  expect('插入前的加總', (await shown(30, 2)) === '36', await shown(30, 2));
  sheet.setSelectionForTest(allRows(41));
  await step();
  await key('+', { metaKey: true, shiftKey: true });
  await expectCell('⇧⌘= 插入列：範圍擴大', 30, 2, '=SUM(A40:A46)');
  await expectCell('插入列：下方儲存格下移', 45, 0, '11');
  await key('-', { metaKey: true });
  await expectCell('⌘- 刪除列：範圍還原', 30, 2, '=SUM(A40:A45)');
  sheet.setSelectionForTest(allCols(0));
  await step();
  await sheet.runMenu('insertCols');
  await step();
  await expectCell('插入欄：公式跟著右移', 30, 3, '=SUM(B40:B45)');
  await key('z', { metaKey: true });
  await expectCell('復原插入欄', 30, 2, '=SUM(A40:A45)');

  // 39. Hide and unhide (⌃0 / ⌃⇧0, ⌃9).
  sheet.setSelectionForTest(allCols(4));
  await step();
  await key('0', { ctrlKey: true });
  let l = await layout();
  expect('⌃0 隱藏欄', (l.cols ?? []).some((s) => s.i === 4 && s.hidden), JSON.stringify(l.cols));
  sheet.setSelectionForTest(allCols(3, 5));
  await step();
  await key(')', { ctrlKey: true, shiftKey: true });
  l = await layout();
  expect('⌃⇧0 取消隱藏欄', !(l.cols ?? []).some((s) => s.hidden), JSON.stringify(l.cols));
  sheet.setSelectionForTest(allRows(33));
  await step();
  await key('9', { ctrlKey: true });
  l = await layout();
  expect('⌃9 隱藏列', (l.rows ?? []).some((s) => s.i === 33 && s.hidden), JSON.stringify(l.rows));
  await key('z', { metaKey: true });

  // 40. Find and replace.
  const hit = await sheetApi.findNext(0, 0, 0, { text: '星期', matchCase: false, wholeCell: false, inFormulas: false, allSheets: false }, true);
  expect('尋找', hit?.row === 39 && hit.col === 1, JSON.stringify(hit));
  const rep = await sheetApi.replace(0, { text: '星期', matchCase: false, wholeCell: false, inFormulas: true, allSheets: false }, '週', true, 0, 0);
  await step();
  expect('全部取代', rep.value === 4, JSON.stringify(rep));
  await expectCell('取代結果', 41, 1, '週三');

  // 41. Sort (header detected; blanks last) and AutoFilter.
  const rows = [['名稱', '分數'], ['乙', '70'], ['甲', '90'], ['丙', '80']];
  for (let r = 0; r < rows.length; r++) for (let c = 0; c < 2; c++) await set(50 + r, 7 + c, rows[r][c]);
  await select(52, 8);
  await sheet.runMenu('sortDesc');
  await step();
  expect('遞減排序（自動判斷標題列）', (await shown(51, 7)) === '甲' && (await shown(53, 7)) === '乙' && (await shown(50, 7)) === '名稱', `${await shown(51, 7)} ${await shown(53, 7)}`);
  await select(50, 7);
  await key('f', { metaKey: true, shiftKey: true });
  l = await layout();
  expect('⇧⌘F 開啟篩選', JSON.stringify(l.filter) === JSON.stringify({ r0: 50, c0: 7, r1: 53, c1: 8 }), JSON.stringify(l.filter));
  const values = await sheetApi.filterValues(0, 7);
  expect('篩選清單', values.map((v) => v.text).join(',') === '乙,丙,甲' || values.length === 3, JSON.stringify(values));
  await sheetApi.setFilter(0, 7, ['甲'], false);
  await step();
  l = await layout();
  const hiddenRows = (l.rows ?? []).filter((s) => s.hidden).map((s) => s.i).sort((a, b) => a - b);
  expect('篩選隱藏不符合的列', hiddenRows.join(',') === '52,53', JSON.stringify(hiddenRows));
  expect('篩選欄顯示漏斗', (l.filteredCols ?? []).includes(7), JSON.stringify(l.filteredCols));

  // 42. Freeze panes, then an xlsx round trip of freeze and filter.
  await select(1, 1);
  await sheet.runMenu('freezePanes');
  await step();
  l = await layout();
  expect('凍結窗格', l.freezeRows === 1 && l.freezeCols === 1, `${l.freezeRows},${l.freezeCols}`);
  const path = await benchApi.saveCopy('selftest-v05.xlsx');
  await documentApi.openPath(path);
  await step();
  await step();
  l = await layout();
  expect('xlsx 存讀：凍結窗格', l.freezeRows === 1 && l.freezeCols === 1, `${l.freezeRows},${l.freezeCols}`);
  expect('xlsx 存讀：篩選範圍與隱藏列', JSON.stringify(l.filter) === JSON.stringify({ r0: 50, c0: 7, r1: 53, c1: 8 }) && (l.rows ?? []).some((s) => s.i === 52 && s.hidden), JSON.stringify(l.filter));
  await expectCell('xlsx 存讀：插入刪除後的公式', 30, 2, '=SUM(A40:A45)');

  // Leave the frozen panes, filter and a copy marquee for the snapshot.
  await select(39, 0, 44, 0);
  await key('c', { metaKey: true });
  await select(39, 2);
}

// v0.6: names, notes, data validation, conditional formats, tables.
// Runs on sheet 0 of the reopened v0.5 file, from row 60.
async function dataTests(sheet: SheetDriver, h: Helpers): Promise<void> {
  const { key, type, step, expect, expectCell } = h;
  const set = async (row: number, col: number, text: string) => {
    await sheetApi.setCell(0, row, col, text);
    await step();
  };
  const select = async (r0: number, c0: number, r1 = r0, c1 = c0) => {
    sheet.setSelectionForTest(area(r0, c0, r1, c1));
    await step();
  };
  const cellAt = async (row: number, col: number) => {
    const res = await sheetApi.getBlocks(0, [{ row: Math.floor(row / 64), col: Math.floor(col / 32) }], 0);
    const styles = new Map((res.styles ?? []).map((s) => [s.id, s]));
    const c = res.blocks?.[0]?.cells?.find((x) => x.r === row % 64 && x.c === col % 32);
    return { v: c?.v ?? '', style: styles.get(c?.st ?? 0), bar: c?.bar ?? null };
  };

  // 43. A name defined from the name box, used through autocomplete.
  for (let r = 0; r < 3; r++) await set(60 + r, 0, String(r + 1));
  await select(60, 0, 62, 0);
  await sheet.goToName('數量');
  await step();
  const names = await documentApi.names();
  expect('名稱方塊定義名稱', names.some((n) => n.name === '數量' && n.refersTo.endsWith('!$A$61:$A$63')), JSON.stringify(names));
  await select(63, 0);
  await type('=SUM(數');
  expect('自動完成列出名稱', sheet.getAssist().suggestions.includes('數量'), JSON.stringify(sheet.getAssist().suggestions));
  await key('Tab');
  await type(')');
  await key('Enter');
  await expectCell('使用名稱的公式', 63, 0, '=SUM(數量)');
  expect('名稱公式的結果', (await cellAt(63, 0)).v === '6', JSON.stringify(await cellAt(63, 0)));
  await sheet.goToName('數量');
  await step();
  expect('名稱方塊跳到名稱的範圍', JSON.stringify(sheet.getSelection().ranges) === JSON.stringify([{ r0: 60, c0: 0, r1: 62, c1: 0 }]), JSON.stringify(sheet.getSelection().ranges));
  await sheet.goToName('C5');
  await step();
  expect('名稱方塊跳到位址', sheet.getSelection().active.row === 4 && sheet.getSelection().active.col === 2, JSON.stringify(sheet.getSelection().active));

  // 43b. Chinese fonts resolve to installed macOS fonts (a missing
  // font silently falls back to the default one).
  for (const name of ['Kaiti TC', 'BiauKaiTC', 'LiSong Pro']) await ensureFont(name);
  const canvas = document.createElement('canvas').getContext('2d')!;
  const widthIn = (name: string) => {
    canvas.font = fontCss({ name, size: 20, bold: false, italic: false, underline: false, strike: false, color: '' }).css;
    return canvas.measureText('中文 Ag 字型').width.toFixed(2);
  };
  const widths = ['PingFang TC', 'Songti TC', 'Kaiti TC', '新細明體', '標楷體'].map(widthIn);
  expect('中文字型會套用（宋體、楷體與蘋方字寬不同）', new Set(widths.slice(0, 3)).size === 3, widths.join(' '));
  expect('Windows 中文字型對應到 macOS 字型', widths[3] !== widths[0] && widths[4] !== widths[0], widths.join(' '));
  // A picture of each Chinese font, from row 200.
  const fontRows = ['PingFang TC', 'Songti TC', 'Kaiti TC', 'BiauKaiTC', 'LiSong Pro', 'Yuanti TC', 'Xingkai TC', '新細明體', '標楷體'];
  for (let i = 0; i < fontRows.length; i++) {
    await sheetApi.setCell(0, 200 + i, 0, `${fontRows[i]}：永和中文字型 ABC 123`);
    sheet.setSelectionForTest(area(200 + i, 0));
    await sheet.applyFormat({ fontName: fontRows[i], fontSize: 16, label: '字型' });
  }
  sheet.setSelectionForTest(area(215, 0));
  await step();
  sheet.setSelectionForTest(area(199, 0));
  await new Promise((r) => setTimeout(r, 800));
  const fontGrid = sheet.getGrid() as unknown as { snapshot(): string } | undefined;
  if (fontGrid) await benchApi.snapshot('selftest-fonts', fontGrid.snapshot());

  // 44. Notes.
  await sheetApi.setNote(0, 60, 1, '記得核對');
  await step();
  let layout = await sheetApi.layout(0, 0);
  expect('插入註解', (layout.notes ?? []).some((n) => n.row === 60 && n.col === 1 && n.text === '記得核對'), JSON.stringify(layout.notes));

  // 45. Data validation: a list refuses other values.
  await sheetApi.setValidation(0, [{ r0: 60, c0: 3, r1: 65, c1: 3 }], {
    type: 'list', operator: '', formula1: '是,否', formula2: '', allowBlank: true, dropdown: true,
    errorStyle: 'stop', errorTitle: '', error: '請選擇是或否', promptTitle: '', prompt: '請選擇',
  });
  await step();
  expect('清單選項', JSON.stringify(await sheetApi.listValues(0, 61, 3)) === '["是","否"]', JSON.stringify(await sheetApi.listValues(0, 61, 3)));
  await select(61, 3);
  await type('也許');
  await key('Enter');
  expect('資料驗證拒絕清單外的值', sheet.getEditing() !== null && sheet.getAssist().error === '請選擇是或否', JSON.stringify(sheet.getAssist()));
  await key('Escape');
  await type('是');
  await key('Enter');
  await expectCell('資料驗證接受清單內的值', 61, 3, '是');

  // 46. Conditional formatting and data bars.
  await sheetApi.addCondFormat(0, [{ r0: 60, c0: 0, r1: 62, c1: 0 }], {
    ranges: [], type: 'cell', op: 'greaterThan', value1: '1', value2: '', percent: false, below: false,
    color: '9C0006', fill: 'FFC7CE', bold: false, italic: false, colors: [], stopIfTrue: false,
  });
  await sheetApi.addCondFormat(0, [{ r0: 60, c0: 4, r1: 62, c1: 4 }], {
    ranges: [], type: 'data_bar', op: '', value1: '', value2: '', percent: false, below: false,
    color: '', fill: '', bold: false, italic: false, colors: ['638EC6'], stopIfTrue: false,
  });
  for (let r = 0; r < 3; r++) await set(60 + r, 4, String((r + 1) * 10));
  const hit = await cellAt(61, 0);
  const miss = await cellAt(60, 0);
  expect('條件式格式套用', hit.style?.fill === 'FFC7CE' && miss.style?.fill !== 'FFC7CE', JSON.stringify({ hit, miss }));
  expect('資料橫條', (await cellAt(62, 4)).bar === 1 && ((await cellAt(60, 4)).bar ?? 0) < 0.5, JSON.stringify(await cellAt(60, 4)));
  await set(60, 0, '5');
  expect('值改變後條件式格式更新', (await cellAt(60, 0)).style?.fill === 'FFC7CE', JSON.stringify(await cellAt(60, 0)));

  // 47. Tables with structured references and auto-expansion.
  const rows = [['品項', '單價'], ['蘋果', '10'], ['香蕉', '20']];
  for (let r = 0; r < 3; r++) for (let c = 0; c < 2; c++) await set(60 + r, 7 + c, rows[r][c]);
  const created = await sheetApi.createTable(0, { r0: 60, c0: 7, r1: 62, c1: 8 }, true, 'TableStyleMedium2');
  await step();
  expect('建立表格', created.value === '表格1', JSON.stringify(created));
  await select(61, 9);
  await type('=[@單價]*2');
  await key('Enter');
  expect('表格往右擴充並計算 [@欄]', (await cellAt(61, 9)).v === '20' && (await sheetApi.tableAt(0, 61, 9))?.range.c1 === 9, JSON.stringify(await cellAt(61, 9)));
  await select(63, 7);
  await type('鳳梨');
  await key('Enter');
  expect('表格往下擴充', (await sheetApi.tableAt(0, 63, 7))?.range.r1 === 63, JSON.stringify(await sheetApi.tableAt(0, 63, 7)));
  await set(64, 7, '=SUM(表格1[單價])');
  expect('結構化參照加總', (await cellAt(64, 7)).v === '30', JSON.stringify(await cellAt(64, 7)));
  expect('表格標題列樣式', (await cellAt(60, 7)).style?.fill === '4472C4' && !!(await cellAt(60, 7)).style?.font.bold, JSON.stringify(await cellAt(60, 7)));

  // 48. xlsx round trip of everything above.
  const path = await benchApi.saveCopy('selftest-v06.xlsx');
  await documentApi.openPath(path);
  await step();
  await step();
  layout = await sheetApi.layout(0, 0);
  const back = await documentApi.names();
  expect('xlsx 存讀：名稱', back.some((n) => n.name === '數量'), JSON.stringify(back));
  expect('xlsx 存讀：註解', (layout.notes ?? []).some((n) => n.row === 60 && n.text === '記得核對'), JSON.stringify(layout.notes));
  expect('xlsx 存讀：資料驗證', JSON.stringify(await sheetApi.listValues(0, 61, 3)) === '["是","否"]', '');
  expect('xlsx 存讀：條件式格式', (await cellAt(61, 0)).style?.fill === 'FFC7CE', JSON.stringify(await cellAt(61, 0)));
  expect('xlsx 存讀：表格與結構化參照', (layout.tables ?? []).some((t) => t.name === '表格1') && (await cellAt(64, 7)).v === '30', JSON.stringify(layout.tables));

  // A picture of this area (table, formats, data bars, notes).
  await select(78, 9);
  await select(61, 3);
  await new Promise((r) => setTimeout(r, 300));
  const grid = sheet.getGrid() as unknown as { snapshot(): string } | undefined;
  if (grid) await benchApi.snapshot('selftest-data', grid.snapshot());
}

// v0.7: charts, floating objects, page setup and the PDF preview.
// Runs on sheet 0 of the reopened v0.6 file, from row 80.
async function outputTests(sheet: SheetDriver, h: Helpers): Promise<void> {
  const { step, expect } = h;
  const set = async (row: number, col: number, text: string) => {
    await sheetApi.setCell(0, row, col, text);
    await step();
  };
  const rows = [['月份', '收入', '支出'], ['一月', '100', '80'], ['二月', '150', '90'], ['三月', '120', '70']];
  for (let r = 0; r < rows.length; r++) for (let c = 0; c < 3; c++) await set(80 + r, c, rows[r][c]);

  // 49. A chart built from the data: series by column, names from row 1.
  const suggested = await sheetApi.chartFromSelection(0, { r0: 80, c0: 0, r1: 83, c1: 2 }, 'column');
  const series = suggested.value?.series ?? [];
  expect('從資料建立圖表', series.length === 2 && series[0].values.endsWith('$B$82:$B$84') && series[0].categories.endsWith('$A$82:$A$84'), JSON.stringify(suggested));
  const added = await sheetApi.addChart(0, 80, 4, 480, 288, { ...suggested.value!, title: '收支' });
  await step();
  const id = added.value ?? 0;
  const data = await sheetApi.chartData(0, id);
  expect('圖表資料', data.series?.[1]?.name === '支出' && data.series?.[0]?.values?.[1] === 150 && data.categories?.[2] === '三月', JSON.stringify(data));
  await set(81, 1, '200');
  expect('圖表跟著資料更新', (await sheetApi.chartData(0, id)).series?.[0]?.values?.[0] === 200, '');
  await new Promise((r) => setTimeout(r, 200));
  const objs = sheet.getObjects();
  expect('物件層顯示圖表', objs.some((o) => o.id === id && o.kind === 'chart'), JSON.stringify(objs));
  const png = sheet.chartImages()[id] ?? '';
  expect('圖表畫成圖片（列印用）', png.startsWith('data:image/png;base64,') && png.length > 2000, `${png.length} bytes`);

  // 50. Moving and resizing, then inserting rows above moves it down.
  const full = ((await sheetApi.layout(0, 0)).objects ?? []).find((x) => x.id === id)!;
  await sheetApi.updateObject(0, { ...full, row: 81, col: 5, dx: 10, dy: 4, w: 400, h: 240, chart: null }, '移動物件');
  await step();
  let layout = await sheetApi.layout(0, 0);
  let chart = (layout.objects ?? []).find((x) => x.id === id);
  expect('移動並調整圖表', chart?.row === 81 && chart.col === 5 && chart.w === 400 && chart.chart?.title === '收支', JSON.stringify(chart));
  await sheetApi.shift(0, true, 75, 2);
  await step();
  layout = await sheetApi.layout(0, 0);
  chart = (layout.objects ?? []).find((x) => x.id === id);
  expect('插入列時圖表下移、參照調整', chart?.row === 83 && !!chart.chart?.series?.[0]?.values.endsWith('$B$84:$B$86'), JSON.stringify(chart?.chart?.series));
  expect('插入列後圖表資料不變', (await sheetApi.chartData(0, id)).series?.[0]?.values?.[0] === 200, '');

  // 51. Page setup and the PDF preview served by the asset server.
  const ps = await sheetApi.pageSetup(0);
  expect('預設版面設定', ps.paper === 'A4' && !ps.landscape && ps.scale === 100, JSON.stringify(ps));
  await sheetApi.setPageSetup(0, { ...ps, landscape: true, printArea: { r0: 82, c0: 0, r1: 95, c1: 12 }, gridlines: true, headerText: '&A' });
  await step();
  const prev = await sheetApi.printPreview(0, sheet.chartImages());
  expect('產生預覽 PDF', !prev.error && (prev.value?.pages ?? 0) >= 1 && !!prev.value?.url.startsWith('/print/'), JSON.stringify(prev));
  if (prev.value) {
    const res = await fetch(prev.value.url);
    const head = new TextDecoder().decode((await res.arrayBuffer()).slice(0, 5));
    expect('預覽 PDF 可以載入', res.ok && res.headers.get('content-type') === 'application/pdf' && head === '%PDF-', `${res.status} ${res.headers.get('content-type')} ${head}`);
  }

  // 52. xlsx round trip of the chart and page setup.
  const path = await benchApi.saveCopy('selftest-v07.xlsx');
  await documentApi.openPath(path);
  await step();
  await step();
  layout = await sheetApi.layout(0, 0);
  const back = (layout.objects ?? []).find((x) => x.kind === 'chart');
  expect('xlsx 存讀：圖表', back?.row === 83 && back.chart?.type === 'column' && back.chart.series?.length === 2 && back.chart.title === '收支', JSON.stringify(back));
  const ps2 = await sheetApi.pageSetup(0);
  expect('xlsx 存讀：版面設定', ps2.landscape && ps2.printArea?.r0 === 82 && ps2.headerText === '&A', JSON.stringify(ps2));

  // The chart as drawn (objects are DOM, not part of the grid snapshot).
  await new Promise((r) => setTimeout(r, 300));
  const drawn = Object.values(sheet.chartImages())[0];
  if (drawn) await benchApi.snapshot('selftest-chart', drawn);
}
