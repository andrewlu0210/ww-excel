import { mount, tick, unmount } from 'svelte';
import SheetTabs from '../components/SheetTabs.svelte';
import { t } from '../i18n';
import { documentApi } from '../bridge/sheet';

// Exercise the rendered menu: calling the Go API directly misses derived-state errors.
export async function sheetTabsSelfTest(expect: (name: string, ok: boolean, detail: string) => void): Promise<void> {
  const target = document.createElement('div');
  document.body.append(target);
  let deleted: number | null = null;
  let moved: number[] = [];
  const props = {
    sheets: ['A', 'B', 'C'], active: 0,
    onselect: () => {}, onadd: () => {}, onrename: async () => null,
    ondelete: (i: number) => { deleted = i; },
    onmove: (from: number, to: number) => { moved = [from, to]; },
    ondone: () => {},
  };
  let component = mount(SheetTabs, { target, props });
  const open = async (i: number) => {
    target.querySelector<HTMLElement>(`[data-sheet-index="${i}"]`)!
      .dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true }));
    await tick();
  };
  const item = (label: string) => [...target.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')]
    .find(button => button.textContent === t(label))!;
  try {
    await tick();
    await open(2);
    expect('工作表選單：多張時允許刪除', !item('刪除').disabled, '3 sheets');
    item('刪除').click();
    await tick();
    expect('工作表選單：刪除右鍵選中的工作表', deleted === 2, String(deleted));
    expect('工作表選單：操作後關閉', target.querySelector('[role="menu"]') === null, 'delete');
    await open(1);
    item('左移').click();
    await tick();
    expect('工作表選單：左移', moved.join(',') === '1,0', moved.join(','));
    await open(1);
    item('右移').click();
    await tick();
    expect('工作表選單：右移', moved.join(',') === '1,2', moved.join(','));
    await unmount(component);
    component = mount(SheetTabs, { target, props: { ...props, sheets: ['A'] } });
    await tick();
    await open(0);
    expect('工作表選單：最後一張禁止刪除', item('刪除').disabled, '1 sheet');
    await unmount(component);
    component = mount(SheetTabs, { target, props: { ...props, hidden: [1, 2] } });
    await tick();
    await open(0);
    expect('工作表選單：最後一張可見工作表禁止刪除', item('刪除').disabled, '1 visible sheet');
  } finally {
    await unmount(component);
    target.remove();
  }
}

// Follow the real tab callbacks through Wails and verify both Go state and UI.
export async function sheetTabsIntegrationTest(
  step: () => Promise<void>,
  expect: (name: string, ok: boolean, detail: string) => void,
): Promise<void> {
  const before = (await documentApi.state()).sheets ?? [];
  const selected = document.querySelector<HTMLElement>('[data-sheet-tabs] [aria-selected="true"]');
  const original = Number(selected?.dataset.sheetIndex ?? 0);
  const waitForSheets = async (matches: (names: string[]) => boolean) => {
    let names: string[] = [];
    for (let i = 0; i < 20; i++) {
      await step();
      names = (await documentApi.state()).sheets ?? [];
      if (matches(names)) break;
    }
    return names;
  };
  document.querySelector<HTMLButtonElement>('[data-sheet-tabs] .add')!.click();
  let names = await waitForSheets(names => names.length === before.length + 1);
  const added = names.find(name => !before.includes(name));
  expect('工作表操作：點加號新增並顯示分頁', !!added && document.querySelectorAll('[data-sheet-index]').length === names.length, JSON.stringify(names));
  if (!added) return;
  const run = async (label: string) => {
    const index = (await documentApi.state()).sheets!.indexOf(added);
    const tab = document.querySelector<HTMLElement>(`[data-sheet-index="${index}"]`)!;
    const rect = tab.getBoundingClientRect();
    tab.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: rect.left + 8, clientY: rect.top + 8 }));
    await step();
    const button = [...document.querySelectorAll<HTMLButtonElement>('.tab-menu [role="menuitem"]')]
      .find(button => button.textContent === t(label))!;
    button.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }));
    button.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, button: 0 }));
    button.click();
  };
  await run('左移');
  names = await waitForSheets(names => names.indexOf(added) === original);
  expect('工作表操作：右鍵左移同步到 Go', names.indexOf(added) === original, JSON.stringify(names));
  await run('右移');
  names = await waitForSheets(names => names.indexOf(added) === original + 1);
  expect('工作表操作：右鍵右移同步到 Go', names.indexOf(added) === original + 1, JSON.stringify(names));
  await run('刪除');
  names = await waitForSheets(names => !names.includes(added));
  expect('工作表操作：右鍵刪除同步到 Go', JSON.stringify(names) === JSON.stringify(before), JSON.stringify(names));
  expect('工作表操作：刪除後分頁消失', document.querySelectorAll('[data-sheet-index]').length === before.length, String(document.querySelectorAll('[data-sheet-index]').length));
  const tab = document.querySelector<HTMLElement>(`[data-sheet-index="${original}"]`)!;
  tab.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }));
  tab.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, button: 0 }));
  await step();
}
