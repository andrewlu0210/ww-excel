import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

// Exercise the actual editor/controller, backend diff, cache and canvas together.
export async function richTextWorkbookSelfTest(driver: ReturnType<typeof Sheet>): Promise<{ markdown: string; failed: number }> {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail = '') => checks.push({ name, ok, detail });
  const idle = async () => {
    await driver.whenIdle();
    await document.fonts.ready;
    await new Promise((resolve) => setTimeout(resolve, 200));
  };
  const verify = async (label: string, body: string) => {
    await idle();
    const blocks = await sheetApi.getBlocks(0, [{ row: 0, col: 0 }], 0);
    const cell = blocks.blocks?.[0]?.cells?.find((c) => c.r === 0 && c.c === 0);
    const runs = cell?.runs ?? [];
    check(`${label}：後端保留紅色粗體標題與黑色內文`,
      cell?.v === `標題${body}` && runs.length === 2 &&
      runs[0].text === '標題' && runs[0].font.color === 'FF0000' && runs[0].font.bold === true &&
      runs[1].text === body && runs[1].font.color === '000000' && !runs[1].font.bold,
      JSON.stringify(cell));
    const grid = driver.getGrid();
    const cached = grid?.cache.cell(0, 0);
    check(`${label}：前端快取保留分段格式`, JSON.stringify(cached?.runs) === JSON.stringify(runs), JSON.stringify(cached));
    const canvas = document.querySelector<HTMLCanvasElement>('.grid-host > canvas');
    const rect = grid?.cellRect({ row: 0, col: 0 });
    let red = 0;
    if (canvas && rect) {
      const dpr = window.devicePixelRatio || 1;
      const pixels = canvas.getContext('2d')!.getImageData(
        Math.ceil(rect.x * dpr), Math.ceil(rect.y * dpr), Math.floor(rect.w * dpr), Math.floor(rect.h * dpr),
      ).data;
      for (let i = 0; i < pixels.length; i += 4) {
        if (pixels[i] > 150 && pixels[i + 1] < 100 && pixels[i + 2] < 100 && pixels[i + 3] > 200) red++;
      }
    }
    check(`${label}：Canvas 實際繪製紅色標題`, red > 20, `${red} red pixels`);
  };
  const replaceBody = async (body: string, bar = false) => {
    driver.clickCell({ row: 0, col: 0 }, 'down');
    driver.clickCell({ row: 0, col: 0 }, 'up');
    driver.clickCell({ row: 0, col: 0 }, 'dbl');
    await idle();
    check('雙擊進入 A1 編輯', driver.getEditing()?.pos.row === 0 && driver.getEditing()?.pos.col === 0);
    if (bar) {
      driver.focusBar();
      await idle();
    }
    const input = document.activeElement;
    if (!(input instanceof HTMLTextAreaElement)) throw new Error('Rich-text editor is not focused');
    input.setSelectionRange(2, input.value.length);
    driver.typeText(body);
    driver.pressKey('Enter');
    await idle();
  };

  await verify('匯入', '內文');
  await benchApi.snapshot('richtext-before', driver.snapshotWithObjects());
  await replaceBody('說明');
  await verify('雙擊修改內文', '說明');
  await benchApi.snapshot('richtext-after-cell-edit', driver.snapshotWithObjects());
  await driver.runMenu('undo');
  await verify('復原', '內文');
  await driver.runMenu('redo');
  await verify('重做', '說明');
  await replaceBody('補充🙂', true);
  await verify('公式列修改內文', '補充🙂');
  for (const name of ['richtext-edited.xlsx', 'richtext-edited.wwx', 'richtext-from-native.xlsx']) {
    const path = await benchApi.saveCopy(name);
    await documentApi.openPath(path);
    await verify(`另存重開 ${name}`, '補充🙂');
  }
  await benchApi.snapshot('richtext-after-roundtrip', driver.snapshotWithObjects());
  const failed = checks.filter((c) => !c.ok).length;
  return {
    failed,
    markdown: ['# 富文字桌面編輯回歸測試', '', `${checks.length - failed} / ${checks.length} 通過`, '',
      ...checks.map((c) => `${c.ok ? '✅' : '❌'} ${c.name}: ${c.detail}`), ''].join('\n'),
  };
}
