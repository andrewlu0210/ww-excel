import { expect, it } from 'vitest';
import { drawShape } from './render';
import type { ShapeDTO } from '../../bindings/ww-excel/internal/service/models';

it('preserves dotted DrawingML arrows and resets solid outlines', () => {
  const patterns: number[][] = [];
  const target = {setLineDash: (dash:number[]) => patterns.push(dash)};
  const ctx = new Proxy(target,{get(t,k){return k in t ? t[k as keyof typeof t] : () => {};}}) as unknown as CanvasRenderingContext2D;
  const shape = {geometry:'arc',lineColor:'359966',lineWidth:2,runs:[],adjust:{},w:200,h:100} as unknown as ShapeDTO;
  drawShape(ctx,200,100,{...shape,lineDash:'sysDot'});
  drawShape(ctx,200,100,shape);
  expect(patterns).toEqual([[2,2],[]]);
});

it('clips overflowing title lines before vertically centering test9 shapes', () => {
  const drawn: {text:string;y:number}[] = [];
  const target = {
    measureText: (text:string) => ({width: [...text].reduce((sum,ch)=>sum+(ch===' '?12:32),0)}),
    fillText: (text:string,_x:number,y:number) => drawn.push({text,y}),
  };
  const ctx = new Proxy(target,{get(t,k){return k in t ? t[k as keyof typeof t] : () => {};}}) as unknown as CanvasRenderingContext2D;
  const shape = {
    geometry:'roundRect',radius:12,lineWidth:0,
    runs:[{text:'        组织绩效设定与评估',font:{name:'Microsoft YaHei',size:24,bold:true,color:'53617A'}}],
    alignH:'left',alignV:'center',left:9.6,right:9.6,top:4.8,bottom:4.8,
  } as unknown as ShapeDTO;
  drawShape(ctx,356,76.8,{...shape,clipText:true});
  expect(drawn.map(r=>r.text)).toEqual(['        组织绩效设定与']);
  const centerY = drawn[0].y;
  drawn.length=0;
  drawShape(ctx,356,76.8,shape);
  expect(drawn.map(r=>r.text)).toEqual(['        组织绩效设定与','评估']);
  expect(centerY).toBeGreaterThan(drawn[0].y);
});
