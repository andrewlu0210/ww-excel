// Read-only display checks for the user-supplied workbook (WW_XLSX_FIXTURE).
import { benchApi, documentApi, settingsApi, sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function workbookCompatibilityTest(driver: ReturnType<typeof Sheet>, path: string): Promise<{ markdown: string; failed: number }> {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail = '') => checks.push({ name, ok, detail });
  const step = async () => { await driver.whenIdle(); await new Promise((r) => setTimeout(r, 100)); };
  const settings = await settingsApi.get();
  await settingsApi.set({ ...settings, appearance: 'light' });
  await documentApi.openPath(path);
  await step(); await step();
  const state = await documentApi.state();
  check('活頁簿讀取八張工作表', state.sheets?.length === 8, JSON.stringify(state.sheets));

  const select = async (i: number) => {
    const tab = document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[i];
    if (!tab) throw new Error(`Missing worksheet tab ${i}`);
    tab.dispatchEvent(new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true }));
    window.dispatchEvent(new PointerEvent('pointerup'));
    await step(); await step();
    await document.fonts.ready;
    await new Promise((r) => setTimeout(r, 200));
  };
  for (let i = 0; i < (state.sheets?.length ?? 0); i++) {
    await select(i);
    const shape = driver.getObjects().find((o) => o.shape);
    const text = shape?.shape?.runs?.map((r) => r.text).join('') ?? '';
    check(`工作表 ${i + 1}：標題完整載入`, text.trim() === '组织绩效设定与评估', text);
    check(`工作表 ${i + 1}：圓角與主題色`, shape?.shape?.geometry === 'roundRect' && shape.shape.fill === 'DCEAF7' && shape.shape.runs?.at(-1)?.font.color === '163E64', JSON.stringify(shape?.shape));
  }

  await select(3);
  driver.getGrid()?.scrollToOrigin();
  await step();
  await benchApi.snapshot('xlsx-compatible-sheet4', driver.snapshotWithObjects());
  const shapeCanvas = document.querySelector<HTMLCanvasElement>('.object.readonly canvas');
  const ctx = shapeCanvas?.getContext('2d');
  if (ctx && shapeCanvas) {
    const pixels = ctx.getImageData(0, 0, shapeCanvas.width, shapeCanvas.height).data;
    let textPixels = 0;
    for (let p = 0; p < pixels.length; p += 4) if (pixels[p] < 80 && pixels[p + 1] < 130 && pixels[p + 2] < 180 && pixels[p + 3] > 200) textPixels++;
    check('標題 Canvas 實際畫出深藍文字', textPixels > 300, `${textPixels} pixels`);
  } else check('標題 Canvas 存在', false);

  for (let row = 3; row <= 5; row++) {
    const blocks = await sheetApi.getBlocks(3, [{ row: 0, col: 0 }], 0);
    const cell = blocks.blocks?.[0]?.cells?.find((c) => c.r === row && c.c === 2);
    check(`C${row + 1}：內文保留淡藍色區段`, cell?.runs?.length === 2 && cell.runs[1].font.color === '00B0F0' && cell.runs[0].font.color !== '00B0F0', JSON.stringify(cell?.runs));
    const grid = driver.getGrid()!;
    grid.scrollIntoView({ row, col: 2 }); await step();
    const rect = grid.cellRect({ row, col: 2 });
    const canvas = document.querySelector<HTMLCanvasElement>('.grid-host > canvas')!;
    const dpr = window.devicePixelRatio || 1;
    const x = Math.max(0, Math.ceil(rect.x * dpr)), y = Math.max(0, Math.ceil(rect.y * dpr));
    const w = Math.max(0, Math.min(canvas.width - x, Math.floor(rect.w * dpr))), h = Math.max(0, Math.min(canvas.height - y, Math.floor(rect.h * dpr)));
    const pixels = w && h ? canvas.getContext('2d')!.getImageData(x, y, w, h).data : [];
    let blue = 0;
    for (let p = 0; p < pixels.length; p += 4) if (pixels[p] < 80 && pixels[p + 1] > 120 && pixels[p + 2] > 190 && pixels[p + 2] > pixels[p + 1] + 20) blue++;
    check(`C${row + 1}：Canvas 實際畫出淡藍文字`, blue > 8, `${blue} blue pixels`);
  }
  driver.getGrid()?.scrollToOrigin(); await step();
  const final = await documentApi.state();
  check('測試未修改原文件內容', !final.dirty, JSON.stringify(final));
  const failed = checks.filter((c) => !c.ok).length;
  return { failed, markdown: `# Workbook display compatibility\n\n- 結果：${checks.length - failed} / ${checks.length} 通過\n\n| | 項目 | 細節 |\n| --- | --- | --- |\n` + checks.map((c) => `| ${c.ok ? '✅' : '❌'} | ${c.name} | ${c.detail.replace(/\|/g, '\\|').replace(/\n/g, '\\n')} |`).join('\n') + '\n' };
}
