import { benchApi, documentApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function shapeTest5SelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}>{
 const checks:{name:string;ok:boolean}[]=[];
 const check=(name:string,ok:boolean)=>checks.push({name,ok});
 const idle=async()=>{await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,220));};
 const select=async()=>{
  const tabs=[...document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')];
  const tab=tabs.find(t=>t.textContent?.trim()==='深入了解');if(!tab)throw new Error('Missing learn-more sheet');
  tab.dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));window.dispatchEvent(new PointerEvent('pointerup'));await idle();
 };
 await select();
 const objects=driver.getObjects(),text=(o:typeof objects[number])=>o.shape?.runs?.map(r=>r.text).join('')??'';
 const target=objects.find(o=>text(o).trim()==='深入了解')??objects.find(o=>text(o).includes('深入了解'));
 if(!target?.shape)throw new Error('Missing learn-more text box');
 const siblings=JSON.stringify(objects.filter(o=>o.id!==target.id)),original=JSON.stringify({...target,shape:{...target.shape,runs:[]}});
 document.querySelector<HTMLElement>(`[data-object-id="${target.id}"]`)!.dispatchEvent(new MouseEvent('dblclick',{bubbles:true}));await idle();
 check('test5 深入了解文字方塊可雙擊編輯',driver.getShapeEditing()===target.id);
 driver.selectShapeText(0,text(target).length);document.execCommand('insertText',false,'深入了解測試');
 document.querySelector<HTMLButtonElement>('.shape-edit-actions button')!.click();await idle();
 const edited=driver.getObjects().find(o=>o.id===target.id)!;
 check('test5 修改文字保留白色字型',text(edited)==='深入了解測試' && edited.shape?.runs?.every(r=>r.font.color==='FFFFFF')===true);
 check('test5 位置尺寸背景與其他物件不變',JSON.stringify({...edited,shape:{...edited.shape,runs:[]}})===original && JSON.stringify(driver.getObjects().filter(o=>o.id!==target.id))===siblings);
 const saved=await benchApi.saveCopy('test5-shape-edited.xlsx');await documentApi.openPath(saved);await idle();await select();
 const restored=driver.getObjects().find(o=>text(o)==='深入了解測試');
 check('test5 XLSX 重開保留文字、白色字型與物件數量',!!restored && restored.shape?.runs?.every(r=>r.font.color==='FFFFFF')===true && driver.getObjects().length===objects.length);
 await benchApi.snapshot('test5-shape-text-edited',driver.snapshotWithObjects());
 await driver.runMenu('undo');await idle();
 check('重新開啟不帶入舊文件的復原紀錄',!((await documentApi.state()).canUndo));
 const failed=checks.filter(c=>!c.ok).length;
 return {failed,markdown:'# test5.xlsx 文字方塊編輯\n\n'+`${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}`).join('\n')+'\n'};
}
