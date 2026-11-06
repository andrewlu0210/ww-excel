// Read-only window geometry checks for test8.xlsx.
import { benchApi, documentApi } from '../bridge/sheet';

export async function windowWorkbookSelfTest(path: string, step: () => Promise<void>) {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail: unknown) => checks.push({name,ok,detail:JSON.stringify(detail)});
  const area = (await benchApi.windowScreen()).WorkArea;
  const size = await benchApi.windowSize();
  const position = await benchApi.windowPosition();
  const width = Math.min(930, area.Width), height = Math.min(884, area.Height);
  const expected = {x:Math.max(area.X, Math.min(0, area.X + area.Width - width)),y:Math.max(area.Y, Math.min(33, area.Y + area.Height - height))};
  check('Retina 可用範圍與 WebView 邏輯點一致', area.Width === window.screen.availWidth && area.Height === window.screen.availHeight, {area,width:window.screen.availWidth,height:window.screen.availHeight});
  check('test8 開檔大小', size.width === width && size.height === height, {size,width,height});
  check('test8 左上角位置不額外下移', position.x === expected.x && position.y === expected.y, {position,expected});
  await benchApi.setWindowPosition(expected.x + 100, expected.y + 50);
  await step();
  await documentApi.openPath(path); await step(); await step();
  const reopened = await benchApi.windowPosition();
  check('重新開檔恢復記錄的位置', reopened.x === expected.x && reopened.y === expected.y, {reopened,expected});
  check('未修改原文件', !(await documentApi.state()).dirty, await documentApi.state());
  const failed = checks.filter(c => !c.ok).length;
  return {failed,markdown:`# Window geometry: test8.xlsx\n\n- 結果：${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`- ${c.ok?'✅':'❌'} ${c.name}：${c.detail}`).join('\n')+'\n'};
}
