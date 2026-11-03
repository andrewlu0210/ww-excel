// IDs and ascending state counts from Microsoft's ST_IconSetType definition.
export const ICON_SETS = [
  {id:'3Arrows',count:3,label:'彩色箭頭'}, {id:'3ArrowsGray',count:3,label:'灰色箭頭'},
  {id:'3Flags',count:3,label:'旗標'}, {id:'3TrafficLights1',count:3,label:'交通燈'},
  {id:'3TrafficLights2',count:3,label:'附框交通燈'}, {id:'3Signs',count:3,label:'形狀'},
  {id:'3Symbols',count:3,label:'圓形符號'}, {id:'3Symbols2',count:3,label:'符號'},
  {id:'4Arrows',count:4,label:'彩色箭頭'}, {id:'4ArrowsGray',count:4,label:'灰色箭頭'},
  {id:'4RedToBlack',count:4,label:'紅至黑圓點'}, {id:'4Rating',count:4,label:'評等'},
  {id:'4TrafficLights',count:4,label:'交通燈'}, {id:'5Arrows',count:5,label:'彩色箭頭'},
  {id:'5ArrowsGray',count:5,label:'灰色箭頭'}, {id:'5Rating',count:5,label:'評等'},
  {id:'5Quarters',count:5,label:'四分圓'}, {id:'3Stars',count:3,label:'星形'},
  {id:'3Triangles',count:3,label:'三角形'}, {id:'5Boxes',count:5,label:'四分方塊'},
] as const;
export function iconSetCount(id:string):number { return ICON_SETS.find(s=>s.id===id)?.count ?? 0; }
export function iconThresholds(count:number) {
  return Array.from({length:count},(_,i)=>({type:'percent',value:String(Math.round(i*100/count)),strict:false}));
}
