// Scripted editing self-test (WW_SELFTEST=1). Drives the real sheet
// controller with simulated key events and typed text, then checks the
// Go model (source of truth) and the frontend cache. Native IME input
// and menu accelerators cannot be simulated here; test those by hand.
import { sheetApi } from '../bridge/sheet';
import type { CellPos, Selection } from '../grid/selection';
import { cellAddress } from '../grid/layout';
import type { BlockCache } from '../stores/viewportCache';

export interface SheetDriver {
  pressKey(key: string, mods?: Partial<KeyboardEventInit>): void;
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
  const expect = (name: string, ok: boolean, detail: string) => checks.push({ name, ok, detail });

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

  // Leave a multi-range selection on screen for the snapshot.
  sheet.setSelectionForTest({
    active: { row: 4, col: 0 },
    anchor: { row: 4, col: 0 },
    focus: { row: 5, col: 1 },
    ranges: [
      { r0: 0, c0: 3, r1: 1, c1: 4 },
      { r0: 4, c0: 0, r1: 5, c1: 1 },
    ],
  });
  await step();

  const failed = checks.filter((c) => !c.ok).length;
  const lines = [
    '# v0.2 Self-test',
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
