// Chart drawing on a 2D canvas: column, bar, line, area, pie and
// scatter, in Excel's default look. Pure apart from the canvas, so the
// same code draws on screen and the pictures sent to the PDF renderer.

interface ChartAxis {
  lineColor: string;
  lineHidden: boolean;
  hidden: boolean;
  majorTick: string;
  gridLines: boolean;
  hideLabels: boolean;
}

export interface ChartSpec {
  plotArea?: {x:number;y:number;w:number;h:number} | null;
  valueAxis?: ChartAxis | null;
  categoryAxis?: ChartAxis | null;
  series?: {
    color?: string;
    pointStyles?: {[key:string]: {
      gradient?: {angle:number;stops:{position:number;color:string}[] | null} | null;
      shadow?: {color:string;opacity:number;blur:number;x:number;y:number} | null;
      lineHidden?:boolean;
    } | undefined} | null;
    pointColors?: { [key: string]: string | undefined } | null;
    labels?: {
      fontSize: number;
      color: string;
      points: { [key: string]: { hidden: boolean; center: boolean } | undefined } | null;
    } | null;
  }[] | null;
  background?: string;
  holeSize?: number;
  type: string; // column, bar, line, area, pie, doughnut, scatter
  title: string;
  legend: string; // right, bottom, top, none
  xTitle: string;
  yTitle: string;
  stacked: boolean;
}

export interface ChartSeriesData {
  labels?: string[];
  name: string;
  values: (number | null)[];
  x: (number | null)[];
}

export interface ChartData {
  valueScale?: {min:number;max:number;step:number;labels:string[] | null} | null;
  categories: string[];
  series: ChartSeriesData[];
}

export interface ChartTheme {
  bg: string;
  text: string;
  muted: string;
  grid: string;
  font: string;
}

export const LIGHT_THEME: ChartTheme = {
  bg: '#ffffff',
  text: '#404040',
  muted: '#595959',
  grid: '#d9d9d9',
  font: '-apple-system, BlinkMacSystemFont, "PingFang TC", sans-serif',
};

// Excel's Office theme accent colors, in series order.
export const PALETTE = ['#4472C4', '#ED7D31', '#A5A5A5', '#FFC000', '#5B9BD5', '#70AD47', '#264478', '#9E480E', '#636363', '#997300'];

export function seriesColor(i: number): string {
  return PALETTE[i % PALETTE.length];
}

function chartColor(spec:ChartSpec, i:number):string {
  const color=spec.series?.[i]?.color;
  return color ? `#${color}` : seriesColor(i);
}
function pointColor(spec:ChartSpec, i:number):string {
  const color=spec.series?.[0]?.pointColors?.[i];
  return color ? `#${color}` : seriesColor(i);
}

// "Nice" axis: rounded bounds and a step of 1, 2 or 5 × 10^n.
export function niceScale(min: number, max: number, ticks = 5): { min: number; max: number; step: number } {
  if (!isFinite(min) || !isFinite(max)) return { min: 0, max: 1, step: 0.2 };
  if (min > 0 && min < max * 0.6) min = 0; // start bars at 0, like Excel
  if (max < 0 && max > min * 0.6) max = 0;
  if (min === max) {
    if (min === 0) return { min: 0, max: 1, step: 0.2 };
    min = Math.min(0, min);
    max = Math.max(0, max);
  }
  const raw = (max - min) / ticks;
  const mag = Math.pow(10, Math.floor(Math.log10(raw)));
  const norm = raw / mag;
  const step = (norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 2.5 ? 2.5 : norm <= 5 ? 5 : 10) * mag;
  return { min: Math.floor(min / step) * step, max: Math.ceil(max / step) * step, step };
}

// Bars encode magnitude as length, so their automatic axis must include zero.
export function categoryScale(type: string, min: number, max: number) {
  return ['column', 'bar', 'area'].includes(type)
    ? niceScale(Math.min(0, min), Math.max(0, max)) : niceScale(min, max);
}

export function formatTick(v: number, step: number): string {
  const decimals = Math.max(0, -Math.floor(Math.log10(step) + 1e-9));
  const s = v.toFixed(Math.min(decimals, 10));
  const [int, frac] = s.split('.');
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return frac ? `${grouped}.${frac}` : grouped;
}

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

