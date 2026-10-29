import type { SheetDriver } from './selfTest';

export async function ribbonSelfTest(sheet: SheetDriver, step: () => Promise<void>, expect: (name: string, ok: boolean, detail: string) => void) {
  const ribbon = document.querySelector<HTMLElement>('.ribbon')!;
  const tabs = [...ribbon.querySelectorAll<HTMLButtonElement>('[role="tab"]')];
  expect('Ribbon：六個分頁', tabs.length === 6, String(tabs.length));
  for (let i = 0; i < tabs.length; i++) {
    tabs[i].click(); await step();
    const panels = [...ribbon.querySelectorAll<HTMLElement>('[role="tabpanel"]')];
    expect(`Ribbon：${tabs[i].textContent} 顯示正確面板`, tabs[i].getAttribute('aria-selected') === 'true' && panels.filter(p => !p.hidden).length === 1 && !panels[i].hidden, `active=${i}`);
  }
  tabs[0].click(); await step();
  tabs[0].dispatchEvent(new KeyboardEvent('keydown', {key:'ArrowRight', bubbles:true})); await step();
  expect('Ribbon：方向鍵切換並移動焦點', tabs[1].getAttribute('aria-selected') === 'true' && document.activeElement === tabs[1], String(document.activeElement?.id));
  tabs[1].dispatchEvent(new KeyboardEvent('keydown', {key:'Home', bubbles:true})); await step();
  expect('Ribbon：Home 返回常用', tabs[0].getAttribute('aria-selected') === 'true', 'Home');
  const originalWidth = ribbon.style.width;
  ribbon.style.width = '620px'; await step();
  const groups = [...ribbon.querySelectorAll<HTMLElement>('.ribbon-group')];
  expect('Ribbon：窄視窗群組不超出邊界', groups.every(g => g.getBoundingClientRect().right <= ribbon.getBoundingClientRect().right + 1), groups.map(g => Math.round(g.getBoundingClientRect().right)).join(','));
  ribbon.style.width = originalWidth; await step();
  const bold = ribbon.querySelector<HTMLButtonElement>('.b')!;
  sheet.focus();
  const keep = !bold.dispatchEvent(new PointerEvent('pointerdown', {bubbles:true,cancelable:true}));
  expect('Ribbon：格式按鈕保留工作表焦點', keep && !ribbon.contains(document.activeElement), String(document.activeElement?.tagName));
  const caret = ribbon.querySelector<HTMLButtonElement>('.font-group .caret')!;
  caret.click(); await step();
  const popup = ribbon.querySelector<HTMLElement>('.color-picker');
  expect('Ribbon：色彩選單可以開啟', popup !== null || ribbon.querySelector('[role="dialog"]') !== null, 'font color');
  caret.click(); await step();
  sheet.focus();
}
