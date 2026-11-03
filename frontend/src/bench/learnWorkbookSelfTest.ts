// Compare the final tutorial sheet's imported text boxes without following links.
import { benchApi, documentApi } from '../bridge/sheet';
import { drawShape } from '../shapes/render';
import type Sheet from '../components/Sheet.svelte';

export async function learnWorkbookSelfTest(driver: ReturnType<typeof Sheet>): Promise<{markdown:string;failed:number}> {
  const checks:{name:string;ok:boolean;detail:string}[]=[];
  const check=(name:string,ok:boolean,detail='')=>checks.push({name,ok,detail});
  const state=await documentApi.state();
  const index=state.sheets?.indexOf('深入了解') ?? -1;
  const tab=document.querySelectorAll<HTMLElement>('[data-sheet-tabs] [role="tab"]')[index];
  if(!tab)throw new Error('Missing learn-more worksheet');
  tab.dispatchEvent(new PointerEvent('pointerdown',{button:0,bubbles:true,cancelable:true}));
  window.dispatchEvent(new PointerEvent('pointerup'));
  await driver.whenIdle();await document.fonts.ready;await new Promise(r=>setTimeout(r,350));
  const objects=driver.getObjects();
  const cards=objects.filter(o=>o.shape?.runs?.some(r=>r.text==='深入了解'));
  check('三欄教學資訊完整載入',cards.length===3);
  for(const card of cards){
    const shape=card.shape!;
    const title=shape.runs?.find(r=>r.text && !r.text.includes('\n'))?.text ?? '';
    const link=shape.runs?.find(r=>r.text==='深入了解');
    check(`${title}：深入了解為白字粗體底線`,link?.font.color==='FFFFFF' && link.font.bold===true && link.font.underline===true,JSON.stringify(link));
    check(`${title}：空白段落保留 18pt`,shape.runs?.some(r=>r.text==='\n' && r.font.size===18)===true,JSON.stringify(shape.runs));
  }
  const linkPositions:number[]=[];
  for(const card of cards){
    const canvas=document.createElement('canvas');canvas.width=Math.ceil(card.w);canvas.height=Math.ceil(card.h);
    const ctx=canvas.getContext('2d')!;
    const recorded=new Proxy(ctx,{
      get(target,key){
        if(key==='fillText')return(text:string,x:number,y:number)=>{if(text==='深入了解')linkPositions.push(y+card.dy);};
        const value=Reflect.get(target,key,target);
        return typeof value==='function'?value.bind(target):value;
      },
      set(target,key,value){return Reflect.set(target,key,value,target);},
    });
    drawShape(recorded,card.w,card.h,card.shape!);
  }
  check('三欄深入了解連結對齊',linkPositions.length===3 && Math.max(...linkPositions)-Math.min(...linkPositions)<3,JSON.stringify(linkPositions));
  const feedback=objects.find(o=>o.shape?.runs?.map(r=>r.text).join('').includes('提供有關本導覽的意見反應'));
  check('意見反應按鈕文字繼承白色',feedback?.shape?.runs?.filter(r=>r.text.trim()).every(r=>r.font.color==='FFFFFF')===true,JSON.stringify(feedback?.shape?.runs));
  const zoom=document.querySelector<HTMLInputElement>('.zoom input[type="range"]');
  const previous=zoom?.value;
  if(zoom){zoom.value='80';zoom.dispatchEvent(new Event('input',{bubbles:true}));}
  await driver.whenIdle();await new Promise(r=>setTimeout(r,350));
  driver.getGrid()?.scrollToOrigin();await new Promise(r=>setTimeout(r,250));
  await benchApi.snapshot('test5-learn-more',driver.snapshotWithObjects());
  for(const card of cards){
    const canvas=document.querySelector<HTMLCanvasElement>(`[data-object-id="${card.id}"] canvas`);
    if(canvas)await benchApi.snapshot(`test5-learn-card-${card.id}`,canvas.toDataURL());
  }
  if(zoom && previous){zoom.value=previous;zoom.dispatchEvent(new Event('input',{bubbles:true}));}
  await driver.whenIdle();
  check('檢查未修改文件',!(await documentApi.state()).dirty);
  const failed=checks.filter(c=>!c.ok).length;
  return{failed,markdown:'# test5.xlsx 深入了解顯示相容性\n\n'+`${checks.length-failed} / ${checks.length} 通過\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}: ${c.detail}`).join('\n')+'\n'};
}
