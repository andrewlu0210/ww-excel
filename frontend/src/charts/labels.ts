import { fontFamilies } from '../stores/fonts';

export interface LabelLayout { x:number; y:number; w:number; h:number; xMode?:string; yMode?:string }
export interface PointLabel {
  hidden?:boolean; center?:boolean; position?:string; customText?:string|null;
  fontName?:string; fontSize?:number; color?:string; bold?:boolean|null; italic?:boolean|null;
  layout?:LabelLayout|null;
}
export interface LabelSpec {
  disabled?:boolean;
  fontSize?:number; fontName?:string; color?:string; bold?:boolean; italic?:boolean;
  rotation?:number; fill?:string; border?:string; leaderLines?:boolean; position?:string;
  showValue?:boolean; showPercent?:boolean; showCategory?:boolean; showSeries?:boolean;
  points?:{[key:string]:PointLabel|undefined}|null;
}
export interface LabelRect {x:number;y:number;w:number;h:number}
export interface ChartLabelHit extends LabelRect { series:number; point:number; rotation:number }
export interface LabelAnchor {
  x:number; y:number; bar?:LabelRect; horizontal?:boolean; negative?:boolean;
  pie?:{cx:number;cy:number;radius:number;angle:number};
}
const overlaps=(a:LabelRect,b:LabelRect)=>a.x<b.x+b.w+2&&a.x+a.w+2>b.x&&a.y<b.y+b.h+2&&a.y+a.h+2>b.y;
const clamp=(n:number,lo:number,hi:number)=>Math.max(lo,Math.min(hi,n));

// The same logical coordinates are used by the WebView, zoomed canvas and PDF.
export function labelPosition(anchor:LabelAnchor,position:string,width:number,height:number):LabelRect {
  let x=anchor.x-width/2,y=anchor.y-height-5;
  const bar=anchor.bar;
  if(bar) {
    const positive=!anchor.negative;
    if(anchor.horizontal) {
      y=bar.y+(bar.h-height)/2;
      if(position==='ctr') x=bar.x+(bar.w-width)/2;
      else if(position==='inEnd') x=positive?bar.x+bar.w-width-4:bar.x+4;
      else if(position==='inBase') x=positive?bar.x+4:bar.x+bar.w-width-4;
      else x=positive?bar.x+bar.w+5:bar.x-width-5;
    } else {
      x=bar.x+(bar.w-width)/2;
      if(position==='ctr') y=bar.y+(bar.h-height)/2;
      else if(position==='inEnd') y=positive?bar.y+4:bar.y+bar.h-height-4;
      else if(position==='inBase') y=positive?bar.y+bar.h-height-4:bar.y+4;
      else y=positive?bar.y-height-5:bar.y+bar.h+5;
    }
  } else if(anchor.pie) {
    if(position==='outEnd'||position==='bestFit') {
      const p=anchor.pie,cos=Math.cos(p.angle),sin=Math.sin(p.angle);
      x=p.cx+cos*(p.radius+9)+(cos>=0?0:-width);
      y=p.cy+sin*(p.radius+9)-height/2;
    } else { x=anchor.x-width/2;y=anchor.y-height/2; }
  }
  if(position==='t'){x=anchor.x-width/2;y=anchor.y-height-5;}
  if(position==='b'){x=anchor.x-width/2;y=anchor.y+5;}
  if(position==='l'){x=anchor.x-width-5;y=anchor.y-height/2;}
  if(position==='r'){x=anchor.x+5;y=anchor.y-height/2;}
  if(position==='ctr'&&!bar){x=anchor.x-width/2;y=anchor.y-height/2;}
  return {x,y,w:width,h:height};
}

