import {afterEach,describe,expect,it,vi} from 'vitest';
import {GridRenderer} from './renderer';
import {AxisLayout} from './layout';
import {single} from './selection';
import type {BlockCache,CellView} from '../stores/viewportCache';

// Capture actual grid drawing calls without a browser or a native canvas.
function renderCell(overrides:Partial<CellView>,merged=false,columnWidth=120) {
 vi.stubGlobal('document',{documentElement:{}});
 vi.stubGlobal('getComputedStyle',()=>({getPropertyValue:()=> '#202020'}));
 const texts:string[]=[],fills:string[]=[],segments:number[][]=[];let lastPoint=[0,0];
 const target={moveTo:(x:number,y:number)=>{lastPoint=[x,y]},lineTo:(x:number,y:number)=>segments.push([...lastPoint,x,y]),fillStyle:'',measureText:(s:string)=>({width:s.length*7}),fillText:(s:string)=>texts.push(s),fill(){fills.push(this.fillStyle)},fillRect(){fills.push(this.fillStyle)}};
 const ctx=new Proxy(target,{get(t,k){return k in t ? t[k as keyof typeof t] : ()=>{}}}) as unknown as CanvasRenderingContext2D;
 const canvas={getContext:()=>ctx,style:{}} as unknown as HTMLCanvasElement;
 const cell:CellView={text:'42',numeric:true,center:false,style:0,color:null,bar:null,...overrides};
 const cache={blockRows:32,blockCols:16,ensure(){},cell:(row:number,col:number)=>row===0 && col===0 ? cell : undefined} as unknown as BlockCache;
 const renderer=new GridRenderer(canvas,new AxisLayout(1,30),new AxisLayout(merged?2:1,columnWidth),cache);
 if(merged) renderer.look.merges=[{r0:0,c0:0,r1:0,c1:1}];
 renderer.resize((merged?2:1)*columnWidth+50,60,1);
 renderer.draw(0,0,single({row:0,col:0}));
 return {texts,fills,segments};
}

afterEach(()=>vi.unstubAllGlobals());

describe('conditional icon fallback in the grid',()=>{
 it.each(['FutureArrows','FutureFlags','FutureSymbols','FutureRating'])('shows the value when icon-only %s cannot be drawn',(set)=>{
  const {texts}=renderCell({hideValue:true,icon:{set,index:1}});
  expect(texts.filter(s=>s==='42')).toHaveLength(1);
 });
 it('also shows the value in a merged cell with an unsupported icon',()=>{
  expect(renderCell({hideValue:true,icon:{set:'FutureArrows',index:0}},true).texts).toContain('42');
 });
 it('falls back for an invalid icon index',()=>{
  expect(renderCell({hideValue:true,icon:{set:'3TrafficLights1',index:99}}).texts).toContain('42');
 });
 it.each(['3TrafficLights1','3TrafficLights2','4TrafficLights'])('keeps supported %s icon-only cells hidden',(set)=>{
  const {texts,fills}=renderCell({hideValue:true,icon:{set,index:0}});
  expect(texts).not.toContain('42');
  expect(fills).toContain(set==='4TrafficLights'?'#222222':'#e04040');
 });
 it('still shows the number beside a supported icon when values are enabled',()=>{
  const {texts,fills}=renderCell({hideValue:false,icon:{set:'4TrafficLights',index:0}});
  expect(texts).toContain('42');expect(fills).toContain('#222222');
 });
 it('does not reserve icon space when falling back in a narrow cell',()=>{
  expect(renderCell({hideValue:true,icon:{set:'FutureArrows',index:0}},false,20).texts).toContain('42');
 });
 it('keeps a supported icon visible without numeric overflow hashes in a narrow status cell',()=>{
  const {texts,fills}=renderCell({hideValue:false,icon:{set:'4TrafficLights',index:0}},false,20);
  expect(fills).toContain('#222222');expect(texts).not.toContain('42');expect(texts.some(s=>s.includes('#'))).toBe(false);
 });
 it('keeps data-bar-only values hidden when there is no icon',()=>{
  const {texts,fills}=renderCell({hideValue:true,bar:{len:.5,color:'#F1C94D',solid:true}});
  expect(texts).not.toContain('42');expect(fills).toContain('#F1C94D');
 });
});

it('draws cached pivot dropdowns alongside table dropdowns without filtering their cells',()=>{
 vi.stubGlobal('document',{documentElement:{}});
 vi.stubGlobal('getComputedStyle',()=>({getPropertyValue:()=> '#202020'}));
 const buttons:number[][]=[];
 const target={measureText:(s:string)=>({width:s.length*7}),strokeStyle:'',strokeRect(...rect:number[]){if(this.strokeStyle==='#8a8a8a')buttons.push(rect)}};
 const ctx=new Proxy(target,{get(t,k){return k in t ? t[k as keyof typeof t] : ()=>{}}}) as unknown as CanvasRenderingContext2D;
 const canvas={getContext:()=>ctx,style:{}} as unknown as HTMLCanvasElement;
 const cache={blockRows:32,blockCols:16,ensure(){},cell:()=>undefined} as unknown as BlockCache;
 const renderer=new GridRenderer(canvas,new AxisLayout(4,30),new AxisLayout(2,100),cache);
 renderer.look.tableFilters=[{r0:0,c0:0,r1:1,c1:1}];
 renderer.look.pivotHeaders=[{r0:2,c0:0,r1:2,c1:0}];
 renderer.resize(250,160,1);
 renderer.draw(0,0,single({row:3,col:1}));
 expect(buttons).toHaveLength(3);
 expect(buttons[0][1]).toBe(buttons[1][1]);
 expect(buttons[2][1]-buttons[0][1]).toBe(60);
 expect(renderer.look.filter).toBeNull();
 expect(renderer.look.filtered.size).toBe(0);
});

