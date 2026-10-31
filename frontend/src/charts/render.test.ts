import { describe, expect, it } from 'vitest';
import { categoryScale, drawChart, formatTick, niceScale } from './render';

describe('niceScale', () => {
  it('rounds bounds to a 1-2-2.5-5 step and starts at zero', () => {
    expect(niceScale(70, 150)).toEqual({ min: 0, max: 150, step: 50 });
    expect(niceScale(0, 1)).toEqual({ min: 0, max: 1, step: 0.2 });
    expect(niceScale(-30, 80)).toEqual({ min: -50, max: 100, step: 25 });
    expect(niceScale(5, 5)).toEqual({ min: 0, max: 5, step: 1 });
  });
  it('keeps a raised baseline for clustered values', () => {
    const s = niceScale(1000, 1100);
    expect(s.min).toBe(1000);
    expect(s.max).toBe(1100);
  });
});

describe('formatTick', () => {
  it('groups thousands and keeps the step precision', () => {
    expect(formatTick(12500, 2500)).toBe('12,500');
    expect(formatTick(0.4, 0.2)).toBe('0.4');
  });
});

describe('chart compatibility', () => {
  it('includes zero for positive and negative column, bar and area values', () => {
    for (const type of ['column', 'bar', 'area']) {
      expect(categoryScale(type, 70080, 112500).min).toBe(0);
      expect(categoryScale(type, -112500, -70080).max).toBe(0);
    }
    expect(categoryScale('line', 1000, 1100).min).toBe(1000);
  });

  it('draws doughnut slices with a reverse inner arc instead of filling the center', () => {
    const arcs: number[][] = [];
    const moves: number[][] = [];
    const ctx = new Proxy({arc: (...args: number[]) => arcs.push(args), moveTo: (...args:number[]) => moves.push(args), measureText: () => ({width: 10})}, {
      get(target, key) {return key in target ? target[key as keyof typeof target] : () => {};},
    }) as unknown as CanvasRenderingContext2D;
    drawChart(ctx, 300, 300, {type:'doughnut',holeSize:75,title:'',legend:'none',xTitle:'',yTitle:'',stacked:false},
      {categories:['a','b'],series:[{name:'',values:[25,75],x:[]}]});
    expect(arcs).toHaveLength(4);
    expect(arcs[1][2]).toBeCloseTo(arcs[0][2] * .75);
    expect(arcs[1][5]).toBe(true);
    expect(moves).toHaveLength(0);
  });
});

describe('chart backgrounds', () => {
  it('clears transparent charts and preserves explicit/default solid backgrounds', () => {
    const fills: unknown[][] = [];
    const clears: unknown[][] = [];
    const ctx = {save(){},restore(){},fillStyle:'',fillRect(...args:unknown[]){fills.push(args)},clearRect(...args:unknown[]){clears.push(args)}} as unknown as CanvasRenderingContext2D;
    const spec={type:'doughnut',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false};
    const data={categories:[],series:[]};
    drawChart(ctx,300,200,{...spec,background:'D9D9D0'},data);
    expect(ctx.fillStyle).toBe('#D9D9D0');
    expect(fills).toHaveLength(1);
    drawChart(ctx,300,200,{...spec,background:'transparent'},data);
    expect(clears).toEqual([[0,0,300,200]]);
    expect(fills).toHaveLength(1);
    drawChart(ctx,300,200,spec,data);
    expect(ctx.fillStyle).toBe('#ffffff');
    expect(fills).toHaveLength(2);
  });
});

