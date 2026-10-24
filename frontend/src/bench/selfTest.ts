// Scripted editing self-test (WW_SELFTEST=1). Drives the real sheet
// controller with simulated key events and typed text, then checks the
// Go model (source of truth) and the frontend cache. Native IME input
// and menu accelerators cannot be simulated here; test those by hand.
import { benchApi, documentApi, sheetApi, type FormatPatch } from '../bridge/sheet';
import { styleStore } from '../stores/styles';
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