import {ICON_SETS} from './iconSets';
it.each(ICON_SETS)('shows or hides values correctly for $id, including merged and narrow cells',({id})=>{
 expect(renderCell({hideValue:false,icon:{set:id,index:0}}).texts).toContain('42');
 expect(renderCell({hideValue:true,icon:{set:id,index:0}}).texts).not.toContain('42');
 expect(renderCell({hideValue:true,icon:{set:id,index:0}},true).texts).not.toContain('42');
 const narrow=renderCell({hideValue:false,icon:{set:id,index:0}},false,20).texts;
 expect(narrow).not.toContain('42');expect(narrow.some(s=>s.includes('#'))).toBe(false);
});

import {styleStore} from '../stores/styles';
import type {StyleDTO} from '../bridge/sheet';
it.each([
 ['double',1,[],2],['thick',3,[],1],['dotted',1,[1,1],1],['mediumDashDot',2,[6,2,2,2],1],
] as const)('draws edited %s borders with the chosen color and stroke', (style,width,dash,count)=>{
 vi.stubGlobal('document',{documentElement:{}});
 vi.stubGlobal('getComputedStyle',()=>({getPropertyValue:()=> '#202020'}));
 const strokes:{color:string;width:number;dash:number[]}[]=[];
 const target={strokeStyle:'',lineWidth:1,dash:[] as number[],measureText:(s:string)=>({width:s.length*7}),setLineDash(d:number[]){this.dash=[...d]},stroke(){if(this.strokeStyle==='#A1B2C3')strokes.push({color:this.strokeStyle,width:this.lineWidth,dash:[...this.dash]})}};
 const ctx=new Proxy(target,{get(t,k){return k in t?t[k as keyof typeof t]:()=>{}}}) as unknown as CanvasRenderingContext2D;
 const canvas={getContext:()=>ctx,style:{}} as unknown as HTMLCanvasElement;
 const dto:StyleDTO={id:201,font:{name:'',size:11},top:{},bottom:{style,color:'A1B2C3'},left:{},right:{}};
 styleStore.merge([dto]);
 const cell:CellView={text:'',numeric:false,center:false,style:201,color:null,bar:null};
 const cache={blockRows:64,blockCols:32,ensure(){},cell:(r:number,c:number)=>r===0&&c===0?cell:undefined} as unknown as BlockCache;
 const renderer=new GridRenderer(canvas,new AxisLayout(1,30),new AxisLayout(1,100),cache);
 renderer.resize(160,90,1);renderer.draw(0,0,single({row:0,col:0}));
 expect(strokes).toHaveLength(count);
 expect(strokes.every(s=>s.width===width&&JSON.stringify(s.dash)===JSON.stringify(dash))).toBe(true);
});

it.each(['112233','FFFFFF',''])('draws solid fill %s independently of the grid background', fill=>{
 styleStore.merge([{id:202,font:{name:'',size:11},fill,top:{},bottom:{},left:{},right:{}}]);
 for(const merged of [false,true]) {
  const result=renderCell({style:202},merged);
  if(fill)expect(result.fills).toContain(`#${fill}`);
  else expect(result.fills).not.toContain('#FFFFFF');
 }
});


it.each([false,true])('draws both diagonal directions across the complete logical cell (merged=%s)',merged=>{
 styleStore.merge([{id:303,font:{name:'',size:11},top:{},bottom:{},left:{},right:{},diagonal:{style:'thin',color:'FF0000'},diagonalUp:true,diagonalDown:true}]);
 const {segments}=renderCell({style:303},merged);
 const width=merged?240:120;
 expect(segments.some(([x,y,a,b])=>Math.abs(a-x)===width && b-y===30)).toBe(true);
 expect(segments.some(([x,y,a,b])=>Math.abs(a-x)===width && b-y===-30)).toBe(true);
});

it('keeps double diagonal strokes separated perpendicular to the line',()=>{
 styleStore.merge([{id:304,font:{name:'',size:11},top:{},bottom:{},left:{},right:{},diagonal:{style:'double'},diagonalDown:true}]);
 const strokes=renderCell({style:304}).segments.filter(([x,y,a,b])=>Math.abs(a-x-120)<1e-6 && Math.abs(b-y-30)<1e-6);
 expect(strokes).toHaveLength(2);
 const [a,b]=strokes,dx=b[0]-a[0],dy=b[1]-a[1];
 expect(Math.hypot(dx,dy)).toBeCloseTo(2);
 expect(dx*120+dy*30).toBeCloseTo(0);
});
