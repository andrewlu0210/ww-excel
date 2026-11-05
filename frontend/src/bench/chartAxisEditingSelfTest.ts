import {benchApi,documentApi,sheetApi} from '../bridge/sheet';
import type Sheet from '../components/Sheet.svelte';

export async function chartAxisEditingSelfTest(driver:ReturnType<typeof Sheet>):Promise<{name:string;ok:boolean}[]> {
  const checks:{name:string;ok:boolean}[]=[];const check=(name:string,ok:boolean)=>checks.push({name,ok});
  const step=async()=>{await driver.whenIdle();await new Promise(r=>setTimeout(r,220));};
  let id=driver.getObjects().find(o=>o.chart)!.id;
  const object=async()=>((await sheetApi.layout(0,0)).objects ?? []).find(o=>o.id===id)!;
  const open=async()=>{driver.selectObject(id);await driver.runMenu('editChart');await step();return document.querySelector<HTMLElement>('.chart-dialog')!;};
  const click=(root:ParentNode,text:string)=>{const b=[...root.querySelectorAll<HTMLButtonElement>('button')].find(b=>b.textContent?.trim()===text);if(!b)throw Error('Missing button '+text);b.click();};
  const set=(root:ParentNode,label:string,value:string)=>{const input=root.querySelector<HTMLInputElement|HTMLSelectElement>(`[aria-label="${label}"]`)!;if(!input)throw Error('Missing axis input '+label);input.value=value;input.dispatchEvent(new Event('input',{bubbles:true}));input.dispatchEvent(new Event('change',{bubbles:true}));};
  let dialog=await open();check('座標軸設定介面可開啟',!!dialog.querySelector('.axis-editor'));
  set(dialog,'座標軸最小值','-100');set(dialog,'座標軸最大值','400');set(dialog,'主要刻度間距','100');set(dialog,'座標軸數值格式','"NT$"#,##0');set(dialog,'座標軸字型','Arial');set(dialog,'座標軸字型大小','14');set(dialog,'軸線色彩','#AA0000');set(dialog,'座標軸刻度線','cross');click(dialog,'確定');await step();
  let obj=await object();let axis=obj.chart!.valueAxis;let data=await sheetApi.chartData(0,id);
  check('數值軸固定範圍與格式已套用',data.valueScale?.min===-100&&data.valueScale.max===400&&data.valueScale.step===100&&data.valueScale.labels?.[1]==='NT$0');
  check('數值軸字型、軸線與刻度已套用',axis?.fontSize===14&&axis.fontName==='Arial'&&axis.lineColor.toUpperCase()==='AA0000'&&axis.majorTick==='cross');
  await driver.runMenu('undo');await step();check('座標軸設定可一次復原',(await object()).chart?.valueAxis?.maximum!==400);
  await driver.runMenu('redo');await step();check('座標軸設定可重做',(await object()).chart?.valueAxis?.maximum===400);
  dialog=await open();set(dialog,'座標軸最大值','900');click(dialog,'取消');await step();check('取消不變更座標軸',(await object()).chart?.valueAxis?.maximum===400);
  dialog=await open();set(dialog,'座標軸最小值','500');click(dialog,'確定');await step();check('拒絕最小值大於最大值',!!document.querySelector('.chart-dialog .error')&&(await object()).chart?.valueAxis?.minimum===-100);click(dialog,'取消');await step();
  dialog=await open();set(dialog,'編輯座標軸','categoryAxis');await step();set(dialog,'座標軸字型大小','12');dialog.querySelector<HTMLInputElement>('[aria-label="主要格線"]')!.click();click(dialog,'確定');await step();check('類別軸設定獨立套用',(await object()).chart?.categoryAxis?.fontSize===12&&(await object()).chart?.categoryAxis?.gridLines===true&&(await object()).chart?.valueAxis?.fontSize===14);
  dialog=await open();click(dialog,'恢復自動範圍');click(dialog,'確定');await step();axis=(await object()).chart!.valueAxis;
  check('可恢復自動範圍且保留字型與格式',!!axis&&axis.minimum==null&&axis.maximum==null&&axis.majorUnit===0&&axis.fontSize===14&&axis.format==='"NT$"#,##0');
  await driver.runMenu('undo');await step();
  const source=(await object()).chart!;const series=source.series![0];
  const combo=await sheetApi.addChart(0,22,3,600,340,{...source,type:'combo',series:[{...series,type:'column',secondary:false},{...series,type:'line',secondary:true}],secondaryAxis:{format:'General',majorUnit:0,lineColor:'',lineHidden:false,hidden:false,majorTick:'none',gridLines:false,hideLabels:false}});if(combo.error||!combo.value)throw Error(combo.error ?? 'Missing combo');id=combo.value;await step();
  dialog=await open();set(dialog,'編輯座標軸','secondaryAxis');await step();set(dialog,'座標軸最小值','0');set(dialog,'座標軸最大值','1000');set(dialog,'主要刻度間距','250');click(dialog,'確定');await step();data=await sheetApi.chartData(0,id);
  check('雙軸左右範圍保持獨立',data.valueScale?.max===400&&data.secondaryScale?.max===1000&&data.secondaryScale.step===250);
  const comboID=id;
  const scatter=await sheetApi.addChart(0,44,3,600,340,{...source,type:'scatter',series:[{...series,type:undefined,secondary:false,categories:series.values}],categoryAxis:{format:'General',majorUnit:0,lineColor:'',lineHidden:false,hidden:false,majorTick:'none',gridLines:true,hideLabels:false}});if(scatter.error||!scatter.value)throw Error(scatter.error ?? 'Missing scatter');id=scatter.value;await step();
  dialog=await open();set(dialog,'編輯座標軸','categoryAxis');await step();set(dialog,'座標軸最小值','-100');set(dialog,'座標軸最大值','500');set(dialog,'主要刻度間距','100');set(dialog,'編輯座標軸','valueAxis');await step();set(dialog,'座標軸最小值','0');set(dialog,'座標軸最大值','1000');set(dialog,'主要刻度間距','250');click(dialog,'確定');await step();data=await sheetApi.chartData(0,id);
  check('散佈圖 X／Y 軸刻度保持獨立',data.xScale?.min===-100&&data.xScale.max===500&&data.xScale.step===100&&data.valueScale?.min===0&&data.valueScale.max===1000&&data.valueScale.step===250);
  for(const ext of ['xlsx','wwx']) {
    const path=await benchApi.saveCopy('chart-axes-edited.'+ext);await documentApi.openPath(path);await step();await step();
    data=await sheetApi.chartData(0,id);const comboData=await sheetApi.chartData(0,comboID);
    check(ext+' 重開保留雙軸與散佈圖設定',data.xScale?.max===500&&data.valueScale?.max===1000&&comboData.secondaryScale?.max===1000&&comboData.valueScale?.max===400);
  }
  const image=driver.chartImages()[id];if(image)await benchApi.snapshot('chart-axis-edited',image);
  return checks;
}
