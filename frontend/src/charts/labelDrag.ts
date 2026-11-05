import type {ChartDTO} from '../bridge/sheet';
import type {ChartLabelHit,LabelLayout} from './labels';

// Both pointer motion and saved layouts use the chart's logical coordinates.
export function draggedLabelLayout(hit:ChartLabelHit,dx:number,dy:number,width:number,height:number,previous?:LabelLayout|null):LabelLayout {
  const angle=hit.rotation*Math.PI/180;
  const extentW=Math.abs(hit.w*Math.cos(angle))+Math.abs(hit.h*Math.sin(angle));
  const extentH=Math.abs(hit.w*Math.sin(angle))+Math.abs(hit.h*Math.cos(angle));
  const center=(value:number,extent:number,size:number)=>Math.max(Math.min(size/2,extent/2+3),Math.min(Math.max(size/2,size-extent/2-3),value));
  const x=center(hit.x+dx+hit.w/2,extentW,width)-hit.w/2;
  const y=center(hit.y+dy+hit.h/2,extentH,height)-hit.h/2;
  return {x:x/width,y:y/height,w:previous?.w ?? 0,h:previous?.h ?? 0,xMode:'edge',yMode:'edge'};
}

// The draft is only for the live preview; the backend commits a single point.
export function chartWithLabelLayout(chart:ChartDTO,series:number,point:number,layout:LabelLayout):ChartDTO {
  const key=`${point}` as `${number}`;
  return {...chart,series:(chart.series ?? []).map((s,i)=>i!==series?s:{...s,labels:{
    ...(s.labels ?? {format:'General',showValue:false,showPercent:true,fontSize:8.25,color:'ffffff'}),
    points:{...s.labels?.points,[key]:{hidden:false,...s.labels?.points?.[key],layout,center:false}},
  }})};
}
