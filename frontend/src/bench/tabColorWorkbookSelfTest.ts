import {benchApi,documentApi,settingsApi} from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function tabColorWorkbookSelfTest(driver:ReturnType<typeof Sheet>):Promise<{markdown:string;failed:number}> {
  const checks:{name:string;ok:boolean;detail:string}[]=[];
  const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
  const step=async()=>{await driver.whenIdle();await new Promise(r=>setTimeout(r,180));};
  const state=await documentApi.state();const original=state.tabColors ?? [];
  const tabs=()=>[...document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')];
  const rgb=(color:string)=>'rgb('+[0,2,4].map(i=>parseInt(color.slice(i,i+2),16)).join(', ')+')';
  const select=async(i:number)=>{tabs()[i].dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));window.dispatchEvent(new PointerEvent('pointerup'));await step();};
  check('test7 工作表名稱正確',state.sheets?.join(',')==='學期,學分,預算,書籍');
  check('四張工作表均有獨立色彩',original.length===4&&original.every(c=>/^[0-9A-F]{6}$/.test(c))&&new Set(original).size===4,JSON.stringify(original));
  check('預算與書籍分別使用綠色及青色',parseInt(original[2].slice(2,4),16)>parseInt(original[2].slice(0,2),16)&&parseInt(original[3].slice(2,4),16)>parseInt(original[3].slice(0,2),16)&&parseInt(original[3].slice(4,6),16)>parseInt(original[3].slice(0,2),16));
  for(let i=0;i<4;i++){
    await select(i);const list=tabs();const selected=list[i];
    check(`切換「${state.sheets![i]}」保留原色底線`,selected.getAttribute('aria-selected')==='true'&&getComputedStyle(selected).borderBottomColor===rgb(original[i]));
    check(`「${state.sheets![i]}」選取樣式與其他分頁區分`,getComputedStyle(selected).backgroundColor!==rgb(original[i])&&list.every((tab,j)=>j===i||getComputedStyle(tab).backgroundColor===rgb(original[j])));
  }
  check('瀏覽與切換不修改文件',!(await documentApi.state()).dirty);
  async function palette(i:number){const element=tabs()[i],rect=element.getBoundingClientRect();element.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:rect.left+4,clientY:rect.top+4}));await step();[...document.querySelectorAll<HTMLButtonElement>('.tab-menu button')].find(b=>b.textContent==='分頁色彩…')!.click();await step();return document.querySelector<HTMLElement>('.tab-color-popup')!;}
  let picker=await palette(2);check('右鍵可開啟分頁色彩選單',!!picker);
  picker.querySelector<HTMLButtonElement>('[aria-label="#FF0000"]')!.click();await step();
  check('選色只修改指定分頁',(await documentApi.state()).tabColors?.[2]==='FF0000'&&(await documentApi.state()).tabColors?.[3]===original[3]);
  await driver.runMenu('undo');await step();check('分頁選色可以復原',(await documentApi.state()).tabColors?.[2]===original[2]);
  await driver.runMenu('redo');await step();check('分頁選色可以重做',(await documentApi.state()).tabColors?.[2]==='FF0000');
  picker=await palette(2);picker.querySelector<HTMLButtonElement>('.reset')!.click();await step();
  check('清除顏色恢復預設外觀',(await documentApi.state()).tabColors?.[2]===''&&!tabs()[2].classList.contains('colored'));
  for(const ext of ['xlsx','wwx']){const copy=await benchApi.saveCopy('test7-cleared-color.'+ext);await documentApi.openPath(copy);await step();await step();check(ext+' 重開不恢復舊主題色',(await documentApi.state()).tabColors?.[2]==='');}
  await documentApi.setTabColor(2,original[2]);await step();
  picker=await palette(2);window.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));await step();check('Escape 關閉選色且保留顏色',!document.querySelector('.tab-color-popup')&&(await documentApi.state()).tabColors?.[2]===original[2]);

  await select(3);
  const settings=await settingsApi.get();await settingsApi.set({...settings,appearance:'dark'});await step();
  check('深色介面保留分頁色彩',tabs().every((tab,i)=>tab.dataset.tabColor===original[i]&&(i===3||getComputedStyle(tab).backgroundColor===rgb(original[i]))));
  await settingsApi.set({...settings,appearance:'light'});await step();
  await documentApi.renameSheet(0,'學期驗收');await step();check('改名後色彩保留',(await documentApi.state()).tabColors?.[0]===original[0]&&tabs()[0].textContent?.trim()==='學期驗收');
  const from=tabs()[3],to=tabs()[0];from.dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));to.dispatchEvent(new PointerEvent('pointerenter',{bubbles:false}));window.dispatchEvent(new PointerEvent('pointerup'));await step();await step();
  let changed=await documentApi.state();check('拖曳分頁後顏色跟著工作表移動',changed.sheets?.[0]==='書籍'&&changed.tabColors?.[0]===original[3]&&tabs()[0].dataset.tabColor===original[3]);
  await driver.runMenu('undo');await step();check('復原移動恢復色彩順序',(await documentApi.state()).tabColors?.join(',')===original.join(','));
  await driver.runMenu('undo');await step();check('復原改名保留原色',(await documentApi.state()).sheets?.[0]==='學期'&&tabs()[0].dataset.tabColor===original[0]);
  const added=await documentApi.addSheet(4);await step();check('新增工作表使用預設分頁樣式',(await documentApi.state()).tabColors?.[added]===''&&!tabs()[added].classList.contains('colored'));
  await documentApi.deleteSheet(added);await step();check('刪除新增分頁不影響既有顏色',(await documentApi.state()).tabColors?.join(',')===original.join(','));
  for(const ext of ['xlsx','wwx']){
    const path=await benchApi.saveCopy('test7-tab-colors.'+ext);await documentApi.openPath(path);await step();await step();changed=await documentApi.state();
    check(ext+' 重開保留四張分頁色彩',changed.tabColors?.join(',')===original.join(',')&&tabs().every((tab,i)=>tab.dataset.tabColor===original[i]));
  }
  const exported=await benchApi.saveCopy('test7-native-tab-colors.xlsx');await documentApi.openPath(exported);await step();await step();
  check('WWX 轉回 XLSX 保留原主題的實際色彩',(await documentApi.state()).tabColors?.join(',')===original.join(','));
  return {failed:checks.filter(c=>!c.ok).length,markdown:'# test7 工作表分頁色彩驗收\n\n'+`${checks.filter(c=>c.ok).length} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}${c.detail?': '+c.detail:''}`).join('\n')+'\n'};
}
