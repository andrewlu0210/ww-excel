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
