import type { FillSpec } from '../../bindings/ww-excel/internal/model/models';
export type FillStop = { position: number; color: string };
export const FILL_PATTERNS = ['darkGray','mediumGray','lightGray','gray125','gray0625','darkHorizontal','darkVertical','darkDown','darkUp','darkGrid','darkTrellis','lightHorizontal','lightVertical','lightDown','lightUp','lightGrid','lightTrellis'];
export function fillStops(spec: FillSpec): FillStop[] {
  try { const stops = JSON.parse(spec.stops || '[]'); return Array.isArray(stops) && stops.length <= 256 ? stops : []; } catch { return []; }
}
export function patternInk(pattern: string, x: number, y: number): boolean {
  x = ((x % 8) + 8) % 8; y = ((y % 8) + 8) % 8;
  switch (pattern) {
    case 'darkGray': return !(x%2===0 && y%2===0);
    case 'mediumGray': return (x+y)%2===0;
    case 'lightGray': return x%2===0 && y%2===0;
    case 'gray125': return y%2===0 && x%4===y%4;
    case 'gray0625': return y%4===0 && x%4===0;
    case 'darkHorizontal': return y%4<2;
    case 'darkVertical': return x%4<2;
    case 'darkDown': return (x-y+8)%4<2;
    case 'darkUp': return (x+y)%4<2;
    case 'darkGrid': return x%4<2 || y%4<2;
    case 'darkTrellis': return (x+y)%4<2 || (x-y+8)%4<2;
    case 'lightHorizontal': return y===0;
    case 'lightVertical': return x===0;
    case 'lightDown': return (x-y+8)%8===0;
    case 'lightUp': return (x+y)%8===0;
    case 'lightGrid': return x===0 || y===0;
    case 'lightTrellis': return (x+y)%8===0 || (x-y+8)%8===0;
    default: return false;
  }
}
export function fillPosition(f: FillSpec, x: number, y: number, width=1, height=1): number {
  if (f.kind === 'path') {
    const axis = (p: number, a = 0, b = 0) => p < a && a > 0 ? (a-p)/a : p > b && b < 1 ? (p-b)/(1-b) : 0;
    return Math.min(1, Math.max(axis(x,f.left,f.right),axis(y,f.top,f.bottom)));
  }
  const a = (f.degree ?? 0)*Math.PI/180, c = Math.cos(a)*width, s = Math.sin(a)*height;
  return Math.max(0, Math.min(1, .5+((x-.5)*c+(y-.5)*s)/(Math.abs(c)+Math.abs(s))));
}
const rgb = (s: string) => { const n = parseInt(s || 'FFFFFF',16); return [(n>>16)&255,(n>>8)&255,n&255]; };
export function fillColor(stops: FillStop[], t: number): number[] {
  if (!stops.length) return [255,255,255];
  if (t <= stops[0].position) return rgb(stops[0].color);
  for (let i=1;i<stops.length;i++) { const a=stops[i-1],b=stops[i]; if(t>b.position) continue; const k=b.position>a.position ? (t-a.position)/(b.position-a.position) : 1; const ca=rgb(a.color),cb=rgb(b.color);return ca.map((v,j)=>Math.round(v+(cb[j]-v)*k)); }
  return rgb(stops[stops.length-1].color);
}
const images = new Map<string, HTMLCanvasElement>();
export function paintCellFill(ctx: CanvasRenderingContext2D, f: FillSpec, x: number, y: number, width: number, height: number): void {
  if (!f.kind || width<=0 || height<=0) return;
  const w=Math.max(1,Math.min(512,Math.ceil(width))),h=Math.max(1,Math.min(512,Math.ceil(height)));
  const key=JSON.stringify([f,w,h]); let canvas=images.get(key);
  if(!canvas) {
    canvas=document.createElement('canvas'); canvas.width=w;canvas.height=h;
    const c=canvas.getContext('2d')!;const pixels=c.createImageData(w,h), stops=fillStops(f),fg=rgb(f.foreground || '000000'),bg=rgb(f.background || 'FFFFFF');
    for(let row=0;row<h;row++) for(let col=0;col<w;col++) {
      const color=f.kind==='pattern' ? (patternInk(f.pattern || '',col,row)?fg:bg) : fillColor(stops,fillPosition(f,(col+.5)/w,(row+.5)/h,width,height));
      const offset=(row*w+col)*4;pixels.data.set([...color,255],offset);
    }
    c.putImageData(pixels,0,0);if(images.size>=64) images.delete(images.keys().next().value!);images.set(key,canvas);
  }
  ctx.drawImage(canvas,x,y,width,height);
}

export function validCellFill(f: FillSpec): boolean {
  const hex = (s = '') => /^[0-9a-f]{6}$/i.test(s);
  if (!f.kind) return true;
  if (f.kind==='pattern') return FILL_PATTERNS.includes(f.pattern || '') && hex(f.foreground) && hex(f.background);
  if (!['linear','path'].includes(f.kind)) return false;
  const stops=fillStops(f);
  if(stops.length<2 || !stops.every(s=>Number.isFinite(s.position)&&s.position>=0&&s.position<=1&&hex(s.color))) return false;
  if(f.kind==='linear') return Number.isFinite(f.degree ?? 0) && (f.degree ?? 0)>=0 && (f.degree ?? 0)<=360;
  const bounds=[f.left ?? 0,f.right ?? 0,f.top ?? 0,f.bottom ?? 0];
  return bounds.every(v=>Number.isFinite(v)&&v>=0&&v<=1) && bounds[0]<=bounds[1] && bounds[2]<=bounds[3];
}

export function representativeFill(f?: FillSpec): string | null {
  if(!f?.kind) return null;
  if(f.kind==='pattern') return `#${f.background || 'FFFFFF'}`;
  return '#'+fillColor(fillStops(f),.5).map(n=>n.toString(16).padStart(2,'0')).join('');
}
