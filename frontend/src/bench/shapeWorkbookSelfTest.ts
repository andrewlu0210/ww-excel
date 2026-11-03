import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function shapeWorkbookSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}>{
 const checks:{name:string;ok:boolean;detail:string}[]=[];
 const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
 const idle=async()=>{await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,220));};
 const text=(id:number)=>driver.getObjects().find(o=>o.id===id)?.shape?.runs?.map(r=>r.text).join('')??'';
 const element=(id:number)=>document.querySelector<HTMLElement>(`[data-object-id="${id}"]`)!;
 const edit=async(id:number)=>{element(id).dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await idle();};
 const input=()=>document.querySelector<HTMLDivElement>('.shape-text-input')!;
 const insert=(value:string)=>document.execCommand('insertText',false,value);
 const key=(key:string,mods:Partial<KeyboardEventInit>={})=>input().dispatchEvent(new KeyboardEvent('keydown',{key,bubbles:true,cancelable:true,...mods}));
 await idle();const objects=driver.getObjects().filter(o=>o.shape),first=objects[0],second=objects[1];
 check('匯入兩個可編輯文字方塊',objects.length===2 && !!first?.shape && !!second?.shape);
 const geometry=(o:typeof first)=>JSON.stringify({...o,shape:{...o.shape,runs:[]}});
 const original=geometry(first),initial=text(first.id),sibling=JSON.stringify(second);
 await benchApi.saveCopy('shape-edit-copy.xlsx');
 await edit(first.id);key('Enter',{metaKey:true});await idle();
 check('未改文字直接完成不增加文件變更',!(await documentApi.state()).dirty);
 await edit(first.id);check('雙擊進入文字編輯',driver.getShapeEditing()===first.id && document.activeElement===input());
 driver.selectShapeText(2,initial.length);insert('取消測試');await idle();key('Escape');await idle();
 check('取消編輯保留文字與外觀',text(first.id)===initial && geometry(driver.getObjects()[0])===original);
 await edit(first.id);driver.selectShapeText(2,initial.length);insert('說明🙂');await idle();
 check('編輯中標題保留紅色粗體',input().querySelector('span')?.textContent==='標題' && input().querySelector('span')?.style.color==='rgb(255, 0, 0)');
 await driver.runMenu('undo');await idle();check('編輯中復原文字',input().textContent===initial);
 await driver.runMenu('redo');await idle();check('編輯中重做文字',input().textContent==='標題說明🙂');
 driver.selectShapeText(2,4);await driver.applyFormat({fontColor:'0070C0',underline:true,italic:true,fontSize:18});await idle();
 check('局部格式即時顯示',[...input().querySelectorAll('span')].some(s=>s.textContent==='說明' && s.style.color==='rgb(0, 112, 192)' && s.style.textDecoration.includes('underline')));
 key('Enter',{metaKey:true});await idle();
 check('完成寫入文字與分段格式',driver.getShapeEditing()===null && text(first.id)==='標題說明🙂' && driver.getObjects()[0].shape?.runs?.some(r=>r.text==='說明' && r.font.underline && r.font.italic)===true);
 check('文字修改不改變位置尺寸與背景',geometry(driver.getObjects()[0])===original);
 check('未修改的其他文字方塊完整保留',JSON.stringify(driver.getObjects()[1])===sibling);
 const edited=JSON.stringify(driver.getObjects()[0].shape?.runs);
 await new Promise(r=>setTimeout(r,160));await driver.runMenu('undo');await idle();check('提交後一次復原文字與格式',text(first.id)===initial);
 await driver.runMenu('redo');await idle();check('提交後重做文字與格式',JSON.stringify(driver.getObjects()[0].shape?.runs)===edited);
 await benchApi.snapshot('shape-text-edited',driver.snapshotWithObjects());
 // Actual move/resize events must remain separate from text editing.
 const surface=element(first.id);surface.dispatchEvent(new PointerEvent('pointerdown',{button:0,pointerId:42,clientX:100,clientY:100,bubbles:true}));
 surface.dispatchEvent(new PointerEvent('pointermove',{button:0,pointerId:42,clientX:124,clientY:112,bubbles:true}));surface.dispatchEvent(new PointerEvent('pointerup',{button:0,pointerId:42,bubbles:true}));await idle();
 check('拖曳移動保留文字且不進入編輯',driver.getShapeEditing()===null && JSON.stringify(driver.getObjects()[0].shape?.runs)===edited && geometry(driver.getObjects()[0])!==original);
 const moved=driver.getObjects()[0];
 const handle=element(first.id).querySelector<HTMLElement>('.handle.se');
 check('選取文字方塊可顯示縮放控制點',!!handle);
 if(handle){handle.dispatchEvent(new PointerEvent('pointerdown',{button:0,pointerId:43,clientX:200,clientY:200,bubbles:true}));handle.dispatchEvent(new PointerEvent('pointermove',{button:0,pointerId:43,clientX:240,clientY:220,bubbles:true}));handle.dispatchEvent(new PointerEvent('pointerup',{button:0,pointerId:43,bubbles:true}));await idle();}
 check('縮放後保留文字與格式',driver.getObjects()[0].w===moved.w+40 && driver.getObjects()[0].h===moved.h+20 && JSON.stringify(driver.getObjects()[0].shape?.runs)===edited);
 for(const name of ['shape-edited.xlsx','shape-edited.wwx','shape-from-native.xlsx']){
  const saved=await benchApi.saveCopy(name);await documentApi.openPath(saved);await idle();
  check(`另存重開 ${name} 保留文字、顏色、位置與尺寸`,text(driver.getObjects()[0].id)==='標題說明🙂' && driver.getObjects()[0].shape?.runs?.some(r=>r.text==='說明' && r.font.color==='0070C0')===true && Math.abs(driver.getObjects()[0].w-(moved.w+40))<1);
 }
 const current=driver.getObjects()[0];await edit(current.id);driver.selectShapeText(0,text(current.id).length);
 // Simulated composition leaves the browser's live DOM intact until completion.
 input().dispatchEvent(new CompositionEvent('compositionstart',{bubbles:true}));insert('中文🙂');key('Enter',{isComposing:true});
 check('組字期間 Enter 不會提交文字方塊',driver.getShapeEditing()===current.id);
 input().dispatchEvent(new CompositionEvent('compositionend',{bubbles:true,data:'中文🙂'}));await idle();key('Enter',{metaKey:true});await idle();
 check('中文組字完成可以提交',text(current.id)==='中文🙂');
 await edit(current.id);driver.selectShapeText(0,text(current.id).length);document.execCommand('delete');key('Enter',{metaKey:true});await idle();
 check('刪除全部文字仍保留方塊',text(current.id)==='' && driver.getObjects().some(o=>o.id===current.id && o.shape));
 const empty=await benchApi.saveCopy('shape-empty.xlsx');await documentApi.openPath(empty);await idle();
 check('空白文字方塊存檔重開仍存在',driver.getObjects().filter(o=>o.shape).length===2 && text(driver.getObjects()[0].id)==='');
 await edit(driver.getObjects()[1].id);driver.selectShapeText(0,4);await driver.applyFormat({bold:false});await idle();
 check('白色文字編輯不會變成黑色',input().querySelector('span')?.style.color==='rgb(255, 255, 255)');key('Escape');await idle();
 await edit(driver.getObjects()[0].id);driver.selectShapeText(0,0);insert('切換工作表保留文字');
 const selectTab=async(index:number)=>{const tab=document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[index];tab.dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));window.dispatchEvent(new PointerEvent('pointerup'));await idle();};
 await selectTab(1);
 const savedShape=(await sheetApi.layout(0,0)).objects?.find(o=>o.shape);
 check('切換工作表會提交文字方塊草稿',savedShape?.shape?.runs?.map(r=>r.text).join('')==='切換工作表保留文字');
 await selectTab(0);await edit(driver.getObjects()[0].id);driver.selectShapeText(0,text(driver.getObjects()[0].id).length);insert('公式列切換保留文字');
 document.querySelector<HTMLElement>('.formula')!.focus();await idle();
 check('點公式列會完成文字方塊編輯',driver.getShapeEditing()===null && text(driver.getObjects()[0].id)==='公式列切換保留文字');driver.pressKey('Escape');await idle();
 const failed=checks.filter(c=>!c.ok).length;
 return{failed,markdown:'# 文字方塊編輯驗收\n\n'+`${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
