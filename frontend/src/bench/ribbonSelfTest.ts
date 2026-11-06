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
  const originalHeight = ribbon.getBoundingClientRect().height;
  ribbon.style.width = '620px'; await step();
  const viewport = ribbon.querySelector<HTMLElement>('#ribbon-panel-0 .ribbon-viewport')!;
  const scroller = viewport.parentElement!;
  const groups = [...ribbon.querySelectorAll<HTMLElement>('.ribbon-group')];
  expect('Ribbon：窄視窗維持高度', Math.abs(ribbon.getBoundingClientRect().height - originalHeight) < 1, `${originalHeight} / ${ribbon.getBoundingClientRect().height}`);
  expect('Ribbon：群組維持同一列並可水平捲動', groups.every(g => Math.abs(g.getBoundingClientRect().top - groups[0].getBoundingClientRect().top) < 1) && viewport.scrollWidth > viewport.clientWidth, `${viewport.clientWidth} / ${viewport.scrollWidth}`);
  const bounds = viewport.getBoundingClientRect();
  const controls = [...viewport.querySelectorAll<HTMLElement>('button, select')];
  expect('Ribbon：所有控制項保持在固定高度內', controls.every(c => c.getBoundingClientRect().top >= bounds.top && c.getBoundingClientRect().bottom <= bounds.bottom + 1), controls.filter(c => c.getBoundingClientRect().bottom > bounds.bottom + 1).map(c => c.title || c.textContent).join(','));
  const right = scroller.querySelector<HTMLButtonElement>('.scroll-arrow.right');
  expect('Ribbon：溢出時顯示向右按鈕', right !== null, '620px');
  const buttonAtCenter = (button: HTMLButtonElement | null) => {
    if (!button) return null;
    const rect = button.getBoundingClientRect();
    return document.elementFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2);
  };
  expect('Ribbon：向右按鈕位於工具列上方且可點選', right !== null && buttonAtCenter(right) === right, buttonAtCenter(right)?.outerHTML ?? 'missing');
  (buttonAtCenter(right) as HTMLElement | null)?.click(); await new Promise(r => setTimeout(r, 400)); await step();
  expect('Ribbon：向右按鈕捲動並顯示向左按鈕', viewport.scrollLeft > 0 && !!scroller.querySelector('.scroll-arrow.left'), String(viewport.scrollLeft));
  const left = scroller.querySelector<HTMLButtonElement>('.scroll-arrow.left');
  expect('Ribbon：向左按鈕位於工具列上方且可點選', left !== null && buttonAtCenter(left) === left, buttonAtCenter(left)?.outerHTML ?? 'missing');
  (buttonAtCenter(left) as HTMLElement | null)?.click();
  await new Promise(r => setTimeout(r, 400)); await step();
  expect('Ribbon：向左按鈕返回起點', viewport.scrollLeft < 1 && !scroller.querySelector('.scroll-arrow.left'), String(viewport.scrollLeft));
  viewport.dispatchEvent(new WheelEvent('wheel', { deltaY: 100, bubbles: true, cancelable: true })); await step();
  expect('Ribbon：滑鼠滾輪水平捲動', viewport.scrollLeft > 0, String(viewport.scrollLeft));
  await new Promise(r => setTimeout(r, 200)); await step();
  const stableLeft = scroller.querySelector<HTMLButtonElement>('.scroll-arrow.left');
  const stableRight = scroller.querySelector<HTMLButtonElement>('.scroll-arrow.right');
  expect('Ribbon：捲動停止後雙向按鈕保持可見', stableLeft !== null && stableRight !== null && buttonAtCenter(stableLeft) === stableLeft && buttonAtCenter(stableRight) === stableRight, String(viewport.scrollLeft));
  viewport.scrollLeft = 0; await step();
  // The palette must escape the ribbon clipping area at narrow widths.
  const narrowCaret = ribbon.querySelector<HTMLButtonElement>('.font-group .caret')!;
  narrowCaret.click(); await step();
  const floating = document.querySelector<HTMLElement>('.toolbar-popup [role="dialog"]');
  const paletteBounds = floating?.getBoundingClientRect();
  const hit = paletteBounds && document.elementFromPoint(paletteBounds.left + paletteBounds.width / 2, paletteBounds.bottom - 10);
  expect('Ribbon：窄視窗色盤未被裁切且可點選', !!floating && !!hit && floating.contains(hit), JSON.stringify(paletteBounds));
  viewport.scrollLeft = 100; await step();
  expect('Ribbon：捲動時關閉浮動選單', !document.querySelector('.toolbar-popup'), 'scroll');
  viewport.scrollLeft = 0; await step();
  narrowCaret.click(); await step();
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await step();
  expect('Ribbon：Escape 關閉浮動選單', !document.querySelector('.toolbar-popup'), 'Escape');
  viewport.scrollLeft = viewport.scrollWidth; await step();
  expect('Ribbon：最右側功能可捲入且隱藏向右按鈕', !scroller.querySelector('.scroll-arrow.right'), String(viewport.scrollLeft));
  ribbon.querySelector<HTMLButtonElement>('.editing .menu-host button')!.click(); await step();
  const clear = document.querySelector<HTMLElement>('.toolbar-popup [role="menu"]');
  const clearBounds = clear?.getBoundingClientRect();
  const clearHit = clearBounds && document.elementFromPoint(clearBounds.left + clearBounds.width / 2, clearBounds.bottom - 10);
  expect('Ribbon：清除選單未被裁切且可點選', !!clear && !!clearHit && clear.contains(clearHit), JSON.stringify(clearBounds));
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await step();
  viewport.scrollLeft = 0; await step();
  for (let i = 1; i < tabs.length; i++) {
    tabs[i].click(); await step();
    expect(`Ribbon：${tabs[i].textContent} 窄視窗維持高度`, Math.abs(ribbon.getBoundingClientRect().height - originalHeight) < 1, String(ribbon.getBoundingClientRect().height));
  }
  tabs[0].click(); await step();
  ribbon.style.width = originalWidth; await step();
  const bold = ribbon.querySelector<HTMLButtonElement>('.b')!;
  sheet.focus();
  const keep = !bold.dispatchEvent(new PointerEvent('pointerdown', {bubbles:true,cancelable:true}));
  expect('Ribbon：格式按鈕保留工作表焦點', keep && !ribbon.contains(document.activeElement), String(document.activeElement?.tagName));
  const caret = ribbon.querySelector<HTMLButtonElement>('.font-group .caret')!;
  caret.click(); await step();
  const popup = document.querySelector<HTMLElement>('.toolbar-popup [role="dialog"]');
  expect('Ribbon：色彩選單可以開啟', popup !== null, 'font color');
  caret.click(); await step();
  sheet.focus();
}
