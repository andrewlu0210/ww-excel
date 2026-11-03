export interface ConditionalIcon { set: string; index: number }
export interface DataBar { len: number; color: string; solid?: boolean; borderColor?: string }

import { iconSetCount } from './iconSets';

const RED='#e04040',YELLOW='#f1c232',GREEN='#52a34a',GRAY='#808080';
const traffic=[RED,YELLOW,GREEN];
const angles:Record<number,number[]>={3:[180,90,0],4:[180,135,45,0],5:[180,135,90,45,0]};

// Icon IDs follow OOXML order, from the lowest to the highest state.
// Pure canvas paths keep symbols independent of installed fonts and emoji.
export function drawConditionalIcon(ctx: CanvasRenderingContext2D, icon: ConditionalIcon, x: number, y: number, size: number): boolean {
  const count=iconSetCount(icon.set);
  if(!count || !Number.isInteger(icon.index) || icon.index<0 || icon.index>=count || ![x,y,size].every(Number.isFinite) || size<=0)return false;
  const index=icon.index,set=icon.set;
  ctx.save();
  // Preserve the existing traffic-light circles and completion-dot dimensions.
  if(set==='3TrafficLights1' || set==='4TrafficLights') {
    ctx.fillStyle=set==='4TrafficLights'?['#222222',...traffic][index]:traffic[index];
    ctx.beginPath();ctx.arc(x+size/2,y+size/2,size/2,0,Math.PI*2);ctx.fill();ctx.restore();return true;
  }
  ctx.translate(x,y);ctx.scale(size,size);
  ctx.lineWidth=.065;ctx.lineJoin='round';ctx.lineCap='round';
  const polygon=(points:number[][])=>{ctx.beginPath();points.forEach(([px,py],i)=>i?ctx.lineTo(px,py):ctx.moveTo(px,py));ctx.closePath();};
  const circle=(color:string,r=.44)=>{ctx.fillStyle=color;ctx.beginPath();ctx.arc(.5,.5,r,0,Math.PI*2);ctx.fill();};
  const outline=()=>{ctx.strokeStyle='#555555';ctx.stroke();};
  if(set.includes('Arrows')) {
    ctx.translate(.5,.5);ctx.rotate(angles[count][index]*Math.PI/180);ctx.translate(-.5,-.5);
    ctx.fillStyle=set.endsWith('Gray')?GRAY:index===0?RED:index===count-1?GREEN:YELLOW;
    polygon([[.5,.04],[.94,.46],[.65,.46],[.65,.95],[.35,.95],[.35,.46],[.06,.46]]);ctx.fill();outline();
  } else if(set==='3Flags') {
    ctx.strokeStyle='#555555';ctx.beginPath();ctx.moveTo(.19,.08);ctx.lineTo(.19,.94);ctx.stroke();
    ctx.fillStyle=traffic[index];polygon([[.23,.1],[.9,.1],[.75,.34],[.9,.57],[.23,.57]]);ctx.fill();outline();
  } else if(set==='3TrafficLights2') {
    ctx.fillStyle='#222222';ctx.fillRect(.04,.04,.92,.92);circle(traffic[index],.35);
  } else if(set==='4RedToBlack') {
    circle(['#222222','#a6a6a6','#f1a1a1',RED][index]);outline();
  } else if(set==='3Signs') {
    ctx.fillStyle=traffic[index];
    if(index===0)polygon([[.5,.04],[.96,.5],[.5,.96],[.04,.5]]);
    else if(index===1)polygon([[.5,.06],[.95,.91],[.05,.91]]);
    else{circle(GREEN);}
    if(index<2)ctx.fill();outline();
  } else if(set==='3Symbols' || set==='3Symbols2') {
    if(set==='3Symbols')circle(traffic[index]);
    ctx.strokeStyle=set==='3Symbols'?'#ffffff':traffic[index];ctx.lineWidth=.12;
    ctx.beginPath();
    if(index===0){ctx.moveTo(.29,.27);ctx.lineTo(.73,.73);ctx.moveTo(.73,.27);ctx.lineTo(.29,.73);}
    else if(index===1){ctx.moveTo(.5,.21);ctx.lineTo(.5,.57);ctx.moveTo(.5,.76);ctx.lineTo(.5,.77);}
    else{ctx.moveTo(.21,.51);ctx.lineTo(.43,.73);ctx.lineTo(.8,.25);}
    ctx.stroke();
  } else if(set==='4Rating' || set==='5Rating') {
    const gap=.04,width=(.92-gap*(count-1))/count;
    for(let i=0;i<count;i++){
      const height=.18+.72*i/(count-1);
      ctx.fillStyle=i<=index?'#426d91':'#c8cdd1';ctx.fillRect(.04+i*(width+gap),.94-height,width,height);
    }
  } else if(set==='5Quarters') {
    circle('#ffffff');outline();
    if(index>0){ctx.fillStyle='#222222';ctx.beginPath();ctx.moveTo(.5,.5);ctx.arc(.5,.5,.44,-Math.PI/2,-Math.PI/2+index*Math.PI/2);ctx.closePath();ctx.fill();}
  } else if(set==='5Boxes') {
    for(let i=0;i<4;i++){
      ctx.fillStyle=i<index?'#426d91':'#edf0f2';ctx.fillRect(.07+(i%2)*.47,.07+Math.floor(i/2)*.47,.39,.39);
      ctx.strokeStyle=i<index?'#355a7a':'#b9c1c8';ctx.lineWidth=.045;ctx.strokeRect(.07+(i%2)*.47,.07+Math.floor(i/2)*.47,.39,.39);
    }
  } else if(set==='3Stars') {
    const points=Array.from({length:10},(_,i)=>{const r=i%2?.19:.46,a=-Math.PI/2+i*Math.PI/5;return[.5+Math.cos(a)*r,.5+Math.sin(a)*r];});
    polygon(points);ctx.fillStyle='#e1e1e1';ctx.fill();
    ctx.save();ctx.clip();ctx.fillStyle='#f1c232';ctx.fillRect(0,0,index/2,1);ctx.restore();
    polygon(points);ctx.strokeStyle=index?'#bf941b':'#999999';ctx.stroke();
  } else if(set==='3Triangles') {
    ctx.fillStyle=traffic[index];
    if(index===0)polygon([[.06,.22],[.94,.22],[.5,.84]]);
    else if(index===1)polygon([[.08,.42],[.92,.42],[.92,.58],[.08,.58]]);
    else polygon([[.5,.16],[.94,.78],[.06,.78]]);
    ctx.fill();outline();
  }
  ctx.restore();return true;
}

export function drawDataBar(ctx: CanvasRenderingContext2D, bar: DataBar, left: number, top: number, width: number, height: number): void {
  const w = Math.max(0, width - 4) * Math.max(0, Math.min(1, bar.len));
  const h = Math.max(0, height - 5);
  if (w <= 0 || h <= 0) return;
  ctx.save();
  if (bar.solid) ctx.fillStyle = bar.color;
  else {
    const gradient = ctx.createLinearGradient(left + 2, 0, left + 2 + w, 0);
    gradient.addColorStop(0, bar.color);
    gradient.addColorStop(1, bar.color + '33');
    ctx.fillStyle = gradient;
  }
  ctx.fillRect(left + 2, top + 2, w, h);
  if (bar.borderColor) {
    ctx.strokeStyle = bar.borderColor;
    ctx.lineWidth = 1;
    ctx.strokeRect(left + 2.5, top + 2.5, Math.max(0, w - 1), Math.max(0, h - 1));
  }
  ctx.restore();
}