describe('imported doughnut labels and colors', () => {
  it('draws only the visible KPI at the center using its number format and theme colors', () => {
    const texts:{text:string;x:number;y:number;font:string;color:string}[]=[];
    const fills:string[]=[];
    const target={fillStyle:'',font:'',fill(){fills.push(this.fillStyle)},fillText(text:string,x:number,y:number){texts.push({text,x,y,font:this.font,color:this.fillStyle})},measureText:()=>({width:10})};
    const ctx=new Proxy(target,{get(t,k){return k in t?t[k as keyof typeof t]:()=>{}}}) as unknown as CanvasRenderingContext2D;
    drawChart(ctx,300,300,{type:'doughnut',holeSize:75,title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,series:[{pointColors:{0:'69BBFE',1:'0592FE'},labels:{fontSize:53,color:'666666',points:{0:{hidden:true,center:false},1:{hidden:false,center:true}}}}]},
      {categories:['a','b'],series:[{name:'',values:[.38,.62],x:[],labels:['','62%']}]});
    expect(fills).toEqual(['#69BBFE','#0592FE']);
    expect(texts).toHaveLength(1);
    expect(texts[0]).toMatchObject({text:'62%',x:150,y:150,color:'#666666'});
    expect(parseFloat(texts[0].font)).toBeCloseTo(53*96/72);
  });
});

describe('imported chart axes', () => {
  it('draws axis lines and outward ticks without inventing gridlines, and uses formatted currency labels', () => {
    const segments:number[][]=[];
    const texts:string[]=[];
    let from:number[]=[];
    const target={measureText:(s:string)=>({width:s.length*6}),moveTo:(x:number,y:number)=>{from=[x,y]},lineTo:(x:number,y:number)=>segments.push([...from,x,y]),fillText:(s:string)=>texts.push(s)};
    const ctx=new Proxy(target,{get(t,k){return k in t?t[k as keyof typeof t]:()=>{}}}) as unknown as CanvasRenderingContext2D;
    const axis={lineColor:'B3B3AD',lineHidden:false,hidden:false,majorTick:'out',gridLines:false,hideLabels:false};
    drawChart(ctx,300,300,{type:'column',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,valueAxis:axis,categoryAxis:{...axis,majorTick:'none'}},
      {categories:[''],series:[{name:'Income',values:[112500],x:[]}],valueScale:{min:0,max:120000,step:20000,labels:['NT$0','NT$20,000','NT$40,000','NT$60,000','NT$80,000','NT$100,000','NT$120,000']}});
    expect(texts).toContain('NT$120,000');
    expect(texts).not.toContain('125,000');
    expect(segments).toHaveLength(9);
    expect(segments.filter(([x1,,x2])=>Math.abs(x2-x1)===4)).toHaveLength(7);
    expect(segments.filter(([x1,,x2])=>x1===x2)).toHaveLength(1);
  });
});

it('uses manual pie bounds, gradient stops and scaled shadows without adding outlines',()=>{
 const arcs:number[][]=[],stops:unknown[][]=[],fills:unknown[][]=[];let strokes=0;
 const gradient={addColorStop:(...args:unknown[])=>stops.push(args)};
 const target={fillStyle:undefined as unknown,shadowBlur:0,shadowOffsetY:0,arc:(...args:number[])=>arcs.push(args),createLinearGradient:()=>gradient,getTransform:()=>({a:2,b:0}),fill(){fills.push([this.fillStyle,this.shadowBlur,this.shadowOffsetY])},stroke:()=>strokes++,measureText:()=>({width:10})};
 const ctx=new Proxy(target,{get(t,k){return k in t?t[k as keyof typeof t]:()=>{}}}) as unknown as CanvasRenderingContext2D;
 drawChart(ctx,300,240,{type:'pie',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,plotArea:{x:.1,y:.05,w:.8,h:.9},series:[{pointStyles:{0:{lineHidden:true,gradient:{angle:90,stops:[{position:0,color:'A5C55E'},{position:1,color:'709621'}]},shadow:{color:'000000',opacity:.63,blur:6,x:0,y:2}}}}]}, {categories:['a'],series:[{name:'',values:[1],x:[]}]});
 expect(arcs[0].slice(0,3)).toEqual([150,120,108]);
 expect(stops).toEqual([[0,'#A5C55E'],[1,'#709621']]);
 expect(fills[0].slice(1)).toEqual([12,4]);expect(fills[1][0]).toBe(gradient);expect(strokes).toBe(0);
});
