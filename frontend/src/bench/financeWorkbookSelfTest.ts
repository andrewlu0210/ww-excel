import {benchApi,documentApi,sheetApi} from '../bridge/sheet';
import {styleStore} from '../stores/styles';
import type Sheet from '../components/Sheet.svelte';
export async function financeWorkbookSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}> {
 const checks:{name:string;ok:boolean;detail:string}[]=[];
 const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
 const idle=async()=>{await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,300));};
 await idle();
 const state=await documentApi.state(),layout=await sheetApi.layout(0,0);
 check('讀取管理財務與隱藏圖表資料',state.sheets?.length===2 && state.sheets[0]==='管理財務' && state.hiddenSheets?.includes(1)===true,JSON.stringify(state));
 check('只顯示可見工作表',document.querySelectorAll('[data-sheet-tabs] [role="tab"]').length===1);
 check('隱藏格線設定正常',layout.hideGridLines===true);
 check('三個表格及篩選按鈕已載入',layout.tables?.length===3 && layout.tables.every(t=>!t.noFilter),JSON.stringify(layout.tables));
 const blocks=await sheetApi.getBlocks(0,[{row:0,col:0}],0);styleStore.merge(blocks.styles);
 const cells=blocks.blocks?.[0]?.cells??[];
 const cell=(r:number,c:number)=>cells.find(x=>x.r===r && x.c===c);
 for(const [row,want]of [[3,'NT$112,500'],[5,'NT$61,740'],[7,'NT$16,500'],[9,'NT$34,260']] as const)check(`C${row+1} 摘要公式與格式`,cell(row,2)?.v===want,JSON.stringify(cell(row,2)));
 check('55% 支出比例顯示正確',cell(10,1)?.v==='55%',JSON.stringify(cell(10,1)));
 check('收入金額保留兩位小數',cell(13,2)?.v==='NT$75,000.00',JSON.stringify(cell(13,2)));
 check('表格文字保留灰綠色與粗體',styleStore.get(cell(13,1)?.st??0)?.dto.font.bold===true && !!styleStore.get(cell(13,1)?.st??0)?.dto.font.color && styleStore.get(cell(13,1)?.st??0)?.dto.font.color!=='91BD30',JSON.stringify(styleStore.get(cell(13,1)?.st??0)?.dto.font));
 const chart=driver.getObjects().find(o=>o.chart);
 check('圓餅圖類型正確',chart?.chart?.type==='pie',JSON.stringify(chart));
 if(chart){
  const data=await sheetApi.chartData(0,chart.id),values=data.series?.[0]?.values??[];
  check('圖表資料比例正確',values.length===2 && Math.abs((values[0]??0)-.4512)<1e-9 && Math.abs((values[1]??0)-.5488)<1e-9,JSON.stringify(values));
  const g=driver.getGrid();
  if(g){const anchor=g.cellRect({row:chart.row,col:chart.col}),summary=g.cellRect({row:3,col:2});const scale=Number(document.querySelector('.zoom .percentage')?.textContent?.replace('%','')??100)/100;const chartRight=anchor.x+(chart.dx+chart.w)*scale;check('圖表不遮住摘要欄位文字',chartRight<=summary.x+1,JSON.stringify({chartRight,summaryLeft:summary.x,overlap:chartRight-summary.x,columnWidth:g.colWidth(1),sourceChartWidth:chart.w}));}
  check('第二區塊保留黑色',chart.chart?.series?.[0]?.pointColors?.['1']==='000000',JSON.stringify(chart.chart?.series?.[0]?.pointColors));
  const canvas=document.querySelector<HTMLCanvasElement>(`[data-object-id="${chart.id}"] canvas`);
  if(canvas){
   const pixels=canvas.getContext('2d')!.getImageData(0,0,canvas.width,canvas.height).data;
   let green=0,shadow=0,x0=Infinity,x1=-Infinity,y0=Infinity,y1=-Infinity;
   const ratio=canvas.width/chart.w;
   const plot=chart.chart?.plotArea;
   const radius=plot ? Math.min(plot.w*chart.w,plot.h*chart.h)/2 : Math.min(chart.w-20,chart.h-20)/2-8;
   const cx=plot ? (plot.x+plot.w/2)*chart.w : chart.w/2,cy=plot ? (plot.y+plot.h/2)*chart.h : chart.h/2;
   for(let y=0;y<canvas.height;y++)for(let x=0;x<canvas.width;x++){
    const p=(y*canvas.width+x)*4,r=pixels[p],g=pixels[p+1],b=pixels[p+2];
    if(g>r+10 && r>b+30 && g>80)green++;
    if((Math.max(r,g,b)<20 || (g>r+10 && r>b+30 && g>80)) && pixels[p+3]>200){x0=Math.min(x0,x);x1=Math.max(x1,x);y0=Math.min(y0,y);y1=Math.max(y1,y);}
    const d=Math.hypot((x+.5)/ratio-cx,(y+.5)/ratio-cy);
    if(d>radius+3 && d<radius+9 && r>30 && r<240 && Math.abs(r-g)<3 && Math.abs(g-b)<3)shadow++;
   }
   check('圓餅第一區塊顯示原檔綠色漸層',green>100,JSON.stringify({green,pointColors:chart.chart?.series?.[0]?.pointColors}));
   check('保留原檔圓餅外陰影',shadow>20,JSON.stringify({shadow}));
   const diameter=Math.min(x1-x0+1,y1-y0+1)/canvas.width;
   const expected=.8354978354978355;
   check('依原檔手動繪圖區尺寸繪製圓餅',Math.abs(diameter-expected)<.01,JSON.stringify({actualDiameterRatio:diameter,sourceDiameterRatio:expected}));
  }
 }
 const zoom=document.querySelector('.zoom .percentage')?.textContent?.trim();
 check('開啟時沿用原檔 125% 縮放',zoom==='125%',String(zoom));
 await benchApi.snapshot('test3-open',driver.snapshotWithObjects());
 const slider=document.querySelector<HTMLInputElement>('.zoom .slider');
 if(slider){slider.value='125';slider.dispatchEvent(new Event('input',{bubbles:true}));await idle();await benchApi.snapshot('test3-at125',driver.snapshotWithObjects());}
 const grid=driver.getGrid();
 if(grid){
  grid.scrollIntoView({row:18,col:1});await idle();
  const scroller=document.querySelector<HTMLElement>('.grid-host .scroller');
  if(scroller){
   const originalSet=scroller.setPointerCapture,originalRelease=scroller.releasePointerCapture;
   // Synthetic events have no native pointer capture; other hit-testing remains unchanged.
   scroller.setPointerCapture=()=>{};scroller.releasePointerCapture=()=>{};
   try{
    const host=document.querySelector<HTMLElement>('.grid-host')!.getBoundingClientRect();const rect=grid.cellRect({row:18,col:1});
    scroller.dispatchEvent(new PointerEvent('pointerdown',{pointerId:41,button:0,buttons:1,bubbles:true,cancelable:true,clientX:host.left+rect.x+rect.w-9,clientY:host.top+rect.y+rect.h/2}));
    await idle();
    const menu=document.querySelector<HTMLElement>('.dlg.menu');const labels=menu?.textContent??'';
    check('月支出按鈕讀取月支出資料',!!menu && labels.includes('房租/房貸') && !labels.includes('收入來源 1'),labels);
    if(menu)await benchApi.snapshot('test3-expenses-filter',driver.snapshotWithObjects());
    window.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));await idle();
   }finally{scroller.setPointerCapture=originalSet;scroller.releasePointerCapture=originalRelease;}
  }
  grid.scrollToOrigin();await idle();
 }
 check('測試不修改原文件',!(await documentApi.state()).dirty);
 const copy=await benchApi.saveCopy('test3-roundtrip.xlsx');await documentApi.openPath(copy);await idle();
 const back=await sheetApi.getBlocks(0,[{row:0,col:0}],0);
 check('另存重開保留摘要公式與兩位小數',back.blocks?.[0]?.cells?.find(c=>c.r===9 && c.c===2)?.v==='NT$34,260' && back.blocks?.[0]?.cells?.find(c=>c.r===13 && c.c===2)?.v==='NT$75,000.00');
 check('另存重開保留隱藏工作表',(await documentApi.state()).hiddenSheets?.includes(1)===true);
 const failed=checks.filter(c=>!c.ok).length;
 return {failed,markdown:['# test3.xlsx 相容性測試','',`${checks.length-failed} / ${checks.length} 通過`,'',...checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`),''].join(String.fromCharCode(10))};
}
