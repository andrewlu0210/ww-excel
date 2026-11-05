import {describe,it,expect} from 'vitest';
import {draggedLabelLayout,chartWithLabelLayout} from './labelDrag';
import type {ChartDTO} from '../bridge/sheet';

const hit={x:100,y:50,w:80,h:25,rotation:0,series:1,point:2};
describe('chart label drag geometry',()=>{
  it('saves positions relative to the entire chart and preserves manual dimensions',()=>{
    expect(draggedLabelLayout(hit,60,30,400,300,{x:0,y:0,w:.3,h:.2})).toEqual({x:.4,y:80/300,w:.3,h:.2,xMode:'edge',yMode:'edge'});
    const doubled={...hit,x:200,y:100,w:160,h:50};
    expect(draggedLabelLayout(doubled,120,60,800,600).x).toBeCloseTo(.4);
  });
  it('keeps both ordinary and rotated labels inside the chart at every edge',()=>{
    for(const rotation of [0,45,90,-30])for(const dx of [-10000,10000])for(const dy of [-10000,10000]) {
      const layout=draggedLabelLayout({...hit,rotation},dx,dy,400,300);
      const angle=rotation*Math.PI/180;
      const ew=Math.abs(hit.w*Math.cos(angle))+Math.abs(hit.h*Math.sin(angle));
      const eh=Math.abs(hit.h*Math.cos(angle))+Math.abs(hit.w*Math.sin(angle));
      const cx=layout.x*400+hit.w/2,cy=layout.y*300+hit.h/2;
      expect(cx-ew/2).toBeGreaterThanOrEqual(2.999);expect(cx+ew/2).toBeLessThanOrEqual(397.001);
      expect(cy-eh/2).toBeGreaterThanOrEqual(2.999);expect(cy+eh/2).toBeLessThanOrEqual(297.001);
      if(dx<0)expect(cx-ew/2).toBeCloseTo(3);else expect(cx+ew/2).toBeCloseTo(397);
      if(dy<0)expect(cy-eh/2).toBeCloseTo(3);else expect(cy+eh/2).toBeCloseTo(297);
    }
  });
  it('previews only the selected point and leaves source settings untouched',()=>{
    const chart={type:'combo',title:'Sales',legend:'right',xTitle:'',yTitle:'',stacked:false,series:[{name:'A',values:'A1',categories:'',labels:{format:'0',showValue:true,showPercent:false,fontSize:12,color:'',points:{2:{hidden:false,center:true,customText:'Keep',fontSize:18}}}},{name:'B',values:'B1',categories:''}]} satisfies ChartDTO;
    const before=JSON.stringify(chart);
    const layout=draggedLabelLayout(hit,40,20,400,300);
    const preview=chartWithLabelLayout(chart,0,2,layout);
    expect(JSON.stringify(chart)).toBe(before);expect(preview.series?.[1]).toBe(chart.series[1]);
    expect(preview.series?.[0].labels?.points?.['2']).toMatchObject({layout,center:false,customText:'Keep',fontSize:18});
    expect(preview.type).toBe('combo');
  });
  it('keeps the default pie percentage labels available in the drag preview',()=>{
    const chart={type:'pie',title:'',legend:'none',xTitle:'',yTitle:'',stacked:false,series:[{name:'',values:'A1',categories:''}]} satisfies ChartDTO;
    const preview=chartWithLabelLayout(chart,0,0,draggedLabelLayout(hit,10,10,400,300));
    expect(preview.series?.[0].labels).toMatchObject({showPercent:true,fontSize:8.25,color:'ffffff'});
  });
});
