// Desktop verification for the generated conditional-icons workbook.
import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import { ICON_SETS } from '../grid/iconSets';
import { drawConditionalIcon } from '../grid/conditional';
import { fromRange } from '../grid/selection';
import type Sheet from '../components/Sheet.svelte';

export async function iconWorkbookSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}>{
 const checks:{name:string;ok:boolean;detail:string}[]=[];
 const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
 const settle=async()=>{await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,180));};
 const select=async(index:number)=>{const tab=document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[index];tab.dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));window.dispatchEvent(new PointerEvent('pointerup'));await settle();};
 const verify=async(label:string)=>{
  const blocks=await sheetApi.getBlocks(0,[{row:0,col:0}],0),cells=blocks.blocks?.[0]?.cells??[];
  for(let row=1;row<=ICON_SETS.length;row++){
   const set=ICON_SETS[row-1];let good=true;
   for(let i=0;i<set.count;i++)for(const start of [1,7]){
    const cell=cells.find(c=>c.r===row && c.c===start+i);
    if(cell?.icon?.set!==set.id || cell.icon.index!==i || !!cell.hideValue!==(start===1))good=false;
   }
   check(`${label}：${set.id} 各狀態與兩種顯示模式`,good);
  }
 };
 await settle();await verify('開啟');
 const zoom=document.querySelector<HTMLInputElement>('.zoom input[type="range"]')!;
 const previous=zoom.value;zoom.value='80';zoom.dispatchEvent(new Event('input',{bubbles:true}));await settle();
 driver.getGrid()?.scrollToOrigin();await settle();
 await benchApi.snapshot('conditional-icons-grid',driver.snapshotWithObjects());
 // Draw a larger gallery with the same canvas paths to inspect small details.
 const gallery=document.createElement('canvas'),dpr=window.devicePixelRatio||1;
 gallery.width=760*dpr;gallery.height=(36+ICON_SETS.length*34)*dpr;
 const ctx=gallery.getContext('2d')!;ctx.scale(dpr,dpr);ctx.fillStyle='#ffffff';ctx.fillRect(0,0,760,gallery.height/dpr);ctx.font='14px sans-serif';
 for(const [row,set] of ICON_SETS.entries()){
  const top=34+row*34;ctx.fillStyle=row%2?'#f3f5f4':'#ffffff';ctx.fillRect(0,top-6,760,34);
  ctx.fillStyle='#33443b';ctx.fillText(set.id,16,top+17);
  for(let i=0;i<set.count;i++)check(`Canvas：${set.id} 圖示 ${i}`,drawConditionalIcon(ctx,{set:set.id,index:i},250+i*75,top,24));
 }
 await benchApi.snapshot('conditional-icons-gallery',gallery.toDataURL());
 check('開啟與擷取未修改文件',!(await documentApi.state()).dirty);
 zoom.value=previous;zoom.dispatchEvent(new Event('input',{bubbles:true}));await settle();
 await select(1);
 const blocks=await sheetApi.getBlocks(1,[{row:0,col:0}],0),cells=blocks.blocks?.[0]?.cells??[];
 const icons=(r:number)=>cells.filter(c=>c.r===r && c.c>=1 && c.c<=5).sort((a,b)=>a.c-b.c).map(c=>c.icon?.index??-1);
 check('反向順序',JSON.stringify(icons(1))===JSON.stringify([2,1,0]),JSON.stringify(icons(1)));
 check('嚴格門檻與相等邊界',JSON.stringify(icons(2))===JSON.stringify([0,0,1,1,2]),JSON.stringify(icons(2)));
 check('自訂無圖示保留空白',cells.find(c=>c.r===3 && c.c===2)?.hideValue===true && !cells.find(c=>c.r===3 && c.c===2)?.icon);
 check('自訂無圖示仍可顯示數值',!cells.find(c=>c.r===4 && c.c===2)?.hideValue && cells.find(c=>c.r===4 && c.c===2)?.v==='50');
 check('百分位數門檻',JSON.stringify(icons(5))===JSON.stringify([0,0,1,1,2]));
 check('公式門檻',JSON.stringify(icons(6))===JSON.stringify([0,1,2]));
 await benchApi.snapshot('conditional-icons-custom',driver.snapshotWithObjects());
 await select(2);await benchApi.snapshot('conditional-icons-narrow',driver.snapshotWithObjects());
 check('合併儲存格與窄欄保留圖示',(await sheetApi.getBlocks(2,[{row:0,col:0}],0)).blocks?.[0]?.cells?.filter(c=>c.r===1 && c.icon).length===3);
 await select(0);await benchApi.saveCopy('icons-edit-copy.xlsx');
 for(const [col,value] of [[1,'20'],[2,'50'],[3,'90']] as const)await sheetApi.setCell(0,25,col,value);
 driver.goTo({row:25,col:1});driver.setSelectionForTest(fromRange({r0:25,c0:1,r1:25,c1:3}));
 await driver.runMenu('condFormat');await settle();
 const dialog=document.querySelector<HTMLElement>('[role="dialog"]')!;
 const kind=dialog.querySelector<HTMLSelectElement>('select')!;kind.value='icon_set';kind.dispatchEvent(new Event('change',{bubbles:true}));await settle();
 const setSelect=dialog.querySelector<HTMLSelectElement>('select[aria-label="圖示集"]')!;
 check('設定視窗提供 20 組圖示',setSelect.options.length===20);
 setSelect.value='5Boxes';setSelect.dispatchEvent(new Event('change',{bubbles:true}));await settle();
 check('五級圖示提供五組預覽',dialog.querySelectorAll('.icon-threshold canvas').length===5);
 setSelect.value='3Arrows';setSelect.dispatchEvent(new Event('change',{bubbles:true}));await settle();
 const hide=[...dialog.querySelectorAll<HTMLLabelElement>('label')].find(l=>l.textContent?.includes('只顯示圖示'))?.querySelector<HTMLInputElement>('input');hide?.click();
 const apply=[...dialog.querySelectorAll<HTMLButtonElement>('button')].find(b=>b.textContent?.trim()==='確定');apply?.click();await settle();
 const row=async()=>((await sheetApi.getBlocks(0,[{row:0,col:0}],0)).blocks?.[0]?.cells??[]).filter(c=>c.r===25 && c.c>=1 && c.c<=3).sort((a,b)=>a.c-b.c);
 let actual=await row();check('介面新增規則後 20／50／90 顯示紅黃綠箭頭',actual.length===3 && actual.every((c,i)=>c.icon?.set==='3Arrows' && c.icon.index===i && c.hideValue),JSON.stringify(actual));
 await sheetApi.setCell(0,25,1,'100');await settle();check('修改數值更新圖示',(await row())[0].icon?.index===2);
 await sheetApi.undo();await settle();check('復原恢復圖示',(await row())[0].icon?.index===0);
 await sheetApi.redo();await settle();check('重做恢復圖示',(await row())[0].icon?.index===2);
 await sheetApi.undo();await settle();
 for(const ext of ['xlsx','wwx','xlsx']){
  const copy=await benchApi.saveCopy(`conditional-icons-roundtrip.${ext}`);
  await documentApi.openPath(copy);await settle();await verify(`另存 ${ext}`);
 }
 const failed=checks.filter(c=>!c.ok).length;
 return{failed,markdown:'# 條件格式圖示驗收\n\n'+`${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
