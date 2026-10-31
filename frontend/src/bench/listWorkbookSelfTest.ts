import {benchApi,documentApi,sheetApi} from '../bridge/sheet';
import {styleStore} from '../stores/styles';
import type Sheet from '../components/Sheet.svelte';
export async function listWorkbookSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}> {
 const checks:{name:string;ok:boolean;detail:string}[]=[];
 const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
 const idle=async()=>{await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,300));};
 await idle();
 const checkCells=async(prefix:string)=>{
  const blocks=await sheetApi.getBlocks(0,[{row:0,col:0}],0);styleStore.merge(blocks.styles);
  const cells=blocks.blocks?.[0]?.cells??[];
  for(let row=6;row<=9;row++) for(let col=1;col<=3;col++) {
   const c=cells.find(c=>c.r===row && c.c===col),font=c ? styleStore.get(c.st??0)?.dto.font : undefined;
   check(`${prefix}${String.fromCharCode(65+col)}${row+1} ${col===1?'日期灰色':'項目／附註藍色'}`,font?.color===(col===1?'7F7F7F':'1C639E') && !font.bold,JSON.stringify(font));
  }
  for(let col=1;col<=3;col++) {
   const c=cells.find(c=>c.r===5 && c.c===col),style=c ? styleStore.get(c.st??0)?.dto : undefined;
   check(`${prefix}標題欄 ${col} 保持黑色粗體`,style?.font.color==='262626' && style.font.bold===true,JSON.stringify(style?.font));
  }
 };
 await checkCells('');
 const g=driver.getGrid();
 if(g) {
  const canvas=document.querySelector<HTMLCanvasElement>('.grid-host > canvas')!,ctx=canvas.getContext('2d')!,dpr=window.devicePixelRatio||1;
  for(const col of [2,3]) {
   const r=g.cellRect({row:6,col}),pixels=ctx.getImageData(Math.ceil(r.x*dpr),Math.ceil(r.y*dpr),Math.floor(r.w*dpr),Math.floor(r.h*dpr)).data;
   let blue=0;for(let p=0;p<pixels.length;p+=4)if(pixels[p+2]>pixels[p]+60 && pixels[p+1]>pixels[p]+30)blue++;
   check(`Canvas 實際繪製 ${col===2?'[項目]':'[附註]'} 藍色文字`,blue>10,`${blue} pixels`);
  }
 }
 const layout=await sheetApi.layout(0,0);check('表格與篩選範圍正常',layout.tables?.[0]?.range.r0===5 && layout.tables[0].range.r1===9);
 await benchApi.snapshot('test4-fixed',driver.snapshotWithObjects());
 check('測試不修改原文件',!(await documentApi.state()).dirty);
 const copy=await benchApi.saveCopy('test4-roundtrip.xlsx');await documentApi.openPath(copy);await idle();await checkCells('另存重開：');
 const failed=checks.filter(c=>!c.ok).length;
 return {failed,markdown:['# test4.xlsx 相容性測試','',`${checks.length-failed} / ${checks.length} 通過`,'',...checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`),''].join('\n')};
}