export function drawDataLabel(
  ctx:CanvasRenderingContext2D, viewport:{w:number;h:number}, anchor:LabelAnchor,
  text:string, style:LabelSpec, point:PointLabel|undefined, theme:{font:string;text:string;muted:string},
  occupied:LabelRect[],
):LabelRect|null {
  if(style.disabled||point?.hidden)return null;
  text=point?.customText ?? text;if(!text)return null;
  ctx.save();
  const size=(point?.fontSize || style.fontSize || 11)*96/72;
  const name=point?.fontName || style.fontName;
  const font=name?fontFamilies(name).map(f=>`"${f.replace(/"/g,'')}"`).join(', ')+', sans-serif':theme.font;
  ctx.font=`${(point?.italic ?? style.italic)?'italic ':''}${(point?.bold ?? style.bold)?'700 ':''}${size}px ${font}`;
  const layout=point?.layout;
  const maxWidth=Math.max(8,Math.min(viewport.w-8,layout?.w ? layout.w*viewport.w-8 : viewport.w*0.6));
  const lines:string[]=[];
  for(const paragraph of text.slice(0,8192).split('\n')) {
    let line='';for(const ch of paragraph){if(line&&ctx.measureText(line+ch).width>maxWidth){lines.push(line);line='';}line+=ch;}lines.push(line);
  }
  if(lines.length>50)lines.splice(50);
  const lineHeight=size*1.2;
  let width=Math.max(...lines.map(l=>ctx.measureText(l).width),1)+6,height=lines.length*lineHeight+4;
  if(layout?.w)width=Math.max(width,layout.w*viewport.w);
  if(layout?.h)height=Math.max(height,layout.h*viewport.h);
  const position=point?.position || style.position || (anchor.pie?'ctr':anchor.bar?'outEnd':'t');
  let rect=labelPosition(anchor,position,width,height);
  if(layout) {
    rect.x=layout.xMode==='edge'?layout.x*viewport.w:rect.x+layout.x*viewport.w;
    rect.y=layout.yMode==='edge'?layout.y*viewport.h:rect.y+layout.y*viewport.h;
  }
  const angle=(style.rotation || 0)*Math.PI/180;
  const extentW=Math.abs(width*Math.cos(angle))+Math.abs(height*Math.sin(angle));
  const extentH=Math.abs(height*Math.cos(angle))+Math.abs(width*Math.sin(angle));
  const centerX=clamp(rect.x+width/2,Math.min(viewport.w/2,extentW/2+3),Math.max(viewport.w/2,viewport.w-extentW/2-3));
  const centerY=clamp(rect.y+height/2,Math.min(viewport.h/2,extentH/2+3),Math.max(viewport.h/2,viewport.h-extentH/2-3));
  rect={...rect,x:centerX-width/2,y:centerY-height/2};
  if(!layout&&!point?.center) {
    const initial=rect.y;
    for(let step=0;step<12&&occupied.some(other=>overlaps(rect,other));step++) {
      const shift=Math.ceil((step+1)/2)*lineHeight*(step%2===0?1:-1);
      rect.y=clamp(initial+shift,3,Math.max(3,viewport.h-height-3));
    }
  }
  if(style.leaderLines&&anchor.pie&&(position==='outEnd'||position==='bestFit'||layout)) {
    const p=anchor.pie;ctx.beginPath();ctx.strokeStyle=theme.muted;ctx.lineWidth=0.8;
    ctx.moveTo(p.cx+Math.cos(p.angle)*p.radius,p.cy+Math.sin(p.angle)*p.radius);
    ctx.lineTo(clamp(anchor.x,rect.x,rect.x+rect.w),clamp(anchor.y,rect.y,rect.y+rect.h));ctx.stroke();
  }
  if(style.fill){ctx.fillStyle='#'+style.fill;ctx.fillRect(rect.x,rect.y,rect.w,rect.h);}
  if(style.border){ctx.strokeStyle='#'+style.border;ctx.lineWidth=1;ctx.strokeRect(rect.x,rect.y,rect.w,rect.h);}
  ctx.fillStyle='#'+(point?.color || style.color || (anchor.pie&&position!=='outEnd'&&position!=='bestFit'?'ffffff':theme.text.replace('#','')));
  ctx.textAlign='center';ctx.textBaseline='middle';
  let x=rect.x+rect.w/2,y=rect.y+rect.h/2-(lines.length-1)*lineHeight/2;
  if(angle){ctx.translate(rect.x+rect.w/2,rect.y+rect.h/2);ctx.rotate(angle);x=0;y=-(lines.length-1)*lineHeight/2;}
  lines.forEach((line,i)=>ctx.fillText(line,x,y+i*lineHeight));
  occupied.push(rect);ctx.restore();return rect;
}
