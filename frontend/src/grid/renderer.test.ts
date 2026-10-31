import {afterEach,describe,expect,it,vi} from 'vitest';
import {GridRenderer} from './renderer';
import {AxisLayout} from './layout';
import {single} from './selection';
import type {BlockCache,CellView} from '../stores/viewportCache';

// Capture actual grid drawing calls without a browser or a native canvas.
function renderCell(overrides:Partial<CellView>,merged=false,columnWidth=120) {
 vi.stubGlobal('document',{documentElement:{}});
 vi.stubGlobal('getComputedStyle',()=>({getPropertyValue:()=> '#202020'}));
 const texts:string[]=[],fills:string[]=[];
 const target={fillStyle:'',measureText:(s:string)=>({width:s.length*7}),fillText:(s:string)=>texts.push(s),fill(){fills.push(this.fillStyle)},fillRect(){fills.push(this.fillStyle)}};
 const ctx=new Proxy(target,{get(t,k){return k in t ? t[k as keyof typeof t] : ()=>{}}}) as unknown as CanvasRenderingContext2D;
 const canvas={getContext:()=>ctx,style:{}} as unknown as HTMLCanvasElement;
 const cell:CellView={text:'42',numeric:true,center:false,style:0,color:null,bar:null,...overrides};
 const cache={blockRows:32,blockCols:16,ensure(){},cell:(row:number,col:number)=>row===0 && col===0 ? cell : undefined} as unknown as BlockCache;
 const renderer=new GridRenderer(canvas,new AxisLayout(1,30),new AxisLayout(merged?2:1,columnWidth),cache);
 if(merged) renderer.look.merges=[{r0:0,c0:0,r1:0,c1:1}];
 renderer.resize((merged?2:1)*columnWidth+50,60,1);
 renderer.draw(0,0,single({row:0,col:0}));
 return {texts,fills};
}

afterEach(()=>vi.unstubAllGlobals());

describe('conditional icon fallback in the grid',()=>{
 it.each(['3Arrows','3Flags','3Symbols','5Rating'])('shows the value when icon-only %s cannot be drawn',(set)=>{
  const {texts}=renderCell({hideValue:true,icon:{set,index:1}});
  expect(texts.filter(s=>s==='42')).toHaveLength(1);
 });
 it('also shows the value in a merged cell with an unsupported icon',()=>{
  expect(renderCell({hideValue:true,icon:{set:'3Arrows',index:0}},true).texts).toContain('42');
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
  expect(renderCell({hideValue:true,icon:{set:'3Arrows',index:0}},false,20).texts).toContain('42');
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
