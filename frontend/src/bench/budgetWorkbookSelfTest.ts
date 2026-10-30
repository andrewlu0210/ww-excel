import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import { styleStore } from '../stores/styles';
import type Sheet from '../components/Sheet.svelte';

// Read-only checks against test1.xlsx; saving is restricted to a temporary copy.
export async function budgetWorkbookSelfTest(driver: ReturnType<typeof Sheet>): Promise<{markdown:string;failed:number}> {
 const checks: {name:string;ok:boolean;detail:string}[] = [];
 const check = (name:string,ok:boolean,detail='') => checks.push({name,ok,detail});
 const idle = async () => {await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,350));};
 await idle();
 const state = await documentApi.state();
 check('兩張工作表仍保留於模型',state.sheets?.length===2,JSON.stringify(state.sheets));
 check('圖表資料標記為隱藏',state.hiddenSheets?.includes(1)===true,JSON.stringify(state.hiddenSheets));
 const tabs = [...document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')];
 check('只顯示目前月份分頁',tabs.length===1 && tabs[0].textContent==='目前月份',tabs.map(t=>t.textContent).join(','));
 const layout=await sheetApi.layout(0,0);
 check('讀取原檔隱藏工作表格線設定',layout.hideGridLines===true);
 const grid=driver.getGrid();
 if(grid) {
  const image=new Image();image.src=grid.snapshot();await image.decode();
  const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;
  const ctx=canvas.getContext('2d')!;ctx.drawImage(image,0,0);
  const rect=grid.areaRect({r0:5,c0:5,r1:5,c1:6});
  const dpr=window.devicePixelRatio||1;
  const pixel=(x:number,y:number)=>Array.from(ctx.getImageData(Math.floor(x*dpr),Math.floor(y*dpr),1,1).data).join(',');
  const right=pixel(rect.x+rect.w-.5,rect.y+rect.h*.8);
  const bottom=pixel(rect.x+rect.w*.8,rect.y+rect.h-.5);
  const fill=pixel(rect.x+rect.w-4,rect.y+rect.h*.8);
  check('摘要合併儲存格右側與下側無白線',right===fill && bottom===fill,JSON.stringify({right,bottom,fill}));
 }
 const tableBlocks=await sheetApi.getBlocks(0,[{row:0,col:0}],0);
 styleStore.merge(tableBlocks.styles);
 const cellStyle=(r:number,c:number)=>styleStore.get(tableBlocks.blocks?.[0]?.cells?.find(cell=>cell.r===r && cell.c===c)?.st??0)?.dto;
 const incomeHead=cellStyle(16,1),incomeRow1=cellStyle(17,1),incomeRow2=cellStyle(18,1);
 check('月收入標題保留藍色非粗體',incomeHead?.font.color==='0592FE' && !incomeHead.font.bold,JSON.stringify(incomeHead?.font));
 check('月收入沒有預設藍色斑馬紋',incomeRow1?.fill==='D8D8D1' && incomeRow2?.fill==='D8D8D1');
 check('月收入資料列有原檔細分隔線',incomeRow1?.bottom?.style==='thin' && incomeRow2?.bottom?.style==='thin');
 check('三個表格皆開啟篩選按鈕',layout.tables?.length===3 && layout.tables.every(t=>!t.noFilter));
 const incomeFilters=await sheetApi.filterValues(0,1);
 check('月收入篩選清單可讀取三筆資料',incomeFilters.length===3,JSON.stringify(incomeFilters));
 const objects=driver.getObjects();
 const ring=objects.find(o=>o.chart?.type==='doughnut');
 const column=objects.find(o=>o.chart?.type==='column');
 check('環圈圖保留 75% 內圈',ring?.chart?.holeSize===75,JSON.stringify({type:ring?.chart?.type,holeSize:ring?.chart?.holeSize}));
 if(ring){
  const data=await sheetApi.chartData(0,ring.id);
  const values=data.series?.[0]?.values ?? [];
  check('隱藏工作表仍供應圖表資料',values.length===2 && Math.abs((values[0]??0)+(values[1]??0)-1)<1e-10,JSON.stringify(values));
  check('環圈中央僅顯示 62%',data.series?.[0]?.labels?.join(',')===',62%' && ring.chart?.series?.[0]?.labels?.points?.['1']?.center===true,JSON.stringify(data.series?.[0]?.labels));
  check('環圈使用檔案指定的兩種藍色',ring.chart?.series?.[0]?.pointColors?.['0']==='69BBFE' && ring.chart?.series?.[0]?.pointColors?.['1']==='0592FE');
  const canvas=document.querySelector<HTMLCanvasElement>(`[data-object-id="${ring.id}"] canvas`);
  const pixel=canvas?.getContext('2d')?.getImageData(Math.floor(canvas.width/2),Math.floor(canvas.height*.65),1,1).data;
  check('環圈圖背景為透明',ring.chart?.background==='transparent',String(ring.chart?.background));
  const container=document.querySelector<HTMLElement>(`[data-object-id="${ring.id}"]`);
  const style=container ? getComputedStyle(container) : null;
  check('圖表容器不覆蓋透明背景',style?.backgroundColor==='rgba(0, 0, 0, 0)' && style.boxShadow==='none',JSON.stringify({background:style?.backgroundColor,shadow:style?.boxShadow}));
  check('環圈圖內圈背景保持透明',!!pixel && pixel[3]===0,String(pixel));
 }
 if(column){
  check('柱狀圖保留主題底色',/^[0-9A-F]{6}$/.test(column.chart?.background??'') && column.chart?.background!=='FFFFFF',String(column.chart?.background));
  const data=await sheetApi.chartData(0,column.id);
  check('原檔空白類別標籤不補上 1',data.categories?.length===1 && data.categories[0]==='',JSON.stringify(data.categories));
  check('縱軸使用 NT$ 金額與 20,000 刻度',data.valueScale?.min===0 && data.valueScale.max===120000 && data.valueScale.step===20000 && data.valueScale.labels?.[6]==='NT$120,000',JSON.stringify(data.valueScale));
  check('原檔軸線及向外刻度設定已讀取',!!column.chart?.valueAxis?.lineColor && column.chart.valueAxis.majorTick==='out' && column.chart.valueAxis.gridLines===false && !!column.chart.categoryAxis?.lineColor,JSON.stringify(column.chart?.valueAxis));
  check('柱狀圖數據正確',data.series?.[0]?.values?.[0]===112500 && data.series?.[1]?.values?.[0]===70080,JSON.stringify(data.series));
  const canvas=document.querySelector<HTMLCanvasElement>(`[data-object-id="${column.id}"] canvas`);
  if(canvas){
   const pixels=canvas.getContext('2d')!.getImageData(0,0,canvas.width,canvas.height).data;
   const dpr=window.devicePixelRatio||1;
   const ranges=[[Infinity,-Infinity],[Infinity,-Infinity]];
   for(let y=0;y<canvas.height-35*dpr;y++)for(let x=0;x<canvas.width;x++){
    const p=(y*canvas.width+x)*4;
    const color=pixels[p]===5 && pixels[p+1]===146 && pixels[p+2]===254?0:pixels[p]===105 && pixels[p+1]===187 && pixels[p+2]===254?1:-1;
    if(color>=0){ranges[color][0]=Math.min(ranges[color][0],y);ranges[color][1]=Math.max(ranges[color][1],y);}
   }
   const heights=ranges.map(([a,b])=>b-a+1);
   const ratio=heights[1]/heights[0];
   check('柱高比例符合從零起算的數值比例',Math.abs(ratio-70080/112500)<.03,JSON.stringify({heights,ratio}));
  }else check('柱狀圖 Canvas 存在',false);
 }
 const blocks=await sheetApi.getBlocks(0,[{row:0,col:0}],0);
 for(const [row,want] of [[5,112500],[8,70080],[11,16500],[14,25920]]){
  const value=blocks.blocks?.[0]?.cells?.find(c=>c.r===row && c.c===5)?.v??'';
  check(`F${row+1} 公式結果正確`,Number(value.replace(/[^\d.-]/g,''))===want,value);
 }
 await benchApi.snapshot('test1-fixed',driver.snapshotWithObjects());
 if(grid){grid.scrollByCells(9,0);await idle();await benchApi.snapshot('test1-tables-fixed',driver.snapshotWithObjects());grid.scrollToOrigin();await idle();}
 check('開啟及顯示不修改文件',!(await documentApi.state()).dirty);
 const copy=await benchApi.saveCopy('test1-roundtrip.xlsx');
 await documentApi.openPath(copy);await idle();
 const back=await documentApi.state();
 check('另存副本後保留隱藏格線',(await sheetApi.layout(0,0)).hideGridLines===true);
 check('另存副本後保留隱藏分頁',back.hiddenSheets?.includes(1)===true && document.querySelectorAll('[data-sheet-tabs] [role="tab"]').length===1);
 check('另存副本後保留環圈與透明背景',driver.getObjects().some(o=>o.chart?.type==='doughnut' && o.chart.holeSize===75 && o.chart.background==='transparent'));
 const restoredRing=driver.getObjects().find(o=>o.chart?.type==='doughnut');
 const restoredData=restoredRing ? await sheetApi.chartData(0,restoredRing.id) : null;
 check('另存副本後保留中央百分比與區塊顏色',restoredData?.series?.[0]?.labels?.join(',')===',62%' && restoredRing?.chart?.series?.[0]?.labels?.points?.['1']?.center===true && restoredRing?.chart?.series?.[0]?.pointColors?.['0']==='69BBFE',JSON.stringify({labels:restoredData?.series?.[0]?.labels,center:restoredRing?.chart?.series?.[0]?.labels?.points?.['1'],colors:restoredRing?.chart?.series?.[0]?.pointColors}));
 const restoredColumn=driver.getObjects().find(o=>o.chart?.type==='column');
 const columnData=restoredColumn ? await sheetApi.chartData(0,restoredColumn.id) : null;
 check('另存副本後空白類別不變成 1',columnData?.categories?.length===1 && columnData.categories[0]==='');
 const failed=checks.filter(c=>!c.ok).length;
 return {failed,markdown:`# Budget workbook compatibility\n\n${checks.length-failed} / ${checks.length} passed\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
