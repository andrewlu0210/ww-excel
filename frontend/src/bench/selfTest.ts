// Scripted editing self-test (WW_SELFTEST=1). Drives the real sheet
// controller with simulated key events and typed text, then checks the
// Go model (source of truth) and the frontend cache. Native IME input
// and menu accelerators cannot be simulated here; test those by hand.
import { documentApi, sheetApi } from '../bridge/sheet';
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
