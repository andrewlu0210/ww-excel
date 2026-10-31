import {benchApi,documentApi,sheetApi} from '../bridge/sheet';
import {drawConditionalIcon} from '../grid/conditional';
import type Sheet from '../components/Sheet.svelte';
export async function taskWorkbookSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}> {
 const checks:{name:string;ok:boolean;detail:string}[]=[];
 const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
 await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,250));
 const state=await documentApi.state(),layout=await sheetApi.layout(0,0);
 check('開啟工作清單',state.sheets?.length===1 && state.sheets[0]==='工作',JSON.stringify(state.sheets));
 check('隱藏工作表格線',layout.hideGridLines===true);
 check('工作表預設列高 44px',layout.defaultRow===44,String(layout.defaultRow));
 check('表格與篩選按鈕設定已載入',layout.tables?.length===1 && layout.tables[0].name==='工作' && !layout.tables[0].noFilter,JSON.stringify(layout.tables));
 const blocks=await sheetApi.getBlocks(0,[{row:0,col:0}],0),cells=blocks.blocks?.[0]?.cells??[];
 const cell=(r:number,c:number)=>cells.find(x=>x.r===r && x.c===c);
 check('工作清單標題與三筆資料載入',cell(0,1)?.v==='工作清單' && [2,3,4].every(r=>cell(r,1)?.v==='工作'));
 for(const [r,want] of [[2,'0%'],[3,'50%'],[4,'100%']] as const) check(`E${r+1} 百分比顯示`,cell(r,4)?.v===want,JSON.stringify(cell(r,4)));
 check('F3/F4 尚未完成保持空白',cell(2,5)?.v==='' && cell(3,5)?.v==='',JSON.stringify([cell(2,5),cell(3,5)]));
 check('F5 完成公式與格式顯示',cell(4,5)?.v==='完成' && cell(4,5)?.textOnly===true,JSON.stringify(cell(4,5)));
 check('完成百分比下拉選項',JSON.stringify(await sheetApi.listValues(0,2,4))===JSON.stringify(['0%','25%','50%','75%','100%']));
 check('表格篩選選單可讀取',(await sheetApi.filterValues(0,1)).length===1);
 check('讀取百分比的資料長條比例',cell(3,4)?.bar===.5 && cell(4,4)?.bar===1,JSON.stringify([cell(3,4),cell(4,4)]));
 check('資料長條保留原檔黃色主題色與實心設定',cell(3,4)?.bc==='F1C94D' && cell(3,4)?.barSolid===true,JSON.stringify(cell(3,4)));
 const rules=await sheetApi.condFormats(0);
 check('完成狀態的條件式圖示已匯入',rules.some(r=>r.type.includes('icon')),JSON.stringify(rules));
 check('0% 與 50% 不顯示完成圖示',!cell(2,5)?.icon && !cell(3,5)?.icon);
 check('100% 顯示原檔黑色圓形圖示',cell(4,5)?.icon?.set==='4TrafficLights' && cell(4,5)?.icon?.index===0,JSON.stringify(cell(4,5)?.icon));
 const grid=driver.getGrid();
 if(grid) {
  const image=new Image();image.src=grid.snapshot();await image.decode();
  const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;
  const ctx=canvas.getContext('2d')!;ctx.drawImage(image,0,0);const dpr=window.devicePixelRatio||1;
  const count=(row:number,col:number,test:(r:number,g:number,b:number)=>boolean)=>{
   const rc=grid.cellRect({row,col});const x=Math.ceil(rc.x*dpr),y=Math.ceil(rc.y*dpr),w=Math.floor(rc.w*dpr),h=Math.floor(rc.h*dpr);
   const data=ctx.getImageData(x,y,w,h).data;let n=0;for(let p=0;p<data.length;p+=4)if(test(data[p],data[p+1],data[p+2]))n++;return n;
  };
  const yellow=(r:number,g:number,b:number)=>r===241 && g===201 && b===77;
  const half=count(3,4,yellow),full=count(4,4,yellow);
  check('Canvas 實際繪製 50% 與 100% 黃色進度條',half>40 && Math.abs(full/half-2)<.15,JSON.stringify({half,full}));
  const status=grid.cellRect({row:4,col:5});const actual=ctx.getImageData(Math.round(status.x*dpr),Math.round(status.y*dpr),Math.round(status.w*dpr),Math.round(status.h*dpr)).data;
  const expected=document.createElement('canvas');expected.width=Math.round(status.w*dpr);expected.height=Math.round(status.h*dpr);const expectedCtx=expected.getContext('2d')!;expectedCtx.scale(dpr,dpr);
  drawConditionalIcon(expectedCtx,{set:'4TrafficLights',index:0},3,(status.h-10)/2,10);
  const expectedPixels=expectedCtx.getImageData(0,0,expected.width,expected.height).data;
  let black=0,extra=0;
  for(let p=0;p<actual.length;p+=4)if(actual[p]<90 && actual[p+1]<90 && actual[p+2]<90){black++;if(expectedPixels[p+3]<10)extra++;}
  check('F5 Canvas 僅畫圓點、不畫井字號',black>15 && extra<12,JSON.stringify({black,extra}));
 }
 check('原檔沒有圖表或圖片' ,driver.getObjects().length===0);
 await benchApi.snapshot('test2-open',driver.snapshotWithObjects());
 check('開啟與檢查不修改原文件',!(await documentApi.state()).dirty);
 const copy=await benchApi.saveCopy('test2-roundtrip-ui.xlsx');await documentApi.openPath(copy);await driver.whenIdle();await new Promise(r=>setTimeout(r,300));
 const back=await sheetApi.getBlocks(0,[{row:0,col:0}],0);
 check('另存副本再開啟保留公式與百分比',back.blocks?.[0]?.cells?.find(c=>c.r===4 && c.c===5)?.v==='完成' && back.blocks?.[0]?.cells?.find(c=>c.r===3 && c.c===4)?.v==='50%');
 check('另存後保留黃色進度條與完成圖示',back.blocks?.[0]?.cells?.find(c=>c.r===3 && c.c===4)?.bc==='F1C94D' && back.blocks?.[0]?.cells?.find(c=>c.r===4 && c.c===5)?.icon?.set==='4TrafficLights');
 check('另存後保留下拉選項',(await sheetApi.listValues(0,2,4)).length===5);
 await sheetApi.setCell(0,3,4,'100%');await driver.whenIdle();await new Promise(r=>setTimeout(r,200));
 const changed=await sheetApi.getBlocks(0,[{row:0,col:0}],0);
 check('修改百分比會更新完成圓點',changed.blocks?.[0]?.cells?.find(c=>c.r===3 && c.c===5)?.icon?.set==='4TrafficLights');
 await sheetApi.undo();await driver.whenIdle();
 const undo=await sheetApi.getBlocks(0,[{row:0,col:0}],0);
 check('復原後恢復 50% 並移除圓點',undo.blocks?.[0]?.cells?.find(c=>c.r===3 && c.c===4)?.v==='50%' && !undo.blocks?.[0]?.cells?.find(c=>c.r===3 && c.c===5)?.icon);
 const failed=checks.filter(c=>!c.ok).length;
 return {failed,markdown:`# test2.xlsx 相容性檢查\n\n${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
