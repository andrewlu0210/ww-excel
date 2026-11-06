// Opt-in native cancellation checks with a generated million-cell fixture.
import { benchApi, documentApi, sheetApi, type FileLoading } from '../bridge/sheet';

export async function importSecuritySelfTest(path: string) {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail: unknown = '') => checks.push({ name, ok, detail: JSON.stringify(detail) });
  const wait = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));
  await sheetApi.setCell(0, 0, 0, '保留未儲存內容');
  const before = await documentApi.state();
  const events: FileLoading[] = [];
  const off = documentApi.onLoading(state => events.push(state));
  try {
    const outcome = documentApi.openPath(path).then(() => 'opened', () => 'cancelled');
    let button: HTMLButtonElement | null = null;
    for (let i = 0; i < 100; i++) {
      button = document.querySelector<HTMLButtonElement>('dialog.import-progress[open] button');
      if (button) break;
      await wait(20);
    }
    check('匯入顯示可操作的取消視窗', !!button);
    const loading = await documentApi.opening();
    check('後端回報正確的匯入狀態', loading.active && loading.name === path.split('/').at(-1), loading);
    if (button) button.click(); else await documentApi.cancelOpen();
    check('取消可中斷正在執行的 binding', await outcome === 'cancelled');
    await wait(100);
    check('取消視窗在結束後移除', !document.querySelector('dialog.import-progress[open]'));
    check('取消後文件及復原狀態保留', JSON.stringify(await documentApi.state()) === JSON.stringify(before), await documentApi.state());
    check('未儲存儲存格保留', await sheetApi.editText(0, 0, 0) === '保留未儲存內容');
    check('匯入狀態正確結束', !(await documentApi.opening()).active && events.some(state => state.active) && events.some(state => !state.active), events);
    await sheetApi.setCell(0, 1, 0, '取消後仍可編輯');
    check('取消後仍可編輯', await sheetApi.editText(0, 1, 0) === '取消後仍可編輯');
    const saved = await benchApi.saveCopy('import-after-cancel.xlsx');
    await documentApi.openPath(saved);
    await wait(100);
    check('取消後可以另存及重新開檔', !(await documentApi.state()).dirty && await sheetApi.editText(0, 0, 0) === '保留未儲存內容' && await sheetApi.editText(0, 1, 0) === '取消後仍可編輯');
  } finally { off(); }
  const failed = checks.filter(check => !check.ok).length;
  return { failed, markdown: `# Import security\n\n- 結果：${checks.length - failed} / ${checks.length} 通過\n\n` + checks.map(check => `- ${check.ok ? '✅' : '❌'} ${check.name}：${check.detail}`).join('\n') + '\n' };
}
