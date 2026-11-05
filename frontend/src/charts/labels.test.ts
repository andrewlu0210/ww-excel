import {describe,it,expect} from 'vitest';
import {drawDataLabel,labelPosition,type ChartLabelHit} from './labels';
import {drawChart,LIGHT_THEME} from './render';
function context() {
  const text:{text:string;x:number;y:number;font:string;color:string;clipped:boolean}[]=[];
  const bars:number[][]=[];const rotations:number[]=[];
  const stack:boolean[]=[];let clipped=false;
  const target={font:'',fillStyle:'',save(){stack.push(clipped)},restore(){clipped=stack.pop() ?? false},clip(){clipped=true},measureText:(s:string)=>({width:s.length*7}),fillText(textValue:string,x:number,y:number){text.push({text:textValue,x,y,font:this.font,color:this.fillStyle,clipped})},fillRect:(...args:number[])=>bars.push(args),rotate:(angle:number)=>rotations.push(angle)};
  const ctx=new Proxy(target,{get(t,k){return k in t?t[k as keyof typeof t]:()=>{}}}) as unknown as CanvasRenderingContext2D;
  return {ctx,text,bars,rotations};
}
describe('data label layout',()=>{
  it('places positive and negative bar labels at the correct ends',()=>{
    const vertical={x:50,y:40,bar:{x:40,y:40,w:20,h:100}};
    expect(labelPosition(vertical,'outEnd',30,15).y).toBeLessThan(40);
    expect(labelPosition({...vertical,negative:true},'outEnd',30,15).y).toBeGreaterThan(140);
    const horizontal={...vertical,horizontal:true};
    expect(labelPosition(horizontal,'outEnd',30,15).x).toBeGreaterThan(60);
    expect(labelPosition({...horizontal,negative:true},'outEnd',30,15).x+30).toBeLessThan(40);
    expect(labelPosition(vertical,'ctr',30,20)).toEqual({x:35,y:80,w:30,h:20});
  });
  it('keeps manual positions relative to chart size and honors individual fonts',()=>{
    for(const w of [300,600]) {
      const {ctx,text}=context();
      const rect=drawDataLabel(ctx,{w,h:300},{x:100,y:100},'A\nB',{fontSize:11,bold:true},{layout:{x:.2,y:.1,w:0,h:0,xMode:'edge',yMode:'edge'},fontName:'Arial',fontSize:18,bold:false,color:'FF0000'},LIGHT_THEME,[])!;
      expect(rect.x).toBeCloseTo(w*.2);expect(rect.y).toBeCloseTo(30);
      expect(text.map(t=>t.text)).toEqual(['A','B']);expect(text[0].font).toContain('24px');expect(text[0].font).not.toContain('700');expect(text[0].color).toBe('#FF0000');
    }
  });
  it('wraps long labels, keeps rotated text inside the canvas and respects hidden labels',()=>{
    const {ctx,text,rotations}=context();
    const rect=drawDataLabel(ctx,{w:300,h:200},{x:299,y:0},'一段很長的文字用來確認標籤會換行並留在圖表範圍，還有更多文字確保跨越換行寬度',{rotation:45,fontSize:12},undefined,LIGHT_THEME,[])!;
    expect(text.length).toBeGreaterThan(1);expect(rect.x).toBeGreaterThanOrEqual(0);expect(rect.y).toBeGreaterThanOrEqual(0);expect(rotations).toContain(Math.PI/4);
    expect(drawDataLabel(ctx,{w:300,h:200},{x:20,y:20},'hidden',{}, {hidden:true},LIGHT_THEME,[])).toBeNull();
    expect(text.some(t=>t.text==='hidden')).toBe(false);
  });
});
describe('labels across chart types',()=>{
  it.each(['column','bar','line','area','scatter'])('renders %s labels outside the plot clip and skips blanks',type=>{
    const {ctx,text}=context();
    drawChart(ctx,400,300,{type,title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,series:[{labels:{showValue:true,fontSize:12,color:'A00000'}}]},
      {categories:['A','B','C'],series:[{name:'Sales',values:[10,null,-5],x:[1,2,3],labels:['Ten','','Minus']}]});
    const labels=text.filter(t=>t.text==='Ten'||t.text==='Minus');
    expect(labels).toHaveLength(2);expect(labels.every(t=>!t.clipped&&t.color==='#A00000'&&Number.isFinite(t.x)&&Number.isFinite(t.y))).toBe(true);
  });
  it('places pie percentages outside slices and draws custom multi-line point text',()=>{
    const {ctx,text}=context();
    drawChart(ctx,400,300,{type:'pie',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,series:[{labels:{position:'outEnd',leaderLines:true,showPercent:true,points:{1:{customText:'Custom\n第二行'}}}}]},
      {categories:['A','B'],series:[{name:'Sales',values:[30,70],x:[],labels:['30%','70%']}]});
    expect(text.map(t=>t.text)).toContain('30%');expect(text.map(t=>t.text)).toContain('Custom');expect(text.map(t=>t.text)).toContain('第二行');expect(text.map(t=>t.text)).not.toContain('70%');
  });
});

describe('interactive data label regions',()=>{
  it.each(['column','bar','line','area','scatter','pie','doughnut','combo'])('reports exactly the rendered %s points in chart coordinates',type=>{
    const {ctx}=context();const hits:ChartLabelHit[]=[];
    drawChart(ctx,400,300,{type,title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,series:[{type:'column',labels:{showValue:true,rotation:30,points:{2:{hidden:true}}}}]},
      {categories:['A','B','C'],series:[{name:'Sales',values:[10,null,30],x:[1,2,3],labels:['Ten','','Hidden']}]},LIGHT_THEME,hits);
    expect(hits).toHaveLength(1);expect(hits[0]).toMatchObject({series:0,point:0,rotation:30});
    expect(hits[0].w).toBeGreaterThan(0);expect(hits[0].h).toBeGreaterThan(0);
    expect(Number.isFinite(hits[0].x)&&Number.isFinite(hits[0].y)).toBe(true);
  });
  it('removes stale hit regions when labels are disabled',()=>{
    const {ctx}=context();const hits:ChartLabelHit[]=[{x:0,y:0,w:10,h:10,series:0,point:0,rotation:0}];
    drawChart(ctx,400,300,{type:'column',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,series:[{labels:{disabled:true,showValue:true}}]},
      {categories:['A'],series:[{name:'Sales',values:[10],x:[],labels:['Ten']}]},LIGHT_THEME,hits);
    expect(hits).toEqual([]);
  });
});
