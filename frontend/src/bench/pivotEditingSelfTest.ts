import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function pivotEditingSelfTest(driver: ReturnType<typeof Sheet>): Promise<{markdown:string;failed:number}> {
  const checks: {name:string;ok:boolean;detail:string}[]=[];
  const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
  const step=async()=>{await driver.whenIdle();await new Promise(r=>setTimeout(r,180));};
  const info=async()=>((await sheetApi.pivots(0)) ?? [])[0];
  const number=async(label:string)=>{
    const p=(await info()).pivot;
    for(let r=p.Range.R0;r<=p.Range.R1;r++) if(await sheetApi.editText(0,r,p.Range.C0)===label) return Number(await sheetApi.editText(0,r,p.Range.C0+1));
    return NaN;
  };
  const click=(root:ParentNode,label:string)=>{const button=[...root.querySelectorAll<HTMLButtonElement>('button')].find(b=>b.textContent?.trim().endsWith(label));if(!button)throw Error('Missing button: '+label);button.click();};
  document.querySelector<HTMLButtonElement>('#ribbon-tab-4')!.click();await step();
  check('匯入來源與欄位', (await info()).fields?.join(',')==='產品,銷售人員,金額');
  check('匯入樞紐金額',await number('啤酒')===2150&&await number('總計')===5950);
  await sheetApi.setCell(0,1,2,'2000');await step();
  check('修改來源後等待手動重新整理',await number('啤酒')===2150);
  click(document.querySelector('.ribbon')!,'重新整理樞紐分析表');await step();
  check('功能區重新整理更新金額',await number('啤酒')===2750&&await number('總計')===6550);
  await driver.runMenu('undo');await step();check('重新整理可復原',await number('啤酒')===2150);
  await driver.runMenu('redo');await step();check('重新整理可重做',await number('啤酒')===2750);
  // Hit the actual canvas dropdown, including the pointer coordinate path.
  const grid=driver.getGrid()!;
  const p=(await info()).pivot;
  grid.scrollToOrigin();await step();
  const rect=grid.cellRect({row:p.Range.R0,col:p.Range.C0});
  const scroller=document.querySelector<HTMLElement>('.scroller')!;
  const bounds=scroller.getBoundingClientRect();
  scroller.dispatchEvent(new PointerEvent('pointerdown',{button:0,buttons:1,pointerId:71,bubbles:true,cancelable:true,clientX:bounds.left+rect.x+rect.w-9,clientY:bounds.top+rect.y+rect.h/2}));
  scroller.dispatchEvent(new PointerEvent('pointerup',{button:0,pointerId:71,bubbles:true}));await step();
  const filter=document.querySelector<HTMLElement>('.menu[role="dialog"]');
  check('樞紐下拉按鈕開啟獨立篩選',!!filter&&!filter.querySelector('.sorts'));
  if(filter) {
    for(const label of filter.querySelectorAll<HTMLLabelElement>('li label')) {const input=label.querySelector<HTMLInputElement>('input')!;const value=label.textContent?.trim();if(value==='紅酒'||value==='汽水'){input.click();}}
    click(filter,'確定');await step();
    check('只選啤酒更新小計與總計',await number('啤酒')===2750&&await number('總計')===2750);
    check('樞紐篩選不改變一般表格篩選',(await sheetApi.layout(0,0)).filteredCols?.length===0);
    await driver.runMenu('undo');await step();check('篩選可復原',await number('總計')===6550);
    await driver.runMenu('redo');await step();check('篩選可重做',await number('總計')===2750);
  }
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();
  let dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;
  check('開啟欄位設定介面',!!dialog);
  const filterSelect=dialog.querySelector<HTMLSelectElement>('[aria-label="篩選欄位"]')!;
  filterSelect.value='產品';filterSelect.dispatchEvent(new Event('change',{bubbles:true}));await step();
  click(dialog,'全選／清除篩選');
  const rows=dialog.querySelectorAll('fieldset')[0];
  rows.querySelector<HTMLButtonElement>('[aria-label="移除欄位"]')!.click();await step();
  const select=rows.querySelector<HTMLSelectElement>('select')!;select.value='銷售人員';select.dispatchEvent(new Event('change',{bubbles:true}));await step();
  click(dialog,'套用並重新整理');await step();
  check('調整列欄位為銷售人員',!document.querySelector('.pivot-dialog')&&await number('甲')===4290&&await number('總計')===6550);
  for(const ext of ['xlsx','wwx']) {
    const path=await benchApi.saveCopy('pivot-edited.'+ext);await documentApi.openPath(path);await step();await step();
    check(ext+' 保留欄位設定與結果',(await info()).pivot.Rows?.[0]==='銷售人員'&&await number('總計')===6550);
    check(ext+' 重開後仍可重新整理',await sheetApi.refreshPivot(0,'SalesPivot')===null);await step();
  }
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;
  const summary=dialog.querySelector<HTMLSelectElement>('[aria-label="彙總方式"]')!;summary.value='count';summary.dispatchEvent(new Event('change',{bubbles:true}));click(dialog,'套用並重新整理');await step();
  check('切換筆數彙總',await number('總計')===6);
  await driver.runMenu('undo');await step();check('欄位設定可復原',await number('總計')===6550);
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;click(dialog,'取消');await step();check('取消不修改樞紐結果',await number('總計')===6550);
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;
  const average=dialog.querySelector<HTMLSelectElement>('[aria-label="彙總方式"]')!;average.value='average';average.dispatchEvent(new Event('change',{bubbles:true}));click(dialog,'套用並重新整理');await step();
  check('平均值總計依全部來源筆數計算',Math.abs(await number('總計')-6550/6)<1e-8);
  await driver.runMenu('undo');await step();
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;
  const addValue=dialog.querySelector<HTMLSelectElement>('[aria-label="新增值欄位"]')!;addValue.value='金額';addValue.dispatchEvent(new Event('change',{bubbles:true}));await step();
  const summaries=dialog.querySelectorAll<HTMLSelectElement>('[aria-label="彙總方式"]');summaries[1].value='count';summaries[1].dispatchEvent(new Event('change',{bubbles:true}));click(dialog,'套用並重新整理');await step();
  let configured=(await info()).pivot;
  check('新增第二個值欄位並彙總筆數',configured.Values?.length===2&&Number(await sheetApi.editText(0,configured.Range.R1,configured.Range.C1))===6);
  await driver.runMenu('undo');await step();
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;
  const columnSelect=dialog.querySelector<HTMLSelectElement>('[aria-label="欄欄位"]')!;columnSelect.value='產品';columnSelect.dispatchEvent(new Event('change',{bubbles:true}));await step();click(dialog,'套用並重新整理');await step();
  configured=(await info()).pivot;
  check('加入欄欄位形成交叉彙總',configured.Columns?.[0]==='產品'&&Number(await sheetApi.editText(0,configured.Range.R1,configured.Range.C1))===6550);
  await driver.runMenu('undo');await step();
  click(document.querySelector('.ribbon')!,'樞紐分析表…');await step();dialog=document.querySelector<HTMLElement>('.pivot-dialog')!;
  const rowArea=dialog.querySelectorAll('fieldset')[0];rowArea.querySelector<HTMLButtonElement>('[aria-label="移除欄位"]')!.click();await step();
  const rowAdd=dialog.querySelector<HTMLSelectElement>('[aria-label="列欄位"]')!;rowAdd.value='產品';rowAdd.dispatchEvent(new Event('change',{bubbles:true}));await step();
  const pageAdd=dialog.querySelector<HTMLSelectElement>('[aria-label="報表篩選"]')!;pageAdd.value='銷售人員';pageAdd.dispatchEvent(new Event('change',{bubbles:true}));await step();
  const pageFilter=dialog.querySelector<HTMLSelectElement>('[aria-label="篩選欄位"]')!;pageFilter.value='銷售人員';pageFilter.dispatchEvent(new Event('change',{bubbles:true}));await step();
  const item=[...dialog.querySelectorAll<HTMLLabelElement>('.items label')].find(l=>l.textContent?.trim()==='乙')!;item.querySelector<HTMLInputElement>('input')!.click();
  click(dialog,'套用並重新整理');await step();
  check('報表篩選欄位只彙總選取項目',await number('總計')===4290&&(await info()).pivot.Pages?.[0]==='銷售人員');
  const pageCopy=await benchApi.saveCopy('pivot-page-filter.xlsx');await documentApi.openPath(pageCopy);await step();await step();
  check('報表篩選另存重開仍保留',await number('總計')===4290&&(await info()).pivot.Filters?.['銷售人員']?.join(',')==='甲');
  await benchApi.snapshot('pivot-edited',driver.snapshotWithObjects());
  const failed=checks.filter(c=>!c.ok).length;
  return {failed,markdown:'# 樞紐分析表編輯驗收\n\n'+`${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
