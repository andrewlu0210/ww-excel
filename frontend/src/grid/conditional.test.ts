import {describe,it,expect} from 'vitest';
import {drawConditionalIcon,drawDataBar} from './conditional';

describe('Excel conditional visuals',()=>{
 it('draws zero-based 4TrafficLights icon 0 as the black circle used by the workbook',()=>{
  const fills:string[]=[];
  const target={save(){},restore(){},fillStyle:'',beginPath(){},arc(){},fill(){fills.push(this.fillStyle)}};
  const ctx=target as unknown as CanvasRenderingContext2D;
  expect(drawConditionalIcon(ctx,{set:'4TrafficLights',index:0},0,0,10)).toBe(true);
  expect(fills).toEqual(['#222222']);
  expect(drawConditionalIcon(ctx,{set:'NoIcons',index:0},0,0,10)).toBe(false);
  expect(fills).toHaveLength(1);
 });
 it('preserves solid yellow bars, their fixed percentage and border without a gradient',()=>{
  const fills:unknown[][]=[];const borders:unknown[][]=[];
  const target={save(){},restore(){},fillStyle:'',strokeStyle:'',lineWidth:1,fillRect(...args:number[]){fills.push([this.fillStyle,...args])},strokeRect(...args:number[]){borders.push([this.strokeStyle,...args])},createLinearGradient(){throw new Error('Solid bar must not create gradient')}};
  const ctx=target as unknown as CanvasRenderingContext2D;
  drawDataBar(ctx,{len:.5,color:'#F1C94D',solid:true,borderColor:'#F1C94D'},0,0,104,25);
  expect(fills).toEqual([['#F1C94D',2,2,50,20]]);
  expect(borders).toEqual([['#F1C94D',2.5,2.5,49,19]]);
  drawDataBar(ctx,{len:0,color:'#F1C94D',solid:true},0,0,104,25);
  expect(fills).toHaveLength(1);
 });
});

import { ICON_SETS } from './iconSets';

function recordIcons() {
 const calls:{name:string;args:unknown[];fill:string}[]=[];
 const target={fillStyle:'',strokeStyle:'',lineWidth:1};
 const ctx=new Proxy(target,{get(t,k){if(k in t)return t[k as keyof typeof t];return(...args:unknown[])=>calls.push({name:String(k),args,fill:t.fillStyle});}}) as unknown as CanvasRenderingContext2D;
 return {ctx,calls};
}

describe('all Excel icon sets',()=>{
 it.each(ICON_SETS)('draws every ascending state of $id',({id,count})=>{
  const {ctx,calls}=recordIcons();
  for(let index=0;index<count;index++)expect(drawConditionalIcon(ctx,{set:id,index},4,7,16)).toBe(true);
  expect(calls.filter(c=>c.name==='save').length).toBeGreaterThanOrEqual(count);
  expect(calls.filter(c=>c.name==='restore')).toHaveLength(calls.filter(c=>c.name==='save').length);
  expect(calls.some(c=>c.name==='fill' || c.name==='fillRect' || c.name==='stroke')).toBe(true);
  expect(calls.some(c=>c.name==='fillText')).toBe(false);
 });
 it('rejects unsupported sets, invalid states and invisible geometry without drawing',()=>{
  const {ctx,calls}=recordIcons();
  for(const icon of [{set:'Unknown',index:0},{set:'NoIcons',index:0},{set:'3Arrows',index:-1},{set:'3Flags',index:3},{set:'5Boxes',index:5},{set:'3Signs',index:1.5}])expect(drawConditionalIcon(ctx,icon,0,0,10)).toBe(false);
  for(const size of [0,-1,NaN,Infinity])expect(drawConditionalIcon(ctx,{set:'3Arrows',index:0},0,0,size)).toBe(false);
  expect(calls).toHaveLength(0);
 });
 it('draws red down, yellow right and green up arrows',()=>{
  const {ctx,calls}=recordIcons();
  for(let i=0;i<3;i++)drawConditionalIcon(ctx,{set:'3Arrows',index:i},0,0,16);
  expect(calls.filter(c=>c.name==='rotate').map(c=>c.args[0])).toEqual([Math.PI,Math.PI/2,0]);
  expect(calls.filter(c=>c.name==='fill').map(c=>c.fill)).toEqual(['#e04040','#f1c232','#52a34a']);
 });
 it('distinguishes circled and plain symbols, and fills increasing rating bars',()=>{
  const circle=recordIcons(),plain=recordIcons();
  drawConditionalIcon(circle.ctx,{set:'3Symbols',index:0},0,0,16);
  drawConditionalIcon(plain.ctx,{set:'3Symbols2',index:0},0,0,16);
  expect(circle.calls.some(c=>c.name==='arc')).toBe(true);
  expect(plain.calls.some(c=>c.name==='arc')).toBe(false);
  for(let index=0;index<5;index++){
   const {ctx,calls}=recordIcons();drawConditionalIcon(ctx,{set:'5Rating',index},0,0,16);
   expect(calls.filter(c=>c.name==='fillRect' && c.fill==='#426d91')).toHaveLength(index+1);
  }
 });
});