export function drawChart(ctx: CanvasRenderingContext2D, w: number, h: number, spec: ChartSpec, data: ChartData, theme: ChartTheme = LIGHT_THEME): void {
  ctx.save();
  const background = spec.background === 'transparent' ? 'transparent' : spec.background ? `#${spec.background}` : theme.bg;
  theme = { ...theme, bg: background };
  if (background === 'transparent') ctx.clearRect(0, 0, w, h);
  else {
    ctx.fillStyle = background;
    ctx.fillRect(0, 0, w, h);
  }
  ctx.textBaseline = 'middle';
  let box: Box = { x: 10, y: 10, w: w - 20, h: h - 20 };

  if (spec.title) {
    ctx.font = `600 14px ${theme.font}`;
    ctx.fillStyle = theme.text;
    ctx.textAlign = 'center';
    ctx.fillText(spec.title, w / 2, 20, w - 20);
    box = { ...box, y: box.y + 26, h: box.h - 26 };
  }

  const pie = spec.type === 'pie' || spec.type === 'doughnut';
  const labels = pie ? data.categories : data.series.map((s) => s.name);
  const colors = pie ? data.categories.map((_, i) => pointColor(spec, i)) : data.series.map((_, i) => chartColor(spec, i));
  box = drawLegend(ctx, box, spec.legend, labels, colors, theme);

  if (data.series.length === 0) {
    ctx.restore();
    return;
  }
  if (pie && spec.plotArea) {
    const p=spec.plotArea;
    if ([p.x,p.y,p.w,p.h].every(Number.isFinite) && p.w>0 && p.h>0) box={x:p.x*w,y:p.y*h,w:p.w*w,h:p.h*h};
  }
  if (pie) drawPie(ctx, box, spec, data, theme, spec.type === 'doughnut' ? Math.max(10, Math.min(90, spec.holeSize || 50)) / 100 : 0);
  else if (spec.type === 'scatter') drawScatter(ctx, box, spec, data, theme);
  else drawCategoryChart(ctx, box, spec, data, theme);
  ctx.restore();
}

function drawLegend(ctx: CanvasRenderingContext2D, box: Box, pos: string, labels: string[], colors: string[], theme: ChartTheme): Box {
  if (pos === 'none' || labels.length === 0) return box;
  ctx.font = `11px ${theme.font}`;
  const key = 9;
  const widths = labels.map((l) => ctx.measureText(l).width + key + 12);
  ctx.textAlign = 'left';
  if (pos === 'right' || pos === '') {
    const lw = Math.min(box.w * 0.4, Math.max(...widths) + 8);
    const lineH = 18;
    let y = box.y + box.h / 2 - (labels.length * lineH) / 2 + lineH / 2;
    const x = box.x + box.w - lw + 4;
    labels.forEach((l, i) => {
      ctx.fillStyle = colors[i];
      ctx.fillRect(x, y - key / 2, key, key);
      ctx.fillStyle = theme.text;
      ctx.fillText(l, x + key + 5, y, lw - key - 10);
      y += lineH;
    });
    return { ...box, w: box.w - lw - 6 };
  }
  // Top or bottom: one centered row (clipped when too long).
  const total = widths.reduce((a, b) => a + b, 0);
  let x = box.x + Math.max(0, (box.w - total) / 2);
  const y = pos === 'top' ? box.y + 8 : box.y + box.h - 8;
  labels.forEach((l, i) => {
    ctx.fillStyle = colors[i];
    ctx.fillRect(x, y - key / 2, key, key);
    ctx.fillStyle = theme.text;
    ctx.fillText(l, x + key + 4, y);
    x += widths[i];
  });
  return pos === 'top' ? { ...box, y: box.y + 22, h: box.h - 22 } : { ...box, h: box.h - 22 };
}

function finite(v: number | null): v is number {
  return v !== null && isFinite(v);
}

