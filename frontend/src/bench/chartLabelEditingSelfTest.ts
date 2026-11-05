import {chartSeriesEditingSelfTest} from './chartSeriesEditingSelfTest';
import {chartAxisEditingSelfTest} from './chartAxisEditingSelfTest';
import {benchApi,documentApi,sheetApi} from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function chartLabelEditingSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}> {
  const checks:{name:string;ok:boolean}[]=[];
  const check=(name:string,ok:boolean)=>checks.push({name,ok});
  const step=async()=>{await driver.whenIdle();await new Promise(r=>setTimeout(r,220));};
  const object=async()=>((await sheetApi.layout(0,0)).objects ?? []).find(o=>o.kind==='chart')!;
  const labels=async()=>(await object()).chart!.series![0].labels;
  const open=async()=>{driver.selectObject((await object()).id);await driver.runMenu('editChart');await step();return document.querySelector<HTMLElement>('.chart-dialog')!;};
  const click=(root:ParentNode,label:string)=>{const b=[...root.querySelectorAll<HTMLButtonElement>('button')].find(b=>b.textContent?.trim()===label);if(!b)throw Error('Missing button '+label);b.click();};
  const set=(root:ParentNode,label:string,value:string)=>{const input=root.querySelector<HTMLInputElement|HTMLSelectElement|HTMLTextAreaElement>(`[aria-label="${label}"]`)!;if(!input)throw Error('Missing field '+label);input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new Event('change',{bubbles:true}));};
  const toggle=(root:ParentNode,label:string)=>root.querySelector<HTMLInputElement>(`input[aria-label="${label}"]`)!.click();
  let dialog=await open();check('開啟圖表標籤介面',!!dialog?.querySelector('.label-editor'));
  toggle(dialog,'啟用資料標籤');await step();
  set(dialog,'標籤位置','outEnd');set(dialog,'標籤字型大小','16');set(dialog,'個別資料標籤','1');await step();
  toggle(dialog,'自訂標籤文字');await step();set(dialog,'標籤文字','自訂收入\n150 元');toggle(dialog,'自訂標籤位置');await step();set(dialog,'標籤水平位置','45');set(dialog,'標籤垂直位置','25');
  click(dialog,'確定');await step();
  let l=await labels();check('套用數列與個別標籤',!document.querySelector('.chart-dialog')&&l?.fontSize===16&&l.position==='outEnd'&&l.points?.['1']?.customText==='自訂收入\n150 元'&&l.points?.['1']?.layout?.x===.45);
  const id=(await object()).id;check('後端提供自訂標籤文字',(await sheetApi.chartData(0,id)).series?.[0]?.labels?.[1]==='自訂收入\n150 元');
  check('列印用圖表圖片已更新',(driver.chartImages()[id] ?? '').length>2000);
  await driver.runMenu('undo');await step();check('標籤編輯可以復原',!(await labels())?.showValue);
  await driver.runMenu('redo');await step();check('標籤編輯可以重做',(await labels())?.fontSize===16);
  dialog=await open();set(dialog,'標籤字型大小','22');click(dialog,'取消');await step();check('取消不套用變更',(await labels())?.fontSize===16);
  for(const ext of ['xlsx','wwx']) {
    const path=await benchApi.saveCopy('chart-label-edited.'+ext);await documentApi.openPath(path);await step();await step();
    l=await labels();check(ext+' 重開保留文字與位置',l?.points?.['1']?.customText==='自訂收入\n150 元'&&l.points?.['1']?.layout?.x===.45&&l.fontSize===16);
  }
  dialog=await open();toggle(dialog,'啟用資料標籤');click(dialog,'確定');await step();check('停用標籤保留編輯設定',(await labels())?.disabled===true&&(await labels())?.points?.['1']?.customText==='自訂收入\n150 元');
  await driver.runMenu('undo');await step();
  const current=(await object()).id;await sheetApi.setCell(0,1,1,'250');await step();check('來源更新會重算數值標籤',(await sheetApi.chartData(0,current)).series?.[0]?.labels?.[0]==='250');
  // Exercise the actual label targets over the canvas, including zoomed coordinates.
  const geometry=(o:Awaited<ReturnType<typeof object>>)=>JSON.stringify([o.row,o.col,o.dx,o.dy,o.w,o.h]);
  const hit=()=>document.querySelector<HTMLElement>(`[data-object-id="${driver.getObjects().find(o=>o.chart)!.id}"] [data-label-series="0"][data-label-point="1"]`)!;
  const pointer=(target:HTMLElement,type:string,x:number,y:number)=>target.dispatchEvent(new PointerEvent(type,{button:0,buttons:type==='pointerup'?0:1,pointerId:81,bubbles:true,cancelable:true,clientX:x,clientY:y}));
  async function dragLabel(dx:number,dy:number,finish:'pointerup'|'pointercancel'|'escape'|'blur'='pointerup') {
    const element=hit();const box=element.getBoundingClientRect();const x=box.left+box.width/2,y=box.top+box.height/2;
    pointer(element,'pointerdown',x,y);pointer(element,'pointermove',x+dx,y+dy);await step();
    if(finish==='escape')window.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}));
    else if(finish==='blur')window.dispatchEvent(new Event('blur'));
    else pointer(element,finish,x+dx,y+dy);
    await step();
  }
  const beforeDrag=await object();const beforeLayout=(await labels())!.points!['1']!.layout!;
  const beforeUndo=(await documentApi.state()).undoLabel;
  await dragLabel(0,0);check('單擊標籤不新增修改',(await documentApi.state()).undoLabel===beforeUndo&&(await labels())?.points?.['1']?.layout?.x===beforeLayout.x);
  await dragLabel(40,24);
  const moved=(await labels())!.points!['1']!.layout!;
  check('拖曳只移動個別標籤且保留文字',Math.abs(moved.x-beforeLayout.x-40/beforeDrag.w)<1e-6&&Math.abs(moved.y-beforeLayout.y-24/beforeDrag.h)<1e-6&&geometry(await object())===geometry(beforeDrag)&&(await labels())?.points?.['1']?.customText==='自訂收入\n150 元');
  await driver.runMenu('undo');await step();check('一次復原還原整次拖曳',(await labels())?.points?.['1']?.layout?.x===beforeLayout.x);
  await driver.runMenu('redo');await step();check('重做恢復拖曳位置',(await labels())?.points?.['1']?.layout?.x===moved.x);
  await dragLabel(35,20,'escape');check('Escape 取消拖曳',(await labels())?.points?.['1']?.layout?.x===moved.x);
  await dragLabel(35,20,'pointercancel');check('取消指標事件不提交位置',(await labels())?.points?.['1']?.layout?.x===moved.x);
  await dragLabel(35,20,'blur');check('視窗失焦取消拖曳',(await labels())?.points?.['1']?.layout?.x===moved.x);
  const scroller=document.querySelector<HTMLElement>('.scroller')!;scroller.scrollLeft=80;scroller.scrollTop=32;await step();
  const zoom=document.querySelector<HTMLInputElement>('[aria-label="縮放比例"]')!;
  zoom.value='150';zoom.dispatchEvent(new Event('input',{bubbles:true}));await step();
  await dragLabel(30,15);const zoomed=(await labels())!.points!['1']!.layout!;
  check('捲動與 150% 縮放下拖曳座標正確',Math.abs(zoomed.x-moved.x-20/beforeDrag.w)<1e-6&&Math.abs(zoomed.y-moved.y-10/beforeDrag.h)<1e-6&&geometry(await object())===geometry(beforeDrag));
  zoom.value='100';zoom.dispatchEvent(new Event('input',{bubbles:true}));await step();
  driver.getGrid()?.scrollToOrigin();await step();
  for(const ext of ['xlsx','wwx']) {
    const path=await benchApi.saveCopy('chart-label-dragged.'+ext);await documentApi.openPath(path);await step();await step();
    const position=(await labels())?.points?.['1']?.layout;
    check(ext+' 重開保留拖曳位置',!!position&&Math.abs(position.x-zoomed.x)<1e-6&&Math.abs(position.y-zoomed.y)<1e-6);
  }
  checks.push(...await chartAxisEditingSelfTest(driver));
  checks.push(...await chartSeriesEditingSelfTest(driver));
  await benchApi.snapshot('chart-label-edited',driver.snapshotWithObjects());
  return {failed:checks.filter(c=>!c.ok).length,markdown:'# 圖表設定與資料標籤驗收\n\n'+`${checks.filter(c=>c.ok).length} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}`).join('\n')+'\n'};
}
