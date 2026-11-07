import { benchApi, documentApi, sheetApi } from '../bridge/sheet';
import type { SheetDriver } from './selfTest';

export async function advancedFormatSelfTest(sheet: SheetDriver, index: number): Promise<string> {
 const lines=['\n## 斜線框線、圖樣與漸層填滿','','| | 項目 |','| --- | --- |'];let passed=0,total=0;
 const expect=(name:string,ok:boolean)=>{total++;if(ok)passed++;lines.push(`| ${ok?'✅':'❌'} | ${name} |`);};
 const step=async()=>{await sheet.whenIdle();await new Promise(r=>setTimeout(r,120));};
 const modal=()=>document.querySelector<HTMLElement>('[role="dialog"][aria-label="儲存格格式"]')!;
 const button=(s:string)=>Array.from(modal().querySelectorAll<HTMLButtonElement>('button')).find(b=>b.textContent?.trim()===s)!;
 const tab=async(s:string)=>{button(s).click();await step();};
 const open=async()=>{sheet.focus();sheet.pressKey('1',{metaKey:true});await step();};
 const select=async(r:number,c:number,c1=c)=>{await sheet.goToName(`${String.fromCharCode(65+c)}${r+1}`);sheet.setSelectionForTest({active:{row:r,col:c},anchor:{row:r,col:c},focus:{row:r,col:c1},ranges:[{r0:r,c0:c,r1:r,c1}]});await step();};
 const raw=(r=154,c=1)=>sheetApi.cellStyle(index,r,c);
 const input=async(label:string,value:string)=>{const el=modal().querySelector<HTMLInputElement>(`input[aria-label="${label}"]`)!;el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));await step();};
 const choose=async(label:string,value:string)=>{const el=modal().querySelector<HTMLSelectElement>(`select[aria-label="${label}"]`)!;el.value=value;el.dispatchEvent(new Event('change',{bubbles:true}));await step();};
 const accept=async()=>{button('確定').click();await step();};
 try {
  await sheetApi.setCell(index,154,1,'advanced');await sheetApi.setCell(index,154,2,'mixed');await select(154,1);
  await open();await tab('框線');
  const style=modal().querySelector<HTMLSelectElement>('select')!;style.value='double';style.dispatchEvent(new Event('change',{bubbles:true}));await step();
  modal().querySelector<HTMLInputElement>('[aria-label="斜線 ╱"]')!.click();modal().querySelector<HTMLInputElement>('[aria-label="斜線 ╲"]')!.click();await step();
  expect('交叉斜線預覽保留雙線',modal().querySelector('svg[aria-label="斜線預覽"]')?.querySelectorAll('line').length===4);
  await accept();expect('交叉斜線共用線型套用',!!(await raw())?.diagonalUp && !!(await raw())?.diagonalDown && (await raw())?.diagonal?.style==='double');
  await sheet.runMenu('undo');await step();expect('交叉斜線一次復原',!(await raw())?.diagonalUp && !(await raw())?.diagonalDown);
  await sheet.runMenu('redo');await step();await open();await tab('框線');modal().querySelector<HTMLInputElement>('[aria-label="斜線 ╱"]')!.click();await accept();
  expect('只清除一條斜線保留另一條原線型',!(await raw())?.diagonalUp && !!(await raw())?.diagonalDown && (await raw())?.diagonal?.style==='double');
  await open();await tab('填滿');await choose('填滿類型','pattern');await choose('圖樣','darkGrid');await input('前景色','#ff0000');await input('背景色','#ffff00');
  const canvas=modal().querySelector<HTMLCanvasElement>('.fill-preview canvas')!;const data=canvas.getContext('2d')!.getImageData(0,0,8,8).data;
  expect('圖樣預覽同時繪製前景與背景色',data.some((_,i)=>i%4===1&&data[i]===0)&&data.some((_,i)=>i%4===1&&data[i]===255));
  await accept();expect('圖樣填滿保留兩種色彩與斜線',(await raw())?.fillSpec?.pattern==='darkGrid' && (await raw())?.fillSpec?.foreground==='FF0000' && (await raw())?.fillSpec?.background==='FFFF00' && !!(await raw())?.diagonalDown);
  const pattern=JSON.stringify((await raw())?.fillSpec);await open();await tab('字型');const bold=modal().querySelector<HTMLInputElement>('input[type="checkbox"]')!;bold.click();await accept();
  expect('僅改字型保留圖樣填滿',JSON.stringify((await raw())?.fillSpec)===pattern);
  await select(154,2);await open();await tab('填滿');await choose('填滿類型','linear');await input('角度','37');button('新增色彩節點').click();await step();await input('位置 2','0.35');await input('色彩 2','#ffff00');await accept();
  expect('任意角度三節點線性漸層套用',(await raw(154,2))?.fillSpec?.degree===37 && JSON.parse((await raw(154,2))?.fillSpec?.stops || '[]').length===3);
  await select(155,1);await sheetApi.setCell(index,155,1,'path');await open();await tab('填滿');await choose('填滿類型','path');await input('gradient-left','0.3');await input('gradient-right','0.7');await accept();
  expect('矩形路徑漸層保留起始範圍',(await raw(155,1))?.fillSpec?.kind==='path' && (await raw(155,1))?.fillSpec?.left===.3 && (await raw(155,1))?.fillSpec?.right===.7);
  await open();await tab('填滿');await input('gradient-left','0.9');expect('不合法漸層矩形阻止送出',button('確定').disabled);button('取消').click();await step();
  expect('取消不合法草稿保留原設定',(await raw(155,1))?.fillSpec?.left===.3);
  await select(154,1,2);await open();await tab('填滿');expect('不同進階填滿顯示混合類型',modal().querySelector<HTMLSelectElement>('[aria-label="填滿類型"]')?.value==='__mixed__');button('取消').click();await step();
  for(const ext of ['xlsx','wwx']) {
   const path=await benchApi.saveCopy(`selftest-advanced.${ext}`);await documentApi.openPath(path);await step();
   expect(`${ext} 重開保留斜線、圖樣與兩類漸層`,(await raw())?.fillSpec?.pattern==='darkGrid' && !!(await raw())?.diagonalDown && (await raw(154,2))?.fillSpec?.degree===37 && (await raw(155,1))?.fillSpec?.kind==='path');
  }
  await select(154,1);await open();await tab('填滿');button('無填滿').click();await accept();expect('無填滿清除圖樣仍保留斜線',!(await raw())?.fillSpec?.kind && !!(await raw())?.diagonalDown);
  await sheet.runMenu('undo');await step();await open();await tab('框線');button('無框線').click();await accept();expect('無框線清除斜線仍保留圖樣',!(await raw())?.diagonalDown && (await raw())?.fillSpec?.kind==='pattern');
 } catch(error) {expect('驗收流程完成',false);lines.push(`| ❌ | ${String(error).replaceAll('|','/').replaceAll('\n',' ')} |`);}
 lines.splice(2,0,`- 結果：${passed} / ${total} 通過`,'');return lines.join('\n')+'\n';
}
