// Verify cached pivot results and custom table appearance without refreshing them.
import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import { styleStore } from '../stores/styles';
import type Sheet from '../components/Sheet.svelte';

export async function pivotWorkbookSelfTest(driver: ReturnType<typeof Sheet>): Promise<{ markdown: string; failed: number }> {
  const checks: {name:string;ok:boolean;detail:string}[] = [];
  const check = (name:string,ok:boolean,detail='') => checks.push({name,ok,detail});
  const state = await documentApi.state();
  const index = state.sheets?.indexOf('10.樞紐分析表') ?? -1;
  const tab = document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[index];
  if (!tab) throw new Error('Missing pivot worksheet');
  tab.dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));
  window.dispatchEvent(new PointerEvent('pointerup'));
  await driver.whenIdle(); await document.fonts.ready; await new Promise(r=>setTimeout(r,400));
  const blocks = await sheetApi.getBlocks(index,[{row:0,col:0}],0);
  styleStore.merge(blocks.styles);
  const cells=blocks.blocks?.[0]?.cells ?? [];
  const cell=(row:number,col:number)=>cells.find(c=>c.r===row && c.c===col);
  const style=(row:number,col:number)=>styleStore.get(cell(row,col)?.st ?? 0)?.dto;
  const layout=await sheetApi.layout(index,0);
  check('兩份來源表格載入',layout.tables?.length===2);
  check('來源表頭白字綠底',style(2,2)?.fill==='339966' && style(2,2)?.font.color==='FFFFFF',JSON.stringify(style(2,2)));
  check('交錯列為淺灰底',style(3,2)?.fill==='F2F2F2',JSON.stringify(style(3,2)));
  check('第二份表格保留相同外觀',style(33,2)?.fill==='339966' && style(34,2)?.fill==='F2F2F2');
  check('樞紐欄位標題載入',cell(10,4)?.v==='列標籤' && cell(10,5)?.v==='加總 - 金額');
  check('樞紐表頭白字綠底',style(10,4)?.fill==='359966' && style(10,4)?.font.color==='FFFFFF',JSON.stringify(style(10,4)));
  check('樞紐總計白字綠底',style(14,4)?.fill==='359966' && style(14,5)?.font.color==='FFFFFF',JSON.stringify(style(14,4)));
  check('樞紐結果保留原檔金額與格式',JSON.stringify([11,12,13,14].map(r=>cell(r,5)?.v.trim()))===JSON.stringify(['NT$2,150','NT$2,110','NT$1,690','NT$5,950']),JSON.stringify([11,12,13,14].map(r=>cell(r,5))));
  check('樞紐下拉按鈕只位於列標籤欄',layout.pivotHeaders?.length===1 && layout.pivotHeaders[0].r0===10 && layout.pivotHeaders[0].c0===4 && layout.pivotHeaders[0].c1===4);
  check('保留旁邊綠色虛線箭頭',driver.getObjects().some(o=>o.shape?.geometry==='arc' && o.shape.lineDash==='sysDot'));
  check('來源銷售金額正確',JSON.stringify([3,4,5,6,7,8].map(r=>cell(r,5)?.v.trim()))===JSON.stringify(['NT$1,400','NT$1,010','NT$750','NT$510','NT$1,600','NT$680']));
  check('日期公式維持有效日期顯示',[3,4,5,6,7,8].every(r=>/^\d{4}年\d{1,2}月\d{1,2}日$/.test(cell(r,2)?.v ?? '')));
  const zoom=document.querySelector<HTMLInputElement>('.zoom input[type="range"]');
  const previousZoom=zoom?.value;
  if(zoom){zoom.value='65';zoom.dispatchEvent(new Event('input',{bubbles:true}));}
  await driver.whenIdle();await new Promise(r=>setTimeout(r,300));
  driver.getGrid()?.scrollToOrigin();await new Promise(r=>setTimeout(r,250));
  await benchApi.snapshot('test5-pivot-top',driver.snapshotWithObjects());
  driver.getGrid()?.scrollIntoView({row:25,col:0});await driver.whenIdle();await new Promise(r=>setTimeout(r,250));
  await benchApi.snapshot('test5-pivot-lower',driver.snapshotWithObjects());
  if(zoom && previousZoom){zoom.value=previousZoom;zoom.dispatchEvent(new Event('input',{bubbles:true}));}
  await driver.whenIdle();
  check('檢查未修改原文件',!(await documentApi.state()).dirty);
  const copy=await benchApi.saveCopy('test5-pivot-roundtrip.xlsx');
  await documentApi.openPath(copy);await driver.whenIdle();await new Promise(r=>setTimeout(r,300));
  const back=await sheetApi.getBlocks(index,[{row:0,col:0}],0);
  const backCells=back.blocks?.[0]?.cells ?? [];
  const backStyle=(row:number,col:number)=>back.styles?.find(s=>s.id===backCells.find(c=>c.r===row && c.c===col)?.st);
  check('另存重開保留來源表格綠底與交錯列',backStyle(2,2)?.fill==='339966' && backStyle(3,2)?.fill==='F2F2F2');
  check('另存重開保留樞紐總計及格式',backStyle(14,5)?.fill==='359966' && backCells.find(c=>c.r===14 && c.c===5)?.v.trim()==='NT$5,950');
  check('另存重開保留樞紐下拉標記',(await sheetApi.layout(index,0)).pivotHeaders?.length===1);
  const failed=checks.filter(c=>!c.ok).length;
  return {failed,markdown:'# test5.xlsx 樞紐分析表顯示相容性\n\n'+`${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
