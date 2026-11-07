import { benchApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

// Check real WebView clipping, hit testing and portal cleanup near sheet edges.
export async function contextMenuSelfTest(sheet: SheetDriver, step: () => Promise<void>, expect: (name: string, ok: boolean, detail: string) => void) {
  const original = await benchApi.windowSize();
  await benchApi.setWindowSize(740, 480);
  await step();
  const scroller = document.querySelector<HTMLElement>('.scroller')!;
  const open = async (x: number, y: number) => {
    scroller.dispatchEvent(new MouseEvent('contextmenu', { clientX: x, clientY: y, bubbles: true, cancelable: true }));
    await step();
    const menu = document.querySelector<HTMLUListElement>('.context-menu');
    if (!menu) throw new Error(`Context menu missing at ${x},${y}; scroller=${JSON.stringify(scroller.getBoundingClientRect())}`);
    return menu;
  };
  const bounds = scroller.getBoundingClientRect();
  try {
    let menu = await open(bounds.right - 24, bounds.bottom - 24);
    const rect = menu.getBoundingClientRect();
    expect('儲存格選單：右下角完整位於視窗內', rect.left >= 3 && rect.top >= 3 && rect.right <= innerWidth - 3 && rect.bottom <= innerHeight - 3, JSON.stringify(rect));
    expect('儲存格選單：不受工作表容器裁切', menu.parentElement === document.body && getComputedStyle(menu).position === 'fixed', menu.parentElement?.tagName ?? 'missing');
    // Also cover a menu longer than its available viewport height.
    menu.style.maxHeight = '120px';
    await step();
    menu.scrollTop = menu.scrollHeight;
    await step();
    expect('儲存格選單：捲動選單不會關閉', document.querySelector('.context-menu') === menu && menu.scrollTop > 0, String(menu.scrollTop));
    const last = menu.querySelector<HTMLButtonElement>('li:last-child button')!;
    const lastRect = last.getBoundingClientRect();
    const hit = document.elementFromPoint(lastRect.left + lastRect.width / 2, lastRect.top + lastRect.height / 2);
    expect('儲存格選單：最末項目可捲動至可點選位置', last.contains(hit), hit?.outerHTML ?? 'missing');
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
    await step();
    expect('儲存格選單：Esc 移除浮層', !document.querySelector('.context-menu'), 'Escape');
    menu = await open(bounds.left + 60, bounds.top + 35);
    const top = menu.getBoundingClientRect();
    expect('儲存格選單：上方開啟也保留視窗內邊距', top.left >= 3 && top.top >= 3 && top.bottom <= innerHeight - 3, JSON.stringify(top));
    window.dispatchEvent(new Event('resize'));
    await step();
    expect('儲存格選單：視窗變更時關閉浮層', !document.querySelector('.context-menu'), 'resize');
    await open(bounds.right - 24, bounds.bottom - 24);
    scroller.dispatchEvent(new WheelEvent('wheel', { deltaY: 100, bubbles: true }));
    await step();
    expect('儲存格選單：工作表滾輪操作時關閉浮層', !document.querySelector('.context-menu'), 'wheel');
  } finally {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await benchApi.setWindowSize(original.width, original.height);
    await step();
    sheet.focus();
  }
}
