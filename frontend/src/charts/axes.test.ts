import {describe,it,expect} from 'vitest';
import {drawChart,type ChartSpec,type ChartData} from './render';
function context(){
 const text:{text:string;x:number;y:number;font:string}[]=[],strokes:string[]=[];const stack:{font:string;strokeStyle:string}[]=[];
 const target={font:'',strokeStyle:'',save(){stack.push({font:this.font,strokeStyle:this.strokeStyle})},restore(){Object.assign(this,stack.pop() ?? {})},measureText:(v:string)=>({width:v.length*7}),fillText(v:string,x:number,y:number){text.push({text:v,x,y,font:this.font})},stroke(){strokes.push(this.strokeStyle)}};
 const ctx=new Proxy(target,{get:(t,k)=>k in t?t[k as keyof typeof t]:()=>{}}) as unknown as CanvasRenderingContext2D;
 return {ctx,text,strokes};
}
const base:ChartSpec={type:'column',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false};
const data:ChartData={categories:['A','B'],series:[{name:'Sales',values:[10,20],x:[]}],valueScale:{min:0,max:20,step:10,labels:['Low','Mid','High']}};
describe('axis settings in canvas charts',()=>{
 it.each(['column','bar'])('uses backend bounds, tick formatting and font for %s',type=>{
  const {ctx,text}=context();drawChart(ctx,500,350,{...base,type,valueAxis:{format:'0',majorUnit:10,fontName:'Arial',fontSize:12,lineColor:'FF0000',lineHidden:false,hidden:false,majorTick:'out',gridLines:true,hideLabels:false}},data);
  const low=text.find(t=>t.text==='Low')!,high=text.find(t=>t.text==='High')!;expect(low).toBeDefined();expect(high).toBeDefined();expect(low.font).toContain('16px');expect(low.font).toContain('Arial');
  if(type==='bar')expect(low.x).toBeLessThan(high.x);else expect(low.y).toBeGreaterThan(high.y);
 });
 it('hides value tick labels without hiding category labels',()=>{
  const {ctx,text}=context();drawChart(ctx,500,350,{...base,valueAxis:{format:'0',majorUnit:10,lineColor:'',lineHidden:false,hidden:false,majorTick:'none',gridLines:true,hideLabels:true}},data);
  expect(text.some(t=>t.text==='Low')).toBe(false);expect(text.some(t=>t.text==='A')).toBe(true);
 });
 it('uses independent right axis formatting and font in combo charts',()=>{
  const {ctx,text}=context();drawChart(ctx,600,350,{...base,type:'combo',series:[{type:'column'},{type:'line',secondary:true}],secondaryAxis:{format:'0',majorUnit:50,fontSize:18,lineColor:'',lineHidden:false,hidden:false,majorTick:'cross',gridLines:false,hideLabels:false}},
   {...data,series:[...data.series,{name:'Other',values:[50,100],x:[]}],secondaryScale:{min:0,max:100,step:50,labels:['R0','R50','R100']}});
  expect(text.find(t=>t.text==='R50')?.font).toContain('24px');expect(text.some(t=>t.text==='High')).toBe(true);
 });
 it('uses independent scatter scales and excludes points outside the configured range',()=>{
  const {ctx,text}=context();const hits:import('./labels').ChartLabelHit[]=[];
  drawChart(ctx,500,350,{...base,type:'scatter',series:[{labels:{showValue:true}}]},
   {categories:[],series:[{name:'XY',x:[5,15,5],values:[50,50,150],labels:['Visible','Outside X','Outside Y']}],xScale:{min:0,max:10,step:10,labels:['X0','X10']},valueScale:{min:0,max:100,step:100,labels:['Y0','Y100']}},undefined,hits);
  expect(text.map(t=>t.text)).toEqual(expect.arrayContaining(['X0','X10','Y0','Y100','Visible']));expect(text.some(t=>t.text.startsWith('Outside'))).toBe(false);expect(hits.map(h=>h.point)).toEqual([0]);
 });
 it('draws category gridlines only when requested',()=>{
  const grid=(enabled:boolean)=>{const {ctx,strokes}=context();drawChart(ctx,500,350,{...base,categoryAxis:{format:'General',majorUnit:0,lineColor:'',lineHidden:true,hidden:false,majorTick:'none',gridLines:enabled,hideLabels:false}},data);return strokes.length;};
  expect(grid(true)-grid(false)).toBe(2);
 });
 it.each(['column','bar','line','area'])('does not show labels for %s points outside fixed bounds',type=>{
   const {ctx,text}=context();drawChart(ctx,500,350,{...base,type,series:[{labels:{showValue:true}}]},
    {categories:['A','B','C'],series:[{name:'Values',x:[],values:[-50,50,150],labels:['Outside low','Inside','Outside high']}],valueScale:{min:0,max:100,step:50,labels:['0','50','100']}});
   expect(text.some(t=>t.text==='Inside')).toBe(true);expect(text.some(t=>t.text.startsWith('Outside'))).toBe(false);
 });

});
