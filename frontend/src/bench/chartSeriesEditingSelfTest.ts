import {benchApi,documentApi,sheetApi} from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';
export async function chartSeriesEditingSelfTest(driver:ReturnType<typeof Sheet>):Promise<{name:string;ok:boolean}[]> {
 const checks:{name:string;ok:boolean}[]=[];const check=(name:string,ok:boolean)=>checks.push({name,ok});
 const step=async()=>{await driver.whenIdle();await new Promise(r=>setTimeout(r,180));};
 let id=driver.getObjects().find(o=>o.chart?.type==='combo')!.id;
 const object=async()=>((await sheetApi.layout(0,0)).objects ?? []).find(o=>o.id===id)!;
 const open=async()=>{driver.selectObject(id);await driver.runMenu('editChart');await step();return document.querySelector<HTMLElement>('.chart-dialog')!;};
 const click=(root:ParentNode,label:string)=>[...root.querySelectorAll<HTMLButtonElement>('button')].find(b=>b.textContent?.trim()===label)!.click();
 const set=(root:ParentNode,label:string,value:string)=>{const field=root.querySelector<HTMLInputElement|HTMLSelectElement>(`[aria-label="${label}"]`)!;field.value=value;field.dispatchEvent(new Event('input',{bubbles:true}));field.dispatchEvent(new Event('change',{bubbles:true}));};
 const before=(await object()).chart!;
 let dialog=await open();check('數列外觀編輯介面可開啟',!!dialog.querySelector('.series-editor'));
 set(dialog,'數列色彩','#ff0000');set(dialog,'柱距（%）','75');set(dialog,'編輯數列','1');await step();set(dialog,'數列色彩','#00aa00');set(dialog,'折線粗細','4.25');set(dialog,'資料點標記','star');click(dialog,'確定');await step();
 let c=(await object()).chart!;check('雙軸數列的色彩、粗細與標記分別套用',c.series?.[0].color?.toUpperCase()==='FF0000'&&c.series[1].color?.toUpperCase()==='00AA00'&&c.series[1].marker==='star'&&c.series[1].lineWidth===4.25&&c.gapWidth===75);
 await driver.runMenu('undo');await step();check('數列外觀可一次復原',(await object()).chart?.gapWidth===before.gapWidth&&(await object()).chart?.series?.[1].marker===before.series?.[1].marker);
 await driver.runMenu('redo');await step();check('數列外觀可重做',(await object()).chart?.series?.[1].marker==='star');
 dialog=await open();set(dialog,'編輯數列','1');await step();set(dialog,'折線粗細','9');click(dialog,'取消');await step();check('取消不套用外觀變更',(await object()).chart?.series?.[1].lineWidth===4.25);
 dialog=await open();set(dialog,'柱距（%）','0');click(dialog,'確定');await step();check('零柱距設定有效',(await object()).chart?.gapWidth===0);
 await driver.runMenu('undo');await step();const comboID=id;
 const source=(await object()).chart!.series![0];
 const ring=await sheetApi.addChart(0,66,3,500,300,{type:'doughnut',title:'環圈外觀驗收',legend:'right',xTitle:'',yTitle:'',stacked:false,holeSize:50,series:[{...source,type:undefined,secondary:false}]});if(ring.error||!ring.value)throw Error(ring.error ?? 'Missing doughnut');id=ring.value;await step();
 dialog=await open();set(dialog,'環圈內徑（%）','70');click(dialog,'確定');await step();check('環圈內徑可調整',(await object()).chart?.holeSize===70);
 for(const ext of ['xlsx','wwx']){
  const copy=await benchApi.saveCopy('series-style-edited.'+ext);await documentApi.openPath(copy);await step();await step();
  const objects=(await sheetApi.layout(0,0)).objects ?? [];c=objects.find(o=>o.id===comboID)!.chart!;
  check(ext+' 保留數列外觀及環圈設定',c.series?.[0].color?.toUpperCase()==='FF0000'&&c.series[1].color?.toUpperCase()==='00AA00'&&c.series[1].marker==='star'&&Math.abs(c.series[1].lineWidth!-4.25)<1e-4&&c.gapWidth===75&&objects.find(o=>o.id===id)?.chart?.holeSize===70);
 }
 const images=driver.chartImages();if(images[comboID])await benchApi.snapshot('series-style-edited',images[comboID]);
 const settings=await sheetApi.pageSetup(0);await sheetApi.setPageSetup(0,{...settings,landscape:true,fitWide:1,printArea:{r0:21,c0:2,r1:42,c1:13}});await step();
 const preview=await sheetApi.printPreview(0,driver.chartImages());check('新數列外觀可產生 PDF 預覽',!preview.error&&!!preview.value?.url&&preview.value.pages>=1);
 const pdf=await benchApi.savePDF('series-style-preview.pdf',0,driver.chartImages());check('實際 PDF 可匯出',pdf.endsWith('.pdf'));
 return checks;
}
