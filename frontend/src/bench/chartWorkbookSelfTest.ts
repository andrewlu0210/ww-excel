// Read-only display checks for test5.xlsx and its imported Excel charts.
import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function chartWorkbookSelfTest(driver: ReturnType<typeof Sheet>): Promise<{ markdown: string; failed: number }> {
  const checks: { name: string; ok: boolean; detail: string }[] = [];
  const check = (name: string, ok: boolean, detail = '') => checks.push({ name, ok, detail });
  const state = await documentApi.state();
  const index = state.sheets?.indexOf('9.圖表') ?? -1;
  const tab = document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[index];
  if (!tab) throw new Error('Missing chart worksheet');
  tab.dispatchEvent(new PointerEvent('pointerdown', { button: 0, bubbles: true, cancelable: true }));
  window.dispatchEvent(new PointerEvent('pointerup'));
  await driver.whenIdle();
  await document.fonts.ready;
  await new Promise(r => setTimeout(r, 500));
  driver.getGrid()?.scrollToOrigin();
  await driver.whenIdle();
  const charts = driver.getObjects().filter(o => o.chart);
  check('讀取兩張 Excel 圖表', charts.length === 2, JSON.stringify(charts.map(o => o.chart)));
  const shapeTexts = driver.getObjects().flatMap(o => o.shape ? [o.shape.runs?.map(r => r.text).join('') ?? ''] : []);
  check('水平與垂直軸說明保留原檔換行', ['水平座標軸\n(類別座標軸)', '垂直座標軸\n(數值座標軸)'].every(t => shapeTexts.includes(t)));
  const first = charts[0];
  if (!first?.chart) throw new Error('Missing attendance chart');
  const data = await sheetApi.chartData(index, first.id);
  check('日期類別顯示 2012–2017', JSON.stringify(data.categories) === JSON.stringify(['2012','2013','2014','2015','2016','2017']), JSON.stringify(data.categories));
  check('六筆會議出席數據正確', JSON.stringify(data.series?.[0]?.values) === JSON.stringify([20000,15000,4000,18000,10000,8000]));
  check('數值軸為 0–25000、每格 5000', data.valueScale?.min === 0 && data.valueScale.max === 25000 && data.valueScale.step === 5000, JSON.stringify(data.valueScale));
  check('自動圖表標題與原檔配色', first.chart.autoTitle === true && data.series?.[0]?.name === '會議出席' && first.chart.series?.[0]?.color === '5B9BD5');
  check('柱距保留 219%', first.chart.gapWidth === 219);
  check('日期軸保留 11pt 與淺灰軸線', first.chart.categoryAxis?.fontSize === 11 && first.chart.categoryAxis.lineColor === 'D9D9D9' && !first.chart.categoryAxis.hidden);
  for (const chart of charts) {
    driver.getGrid()?.scrollIntoView({ row: chart.row, col: chart.col });
    await driver.whenIdle();
    await new Promise(r => setTimeout(r, 250));
    const canvas = document.querySelector<HTMLCanvasElement>(`[data-object-id="${chart.id}"] canvas`);
    check(`圖表 ${chart.id} Canvas 已繪製`, !!canvas && canvas.width > 0 && canvas.height > 0);
    if (canvas) await benchApi.snapshot(`test5-chart-${chart.id}`, canvas.toDataURL());
  }
  const combo=charts.find(o=>o.chart?.type==='combo');
  check('組合圖包含柱狀與折線數列',!!combo && combo.chart?.series?.length===2 && combo.chart.series[0].type==='column' && combo.chart.series[1].type==='line' && combo.chart.series[1].secondary===true);
  if(combo?.chart){
    const mixed=await sheetApi.chartData(index,combo.id);
    check('兩組數列名稱與數據完整',mixed.series?.[0]?.name==='會議出席' && mixed.series?.[1]?.name==='銷售' && JSON.stringify(mixed.series[1].values)===JSON.stringify([5000,11200,30000,25000,5000,8000]));
    check('左右軸各自計算範圍與格式',mixed.valueScale?.max===25000 && mixed.secondaryScale?.max===35000 && mixed.secondaryScale.labels?.at(-1)?.trim()==='NT$35,000',JSON.stringify([mixed.valueScale,mixed.secondaryScale]));
    const canvas=document.querySelector<HTMLCanvasElement>(`[data-object-id="${combo.id}"] canvas`);
    if(canvas){
      const pixels=canvas.getContext('2d')!.getImageData(0,0,canvas.width,canvas.height).data;
      let blue=0,orange=0;
      for(let i=0;i<pixels.length;i+=4){if(pixels[i]===91 && pixels[i+1]===155 && pixels[i+2]===213)blue++;if(pixels[i]===237 && pixels[i+1]===125 && pixels[i+2]===49)orange++;}
      check('Canvas 實際繪製藍色柱與橘色折線',blue>500 && orange>500,JSON.stringify({blue,orange}));
      await benchApi.snapshot('test5-combo-chart',canvas.toDataURL());
    }
    document.querySelector<HTMLElement>(`[data-object-id="${combo.id}"]`)?.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));
    await new Promise(r=>setTimeout(r,300));
    const axes=[...document.querySelectorAll<HTMLSelectElement>('[role="dialog"] select[aria-label="座標軸"]')];
    check('編輯視窗保留主次軸設定',axes.length===2 && axes[0].value==='primary' && axes[1].value==='secondary');
    [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(b=>b.textContent?.trim()==='取消')?.click();
    await new Promise(r=>setTimeout(r,150));
  }
  // Compose the actual object canvases, including offscreen axis annotations.
  const chartElement = document.querySelector<HTMLElement>(`[data-object-id="${first.id}"]`)!;
  const chartRect = chartElement.getBoundingClientRect();
  const selected = driver.getObjects().filter(o => o.id === first.id || (o.shape && (
    o.shape.runs?.some(r => r.text.includes('水平座標軸') || r.text.includes('垂直座標軸')) || ['leftBrace','rightBrace','custom'].includes(o.shape.geometry)
  ))).flatMap(o => {
    const element = document.querySelector<HTMLElement>(`[data-object-id="${o.id}"]`);
    const canvas = element?.querySelector('canvas');
    if (!element || !canvas) return [];
    const rect = element.getBoundingClientRect();
    if (rect.right < chartRect.left - 200 || rect.left > chartRect.right + 200 || rect.bottom < chartRect.top - 100 || rect.top > chartRect.bottom + 150) return [];
    return [{ canvas, rect }];
  });
  const left = Math.min(...selected.map(o => o.rect.left)) - 8;
  const top = Math.min(...selected.map(o => o.rect.top)) - 8;
  const right = Math.max(...selected.map(o => o.rect.right)) + 8;
  const bottom = Math.max(...selected.map(o => o.rect.bottom)) + 8;
  const preview = document.createElement('canvas');
  const dpr = window.devicePixelRatio || 1;
  preview.width = Math.ceil((right-left)*dpr); preview.height = Math.ceil((bottom-top)*dpr);
  const ctx = preview.getContext('2d')!;
  ctx.scale(dpr,dpr); ctx.fillStyle = '#ffffff'; ctx.fillRect(0,0,right-left,bottom-top);
  for (const o of selected) ctx.drawImage(o.canvas,o.rect.left-left,o.rect.top-top,o.rect.width,o.rect.height);
  await benchApi.snapshot('test5-attendance-comparison', preview.toDataURL());
  driver.getGrid()?.scrollToOrigin();
  await driver.whenIdle();
  await new Promise(r => setTimeout(r, 250));
  await benchApi.snapshot('test5-chart-sheet', driver.snapshotWithObjects());
  check('測試未修改原文件', !(await documentApi.state()).dirty);
  if(combo?.chart){
    await benchApi.saveCopy('test5-combo-copy.xlsx');
    await sheetApi.setCell(index,67,5,'60000');await driver.whenIdle();
    const changed=await sheetApi.chartData(index,combo.id);
    check('修改來源後兩軸維持獨立並更新折線',changed.series?.[1]?.values?.[0]===60000 && changed.valueScale?.max===25000 && changed.secondaryScale?.max===70000);
    await sheetApi.undo();await driver.whenIdle();
    check('復原恢復折線原始值',(await sheetApi.chartData(index,combo.id)).series?.[1]?.values?.[0]===5000);
    await sheetApi.redo();await driver.whenIdle();
    check('重做恢復折線修改',(await sheetApi.chartData(index,combo.id)).series?.[1]?.values?.[0]===60000);
    await sheetApi.undo();await driver.whenIdle();
    check('調整組合圖大小成功',await sheetApi.updateObject(index,{...combo,w:combo.w+20},'調整圖表大小')===null);
    const copy=await benchApi.saveCopy('test5-combo-rewritten.xlsx',true);
    await documentApi.openPath(copy);await driver.whenIdle();await new Promise(r=>setTimeout(r,250));
    const back=(await sheetApi.layout(index,0)).objects?.find(o=>o.chart?.type==='combo');
    const backData=back?await sheetApi.chartData(index,back.id):undefined;
    check('XLSX 重寫重開保留組合圖與兩軸',back?.chart?.series?.length===2 && back.chart.series[1].secondary===true && backData?.secondaryScale?.labels?.at(-1)?.trim()==='NT$35,000');
    const native=await benchApi.saveCopy('test5-combo-native.wwx');
    await documentApi.openPath(native);await driver.whenIdle();await new Promise(r=>setTimeout(r,250));
    const nativeChart=(await sheetApi.layout(index,0)).objects?.find(o=>o.chart?.type==='combo');
    check('WWX 重開保留折線樣式與座標軸',nativeChart?.chart?.series?.[1]?.marker==='none' && nativeChart.chart.series[1].color==='ED7D31' && !!nativeChart.chart.secondaryAxis);
  }
  const failed = checks.filter(c => !c.ok).length;
  return { failed, markdown: '# test5.xlsx 圖表相容性檢查\n\n' + `${checks.length-failed} / ${checks.length} 通過\n\n` + checks.map(c => `${c.ok ? '✅' : '❌'} ${c.name}: ${c.detail}`).join('\n') + '\n' };
}
