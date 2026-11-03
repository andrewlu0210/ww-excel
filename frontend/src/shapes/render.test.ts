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