// Column, bar, line and area charts share a category axis.
function drawCategoryChart(ctx: CanvasRenderingContext2D, box: Box, spec: ChartSpec, data: ChartData, theme: ChartTheme): void {
  const horizontal = spec.type === 'bar';
  const stacked = spec.stacked && spec.type !== 'line';
  const n = Math.max(data.categories.length, ...data.series.map((s) => s.values.length));
  // Value range (stacked: per-category sums of positives and negatives).
  let lo = Infinity;
  let hi = -Infinity;
  for (let i = 0; i < n; i++) {
    let pos = 0;
    let neg = 0;
    for (const s of data.series) {
      const v = s.values[i];
      if (!finite(v)) continue;
      if (stacked) {
        if (v >= 0) pos += v;
        else neg += v;
      } else {
        lo = Math.min(lo, v);
        hi = Math.max(hi, v);
      }
    }
    if (stacked) {
      lo = Math.min(lo, neg);
      hi = Math.max(hi, pos);
    }
  }
  if (lo === Infinity) {
    lo = 0;
    hi = 1;
  }
  const scale = data.valueScale ?? categoryScale(spec.type, lo, hi);
  ctx.font = `11px ${theme.font}`;
  const tickLabels: string[] = [];
  for (let i=0; i<=200 && scale.min+i*scale.step<=scale.max+scale.step*1e-8; i++) {
    tickLabels.push(data.valueScale?.labels?.[i] ?? formatTick(scale.min+i*scale.step,scale.step));
  }
  const valueLabelW = spec.valueAxis?.hidden || spec.valueAxis?.hideLabels ? 0 : Math.max(...tickLabels.map((t) => ctx.measureText(t).width)) + 8;
  const catLabelW = Math.max(0, ...data.categories.map((c) => ctx.measureText(c).width)) + 8;

  // Plot area inside the axes and axis titles.
  const yTitle = spec.yTitle ? 18 : 0;
  const xTitle = spec.xTitle ? 18 : 0;
  const plot: Box = horizontal
    ? { x: box.x + Math.min(catLabelW, box.w * 0.35) + yTitle, y: box.y + 4, w: 0, h: 0 }
    : { x: box.x + valueLabelW + yTitle, y: box.y + 6, w: 0, h: 0 };
  plot.w = box.x + box.w - plot.x - 8;
  plot.h = box.y + box.h - plot.y - 20 - xTitle;
  if (plot.w < 20 || plot.h < 20) return;

  const toPx = (v: number) =>
    horizontal ? plot.x + ((v - scale.min) / (scale.max - scale.min)) * plot.w : plot.y + plot.h - ((v - scale.min) / (scale.max - scale.min)) * plot.h;

  // Gridlines, axis lines and tick marks are independent Excel settings.
  const axisLine = (axis:ChartAxis | null | undefined, x1:number,y1:number,x2:number,y2:number) => {
    if (axis?.hidden || axis?.lineHidden) return;
    ctx.strokeStyle = axis?.lineColor ? `#${axis.lineColor}` : theme.muted;
    ctx.lineWidth=1;
    ctx.beginPath();ctx.moveTo(x1,y1);ctx.lineTo(x2,y2);ctx.stroke();
  };
  const valueAxis=spec.valueAxis;
  const categoryAxis=spec.categoryAxis;
  const zero=toPx(Math.max(scale.min,Math.min(0,scale.max)));
  axisLine(valueAxis,plot.x,horizontal ? plot.y+plot.h : plot.y,horizontal ? plot.x+plot.w : plot.x,plot.y+plot.h);
  axisLine(categoryAxis,horizontal ? zero : plot.x,horizontal ? plot.y : zero,horizontal ? zero : plot.x+plot.w,horizontal ? plot.y+plot.h : zero);
  ctx.fillStyle = theme.muted;
  tickLabels.forEach((label, i) => {
    const p = toPx(scale.min+i*scale.step);
    if (!valueAxis?.hidden && (valueAxis?.gridLines ?? true)) {
      ctx.strokeStyle=theme.grid;ctx.lineWidth=1;
      ctx.beginPath();
      ctx.moveTo(horizontal ? p : plot.x,horizontal ? plot.y : p);
      ctx.lineTo(horizontal ? p : plot.x+plot.w,horizontal ? plot.y+plot.h : p);
      ctx.stroke();
    }
    if (!valueAxis?.hidden && !valueAxis?.hideLabels) {
      ctx.textAlign=horizontal ? 'center' : 'right';
      ctx.fillText(label,horizontal ? p : plot.x-8,horizontal ? plot.y+plot.h+10 : p);
    }
    if (valueAxis?.majorTick && valueAxis.majorTick!=='none') {
      const outside=valueAxis.majorTick==='out' || valueAxis.majorTick==='cross' ? 4 : 0;
      const inside=valueAxis.majorTick==='in' || valueAxis.majorTick==='cross' ? 4 : 0;
      axisLine(valueAxis,horizontal ? p : plot.x-outside,horizontal ? plot.y+plot.h+outside : p,horizontal ? p : plot.x+inside,horizontal ? plot.y+plot.h-inside : p);
    }
  });

  // Category labels (skip some when crowded).
  const band = (horizontal ? plot.h : plot.w) / Math.max(n, 1);
  const every = Math.max(1, Math.ceil((horizontal ? 14 : catLabelW * 0.8) / band));
  ctx.fillStyle = theme.muted;
  for (let i = 0; i < n; i += every) {
    const p = (horizontal ? plot.y : plot.x) + band * (i + .5);
    if (categoryAxis?.majorTick && categoryAxis.majorTick!=='none') {
      const outside=categoryAxis.majorTick==='out' || categoryAxis.majorTick==='cross' ? 4 : 0;
      const inside=categoryAxis.majorTick==='in' || categoryAxis.majorTick==='cross' ? 4 : 0;
      axisLine(categoryAxis,horizontal ? zero-outside : p,horizontal ? p : zero-inside,horizontal ? zero+inside : p,horizontal ? p : zero+outside);
    }
    if(categoryAxis?.hidden || categoryAxis?.hideLabels) continue;
    const label = data.categories[i] ?? String(i + 1);
    if (horizontal) {
      ctx.textAlign = 'right';
      ctx.fillText(label, plot.x - 6, plot.y + band * (i + 0.5), plot.x - box.x - 8);
    } else {
      ctx.textAlign = 'center';
      ctx.fillText(label, plot.x + band * (i + 0.5), plot.y + plot.h + 10, band * every - 2);
    }
  }
  // Axis titles.
  ctx.fillStyle = theme.text;
  if (spec.xTitle) {
    ctx.textAlign = 'center';
    ctx.fillText(spec.xTitle, plot.x + plot.w / 2, box.y + box.h - 6);
  }
  if (spec.yTitle) {
    ctx.save();
    ctx.translate(box.x + 8, plot.y + plot.h / 2);
    ctx.rotate(-Math.PI / 2);
    ctx.textAlign = 'center';
    ctx.fillText(spec.yTitle, 0, 0);
    ctx.restore();
  }

  const k = data.series.length;
  if (spec.type === 'column' || spec.type === 'bar') {
    const gap = band * 0.25;
    const inner = band - 2 * gap;
    const barW = stacked ? inner : inner / k;
    const pos = new Array<number>(n).fill(0);
    const neg = new Array<number>(n).fill(0);
    data.series.forEach((s, si) => {
      ctx.fillStyle = chartColor(spec, si);
      for (let i = 0; i < n; i++) {
        const v = s.values[i];
        if (!finite(v)) continue;
        let a = 0;
        let b = v;
        if (stacked) {
          a = v >= 0 ? pos[i] : neg[i];
          b = a + v;
          if (v >= 0) pos[i] = b;
          else neg[i] = b;
        }
        const p0 = stacked ? toPx(a) : zero;
        const p1 = toPx(b);
        const offset = band * i + gap + (stacked ? 0 : barW * si);
        if (horizontal) ctx.fillRect(Math.min(p0, p1), plot.y + offset, Math.abs(p1 - p0), barW - 1);
        else ctx.fillRect(plot.x + offset, Math.min(p0, p1), barW - 1, Math.abs(p1 - p0));
      }
    });
    return;
  }
  // Line and area: points at category centers.
  const stack = new Array<number>(n).fill(0);
  const series = spec.type === 'area' && stacked ? data.series : data.series;
  series.forEach((s, si) => {
    const color = chartColor(spec, si);
    const pts: ([number, number] | null)[] = [];
    for (let i = 0; i < n; i++) {
      const v = s.values[i];
      if (!finite(v)) {
        pts.push(null);
        continue;
      }
      const base = stacked ? stack[i] : 0;
      if (stacked) stack[i] += v;
      pts.push([plot.x + band * (i + 0.5), toPx(base + v)]);
    }
    if (spec.type === 'area') {
      ctx.globalAlpha = stacked ? 0.9 : 0.6;
      ctx.fillStyle = color;
      ctx.beginPath();
      let started = false;
      let firstX = 0;
      let lastX = 0;
      for (const p of pts) {
        if (!p) continue;
        if (!started) {
          ctx.moveTo(p[0], zero);
          firstX = p[0];
          started = true;
        }
        ctx.lineTo(p[0], p[1]);
        lastX = p[0];
      }
      if (started) {
        ctx.lineTo(lastX, zero);
        ctx.lineTo(firstX, zero);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      return;
    }
    ctx.strokeStyle = color;
    ctx.lineWidth = 2.25;
    ctx.lineJoin = 'round';
    ctx.beginPath();
    let drawing = false;
    for (const p of pts) {
      if (!p) {
        drawing = false; // gaps for blanks
        continue;
      }
      if (drawing) ctx.lineTo(p[0], p[1]);
      else ctx.moveTo(p[0], p[1]);
      drawing = true;
    }
    ctx.stroke();
    ctx.fillStyle = color;
    for (const p of pts) {
      if (p) {
        ctx.beginPath();
        ctx.arc(p[0], p[1], 3, 0, Math.PI * 2);
        ctx.fill();
      }
    }
  });
}

function drawPie(ctx: CanvasRenderingContext2D, box: Box, spec:ChartSpec, data: ChartData, theme: ChartTheme, hole = 0): void {
  const values = data.series[0].values.map((v) => (finite(v) && v > 0 ? v : 0));
  const total = values.reduce((a, b) => a + b, 0);
  if (total <= 0) return;
  const r = Math.max(10, Math.min(box.w, box.h) / 2 - (spec.plotArea ? 0 : 8));
  const cx = box.x + box.w / 2;
  const cy = box.y + box.h / 2;
  let angle = -Math.PI / 2;
  ctx.font = `11px ${theme.font}`;
  // Draw all shadows before the slices so neighboring wedges cannot cast seams over each other.
  for(let i=0;i<values.length;i++) {
    const v=values[i],style=spec.series?.[0]?.pointStyles?.[i];
    const sweep=(v/total)*Math.PI*2;
    if(v>0 && style?.shadow) {
      ctx.save();
    if(style?.shadow) {
      const shadow=style.shadow,transform=ctx.getTransform?.();
      const scale=transform ? Math.hypot(transform.a,transform.b) : 1;
      ctx.shadowColor=`#${shadow.color}${Math.round(Math.max(0,Math.min(1,shadow.opacity))*255).toString(16).padStart(2,'0')}`;
      ctx.shadowBlur=shadow.blur*scale;ctx.shadowOffsetX=shadow.x*scale;ctx.shadowOffsetY=shadow.y*scale;
    }
      ctx.fillStyle=pointColor(spec,i);
      ctx.beginPath();if(!hole) ctx.moveTo(cx,cy);
      ctx.arc(cx,cy,r,angle,angle+sweep);
      if(hole) ctx.arc(cx,cy,r*hole,angle+sweep,angle,true);
      ctx.closePath();ctx.fill();ctx.restore();
    }
    angle+=sweep;
  }
  angle=-Math.PI/2;
  values.forEach((v, i) => {
    if (v <= 0) return;
    const sweep = (v / total) * Math.PI * 2;
    ctx.save();
    const style=spec.series?.[0]?.pointStyles?.[i];
    ctx.fillStyle = pointColor(spec, i);
    if (style?.gradient?.stops?.length) {
      const angle=style.gradient.angle*Math.PI/180;
      const dx=Math.cos(angle)*r,dy=Math.sin(angle)*r;
      const gradient=ctx.createLinearGradient(cx-dx,cy-dy,cx+dx,cy+dy);
      for(const stop of style.gradient.stops) gradient.addColorStop(Math.max(0,Math.min(1,stop.position)),`#${stop.color}`);
      ctx.fillStyle=gradient;
    }
    ctx.beginPath();
    if (!hole) ctx.moveTo(cx, cy);
    ctx.arc(cx, cy, r, angle, angle + sweep);
    if (hole) ctx.arc(cx, cy, r * hole, angle + sweep, angle, true);
    ctx.closePath();
    ctx.fill();
    ctx.shadowColor='transparent';
    if(!style?.lineHidden) {
      ctx.strokeStyle = theme.bg;
      ctx.lineWidth = 1.5;
      ctx.stroke();
    }
    ctx.restore();
    const labels = spec.series?.[0]?.labels;
    const point = labels?.points?.[i];
    const text = labels ? data.series[0].labels?.[i] ?? '' : `${Math.round((v / total) * 100)}%`;
    if (!point?.hidden && text && (labels || sweep > 0.25)) {
      const mid = angle + sweep / 2;
      ctx.fillStyle = labels?.color ? `#${labels.color}` : '#ffffff';
      ctx.font = `${labels?.fontSize ? labels.fontSize * 96 / 72 : 11}px ${theme.font}`;
      ctx.textAlign = 'center';
      const radius = point?.center && hole ? 0 : r * (hole ? (1 + hole) / 2 : 0.62);
      ctx.fillText(text, cx + Math.cos(mid) * radius, cy + Math.sin(mid) * radius);
    }
    angle += sweep;
  });
}

function drawScatter(ctx: CanvasRenderingContext2D, box: Box, spec: ChartSpec, data: ChartData, theme: ChartTheme): void {
  const xs: number[] = [];
  const ys: number[] = [];
  const pointsOf = (s: ChartSeriesData) =>
    s.values.map((y, i) => {
      const x = s.x.length > 0 ? s.x[i] : i + 1;
      return finite(x) && finite(y) ? ([x, y] as [number, number]) : null;
    });
  for (const s of data.series) {
    for (const p of pointsOf(s)) {
      if (p) {
        xs.push(p[0]);
        ys.push(p[1]);
      }
    }
  }
  if (xs.length === 0) return;
  const sx = niceScale(Math.min(...xs), Math.max(...xs));
  const sy = niceScale(Math.min(...ys), Math.max(...ys));
  ctx.font = `11px ${theme.font}`;
  const yLabels: string[] = [];
  for (let v = sy.min; v <= sy.max + sy.step / 2; v += sy.step) yLabels.push(formatTick(v, sy.step));
  const labelW = Math.max(...yLabels.map((t) => ctx.measureText(t).width)) + 8;
  const plot: Box = { x: box.x + labelW + (spec.yTitle ? 18 : 0), y: box.y + 6, w: 0, h: 0 };
  plot.w = box.x + box.w - plot.x - 8;
  plot.h = box.y + box.h - plot.y - 20 - (spec.xTitle ? 18 : 0);
  if (plot.w < 20 || plot.h < 20) return;
  const px = (v: number) => plot.x + ((v - sx.min) / (sx.max - sx.min)) * plot.w;
  const py = (v: number) => plot.y + plot.h - ((v - sy.min) / (sy.max - sy.min)) * plot.h;
  ctx.strokeStyle = theme.grid;
  ctx.fillStyle = theme.muted;
  ctx.lineWidth = 1;
  yLabels.forEach((label, i) => {
    const y = Math.round(py(sy.min + i * sy.step)) + 0.5;
    ctx.beginPath();
    ctx.moveTo(plot.x, y);
    ctx.lineTo(plot.x + plot.w, y);
    ctx.stroke();
    ctx.textAlign = 'right';
    ctx.fillText(label, plot.x - 6, y);
  });
  for (let v = sx.min; v <= sx.max + sx.step / 2; v += sx.step) {
    const x = Math.round(px(v)) + 0.5;
    ctx.beginPath();
    ctx.moveTo(x, plot.y);
    ctx.lineTo(x, plot.y + plot.h);
    ctx.stroke();
    ctx.textAlign = 'center';
    ctx.fillText(formatTick(v, sx.step), x, plot.y + plot.h + 10);
  }
  ctx.fillStyle = theme.text;
  if (spec.xTitle) {
    ctx.textAlign = 'center';
    ctx.fillText(spec.xTitle, plot.x + plot.w / 2, box.y + box.h - 6);
  }
  if (spec.yTitle) {
    ctx.save();
    ctx.translate(box.x + 8, plot.y + plot.h / 2);
    ctx.rotate(-Math.PI / 2);
    ctx.textAlign = 'center';
    ctx.fillText(spec.yTitle, 0, 0);
    ctx.restore();
  }
  data.series.forEach((s, si) => {
    ctx.fillStyle = chartColor(spec, si);
    for (const p of pointsOf(s)) {
      if (!p) continue;
      ctx.beginPath();
      ctx.arc(px(p[0]), py(p[1]), 3.5, 0, Math.PI * 2);
      ctx.fill();
    }
  });
}
