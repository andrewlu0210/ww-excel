import { sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

// Exercise native contenteditable events rather than the draft-only test driver.
export async function formulaAssistSelfTest(driver: ReturnType<typeof Sheet>): Promise<{ markdown: string; failed: number }> {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail = '') => checks.push({ name, ok, detail });
  const step = async () => { await driver.whenIdle(); await new Promise(r => setTimeout(r, 100)); };
  const bar = document.querySelector<HTMLDivElement>('.formula')!;
  const type = async (text: string) => {
    for (const key of text) {
      bar.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
      document.execCommand('insertText', false, key);
      bar.dispatchEvent(new KeyboardEvent('keyup', { key, bubbles: true }));
      await step();
    }
  };
  const inspect = (label: string) => {
    const assist = driver.getAssist();
    const list = document.querySelector<HTMLElement>('.suggestions');
    check(`${label}: P includes PRODUCT`, assist.suggestions.includes('PRODUCT'), JSON.stringify(assist));
    const box = list?.getBoundingClientRect();
    check(`${label}: list is visible inside viewport`, !!box && box.width > 0 && box.height > 0 && box.top >= 0 && box.bottom <= innerHeight, JSON.stringify(box));
    const hit = box ? document.elementFromPoint(box.left + 10, box.top + 10) : null;
    check(`${label}: list is not covered`, !!list && !!hit && list.contains(hit), hit?.outerHTML ?? 'no hit');
  };
  check('function metadata loaded', (await sheetApi.functions()).some(f => f.name === 'PRODUCT'));
  for (const prefix of ['P', 'p']) {
    driver.pressKey('Escape');
    driver.focusBar();
    await step();
    document.execCommand('selectAll');
    await type(`=${prefix}`);
    inspect(`native =${prefix}`);
    driver.pressKey('Escape');
    driver.pressKey('Escape');
    await step();
  }
  await sheetApi.setCell(0, 0, 0, '=SUM(1,2)');
  await step();
  driver.focusBar();
  await step();
  document.execCommand('selectAll');
  await type('=P');
  inspect('replace existing formula');
  driver.pressKey('Escape');
  driver.pressKey('Escape');
  await step();

  driver.focusBar();
  await step();
  document.execCommand('selectAll');
  await type('=');
  bar.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true, data: '' }));
  document.execCommand('insertText', false, 'P');
  bar.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: 'P' }));
  await step();
  inspect('after composition commit');
  driver.pressKey('Escape');
  driver.pressKey('Escape');
  await step();

  driver.focusBar();
  await step();
  document.execCommand('selectAll');
  await type('=PRODUCT(');
  check('PRODUCT argument hint', driver.getAssist().hint === 'PRODUCT', JSON.stringify(driver.getAssist()));
  const hint = document.querySelector<HTMLElement>('.popup.hint');
  check('PRODUCT argument hint rendered', !!hint && hint.getBoundingClientRect().height > 0, hint?.textContent ?? 'no hint');
  driver.pressKey('Escape');
  await step();
  await sheetApi.setCell(0, 2, 0, 'delete this row');
  await step();
  driver.goTo({ row: 2, col: 0 });
  driver.pressKey(' ', { shiftKey: true });
  await step();
  await driver.runMenu('deleteRows');
  await step();
  driver.clickCell({ row: 0, col: 4 }, 'down');
  driver.clickCell({ row: 0, col: 4 }, 'up');
  await step();
  driver.focusBar();
  await step();
  document.execCommand('selectAll');
  await type('=P');
  inspect('after deleting entire row');
  driver.pressKey('Escape');
  driver.pressKey('Escape');
  await step();
  const scroller = document.querySelector<HTMLElement>('.scroller')!;
  const bounds = scroller.getBoundingClientRect();
  const cell = driver.getGrid()!.cellRect({ row: 2, col: 0 });
  await sheetApi.setCell(0, 2, 0, 'delete this row');
  await sheetApi.setCell(0, 3, 0, 'surviving row');
  await step();
  scroller.dispatchEvent(new MouseEvent('contextmenu', { clientX: bounds.left + cell.x + 10, clientY: bounds.top + cell.y + 5, bubbles: true, cancelable: true }));
  await step();
  const deleteCells = [...document.querySelectorAll<HTMLButtonElement>('.context-menu button')].find(b => b.textContent?.includes('刪除…'));
  check('cell context menu opened', !!deleteCells);
  deleteCells?.click();
  await step();
  const dialog = document.querySelector<HTMLElement>('[role="dialog"][aria-label="刪除"]');
  check('whole row delete dialog opened', !!dialog);
  const wholeRow = dialog?.querySelector<HTMLInputElement>('input[type="radio"]');
  wholeRow?.click();
  [...(dialog?.querySelectorAll<HTMLButtonElement>('button') ?? [])].find(b => b.textContent === '確定')?.click();
  await step();
  check('whole row actually deleted', await sheetApi.editText(0, 2, 0) === 'surviving row');
  driver.clickCell({ row: 0, col: 4 }, 'down');
  driver.clickCell({ row: 0, col: 4 }, 'up');
  await step();
  driver.focusBar();
  await step();
  document.execCommand('selectAll');
  await type('=P');
  inspect('after context menu row deletion');
  for (const key of ['ArrowDown', 'Tab']) {
    bar.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
    bar.dispatchEvent(new KeyboardEvent('keyup', { key, bubbles: true }));
    await step();
  }
  check('suggestion keyboard selection survives row deletion', bar.textContent === '=PEARSON(' && driver.getAssist().hint === 'PEARSON', bar.textContent ?? '');
  const restoredHint = document.querySelector<HTMLElement>('.popup.hint');
  check('argument hint survives row deletion', !!restoredHint && restoredHint.getBoundingClientRect().height > 0, restoredHint?.textContent ?? 'no hint');
  driver.pressKey('Escape');
  return {
    failed: checks.filter(c => !c.ok).length,
    markdown: '# Formula assist self-test\n\n' + checks.map(c => `- ${c.ok ? '✅' : '❌'} ${c.name}: ${c.detail}`).join('\n') + '\n',
  };
}
