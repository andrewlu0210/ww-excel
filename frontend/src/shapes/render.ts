import type { ShapeDTO } from '../../bindings/ww-excel/internal/service/models';
import { TextCache } from '../grid/text';
import { drawRichLines, layoutRichText, styledRuns } from '../grid/richtext';

const LINE_DASHES: Record<string, number[]> = {
  dot:[1,3], sysDot:[1,1], dash:[4,3], sysDash:[3,1], lgDash:[8,3],
  dashDot:[4,3,1,3], sysDashDot:[3,1,1,1], lgDashDot:[8,3,1,3],
  lgDashDotDot:[8,3,1,3,1,3], sysDashDotDot:[3,1,1,1,1,1],
};

const pin = (lo: number, v: number, hi: number) => Math.max(lo, Math.min(v, hi));
const rad = (deg: number) => (deg * Math.PI) / 180;

// An end of an open path: where it is and which way the path runs there.
interface PathEnd { x: number; y: number; dx: number; dy: number }
interface Geometry {
  // Text rectangle inside the shape (callouts keep text in the box part).
  text: { x: number; y: number; w: number; h: number };
  // Open paths may carry arrowheads; start/end describe them.
  start?: PathEnd;
  end?: PathEnd;
  open?: boolean;
}

// Appends an elliptical arc that begins at the current point (DrawingML arcTo).
function arcFrom(ctx: CanvasRenderingContext2D, x: number, y: number, rx: number, ry: number, startDeg: number, sweepDeg: number): [number, number] {
  const st = rad(startDeg);
  const en = rad(startDeg + sweepDeg);
  const cx = x - rx * Math.cos(st);
  const cy = y - ry * Math.sin(st);
  ctx.ellipse(cx, cy, Math.max(rx, 0.001), Math.max(ry, 0.001), 0, st, en, sweepDeg < 0);
  return [cx + rx * Math.cos(en), cy + ry * Math.sin(en)];
}

// Builds the outline of a callout whose arrow points down; the other three
// directions are produced by mirroring/swapping the same geometry.
function calloutPath(ctx: CanvasRenderingContext2D, kind: string, w: number, h: number, adj: (name: string, def: number) => number): Geometry {
  const vertical = kind === 'downArrowCallout' || kind === 'upArrowCallout';
  // Work in a frame where the arrow always points "down" (length along v).
  const across = vertical ? w : h;
  const along = vertical ? h : w;
  const ss = Math.min(w, h);
  const a2 = pin(0, adj('adj2', 25000), (50000 * across) / ss);
  const a1 = pin(0, adj('adj1', 25000), a2 * 2);
  const a3 = pin(0, adj('adj3', 25000), (100000 * along) / ss);
  const q2 = (a3 * ss) / along;
  const a4 = pin(0, adj('adj4', 64977), 100000 - q2);
  const mid = across / 2;
  const dx1 = (ss * a2) / 100000;
  const dx2 = (ss * a1) / 200000;
  const dy3 = (ss * a3) / 100000;
  const y3 = along - dy3;
  const y2 = (along * a4) / 100000;
  // [u, v] pairs: u across the arrow, v along it.
  const pts: [number, number][] = [
    [0, 0], [across, 0], [across, y2], [mid + dx2, y2], [mid + dx2, y3], [mid + dx1, y3],
    [mid, along], [mid - dx1, y3], [mid - dx2, y3], [mid - dx2, y2], [0, y2],
  ];
  const map = ([u, v]: [number, number]): [number, number] => {
    switch (kind) {
      case 'downArrowCallout': return [u, v];
      case 'upArrowCallout': return [u, along - v];
      case 'rightArrowCallout': return [v, u];
      default: return [along - v, u]; // leftArrowCallout
    }
  };
  pts.forEach((p, i) => { const [x, y] = map(p); if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y); });
  ctx.closePath();
  const [x0, y0] = map([0, 0]);
  const [x1, y1] = map([across, y2]);
  return { text: { x: Math.min(x0, x1), y: Math.min(y0, y1), w: Math.abs(x1 - x0), h: Math.abs(y1 - y0) } };
}

function bracePath(ctx: CanvasRenderingContext2D, mirror: boolean, w: number, h: number, adj: (name: string, def: number) => number): Geometry {
  const ss = Math.min(w, h);
  const a2 = pin(0, adj('adj2', 50000), 100000);
  const q3 = Math.min(100000 - a2, a2) / 2;
  const a1 = pin(0, adj('adj1', 8333), (q3 * h) / ss);
  const y1 = (ss * a1) / 100000;
  const y3 = (h * a2) / 100000;
  const y4 = y3 + y1;
  const wd2 = w / 2;
  ctx.save();
  if (mirror) { ctx.translate(w, 0); ctx.scale(-1, 1); }
  ctx.moveTo(w, h);
  let [x, y] = arcFrom(ctx, w, h, wd2, y1, 90, 90);
  ctx.lineTo(wd2, y4);
  [x, y] = arcFrom(ctx, wd2, y4, wd2, y1, 0, -90);
  [x, y] = arcFrom(ctx, x, y, wd2, y1, 90, -90);
  ctx.lineTo(wd2, y1);
  arcFrom(ctx, wd2, y1, wd2, y1, 180, 90);
  ctx.restore();
  return { text: { x: 0, y: 0, w, h }, open: true };
}

// Builds the path of a shape's geometry in a w x h box.
function geometryPath(ctx: CanvasRenderingContext2D, shape: ShapeDTO, w: number, h: number): Geometry {
  const adjust = shape.adjust ?? {};
  const adj = (name: string, def: number) => adjust[name] ?? def;
  const whole = { x: 0, y: 0, w, h };
  switch (shape.geometry) {
    case 'ellipse':
      ctx.ellipse(w / 2, h / 2, w / 2, h / 2, 0, 0, Math.PI * 2);
      return { text: whole };
    case 'line':
    case 'straightConnector1':
      ctx.moveTo(0, 0);
      ctx.lineTo(w, h);
      return { text: whole, open: true, start: { x: 0, y: 0, dx: w, dy: h }, end: { x: w, y: h, dx: w, dy: h } };
    case 'arc': {
      const st = (adj('adj1', 16200000) / 60000) % 360;
      let en = (adj('adj2', 0) / 60000) % 360;
      if (en <= st) en += 360;
      const rx = w / 2, ry = h / 2;
      ctx.ellipse(rx, ry, Math.max(rx, 0.001), Math.max(ry, 0.001), 0, rad(st), rad(en));
      const tangent = (deg: number): [number, number] => [-rx * Math.sin(rad(deg)), ry * Math.cos(rad(deg))];
      const [sdx, sdy] = tangent(st);
      const [edx, edy] = tangent(en);
      return {
        text: whole, open: true,
        start: { x: rx + rx * Math.cos(rad(st)), y: ry + ry * Math.sin(rad(st)), dx: sdx, dy: sdy },
        end: { x: rx + rx * Math.cos(rad(en)), y: ry + ry * Math.sin(rad(en)), dx: edx, dy: edy },
      };
    }
    case 'leftBrace':
      return bracePath(ctx, false, w, h, adj);
    case 'rightBrace':
      return bracePath(ctx, true, w, h, adj);
    case 'downArrowCallout':
    case 'upArrowCallout':
    case 'rightArrowCallout':
    case 'leftArrowCallout':
      return calloutPath(ctx, shape.geometry, w, h, adj);
    case 'custom': {
      const pts: [number, number][] = [];
      let closed = false;
      for (const cmd of shape.path ?? []) {
        const p = (cmd.pts ?? []).map((v, i) => (i % 2 === 0 ? v * w : v * h));
        for (let i = 0; i + 1 < p.length; i += 2) pts.push([p[i], p[i + 1]]);
        if (cmd.op === 'moveTo') ctx.moveTo(p[0], p[1]);
        else if (cmd.op === 'lnTo') ctx.lineTo(p[0], p[1]);
        else if (cmd.op === 'cubicBezTo') ctx.bezierCurveTo(p[0], p[1], p[2], p[3], p[4], p[5]);
        else if (cmd.op === 'quadBezTo') ctx.quadraticCurveTo(p[0], p[1], p[2], p[3]);
        else if (cmd.op === 'close') { ctx.closePath(); closed = true; }
      }
      const out: Geometry = { text: whole, open: !closed };
      // Direction at the ends: toward the first/last point that differs.
      const far = (from: [number, number], list: [number, number][]) => list.find((q) => Math.hypot(q[0] - from[0], q[1] - from[1]) > 0.5) ?? from;
      if (pts.length > 1) {
        const first = pts[0], last = pts[pts.length - 1];
        const next = far(first, pts.slice(1)), prev = far(last, [...pts].reverse().slice(1));
        out.start = { x: first[0], y: first[1], dx: next[0] - first[0], dy: next[1] - first[1] };
        out.end = { x: last[0], y: last[1], dx: last[0] - prev[0], dy: last[1] - prev[1] };
      }
      return out;
    }
    default: {
      // rect / roundRect
      const r = Math.min(shape.radius, w / 2, h / 2);
      ctx.moveTo(r, 0); ctx.arcTo(w, 0, w, h, r); ctx.arcTo(w, h, 0, h, r);
      ctx.arcTo(0, h, 0, 0, r); ctx.arcTo(0, 0, w, 0, r); ctx.closePath();
      return { text: whole };
    }
  }
}

// Draws a triangular arrowhead whose tip is at the end, pointing along (dx, dy).
function arrowHead(ctx: CanvasRenderingContext2D, end: PathEnd, forward: boolean, lineWidth: number, color: string): void {
  const len = Math.hypot(end.dx, end.dy);
  if (len === 0) return;
  const sign = forward ? 1 : -1;
  const ux = (end.dx / len) * sign, uy = (end.dy / len) * sign;
  const size = Math.max(lineWidth * 3, 6);
  const bx = end.x - ux * size, by = end.y - uy * size;
  ctx.beginPath();
  ctx.moveTo(end.x, end.y);
  ctx.lineTo(bx - uy * size * 0.5, by + ux * size * 0.5);
  ctx.lineTo(bx + uy * size * 0.5, by - ux * size * 0.5);
  ctx.closePath();
  ctx.fillStyle = color;
  ctx.fill();
}

// Draws a shape into a width x height canvas box. Shapes that carry their own
// unrotated size (shape.w > 0) are rotated about the box center.
export function drawShape(ctx: CanvasRenderingContext2D, width: number, height: number, shape: ShapeDTO): void {
  const framed = shape.w > 0;
  const sw = framed ? shape.w : width;
  const sh = framed ? shape.h : height;
  ctx.save();
  ctx.beginPath();
  ctx.rect(0, 0, width, height);
  ctx.clip();
  ctx.translate(width / 2, height / 2);
  if (shape.rot) ctx.rotate(rad(shape.rot));
  ctx.translate(-sw / 2, -sh / 2);

  // Geometry is flipped; text is not.
  ctx.save();
  if (shape.flipH) { ctx.translate(sw, 0); ctx.scale(-1, 1); }
  if (shape.flipV) { ctx.translate(0, sh); ctx.scale(1, -1); }
  ctx.beginPath();
  const geometry = geometryPath(ctx, shape, sw, sh);
  if (shape.fill && !geometry.open) { ctx.fillStyle = `#${shape.fill}`; ctx.fill(); }
  if (shape.lineColor && shape.lineWidth > 0) {
    ctx.strokeStyle = `#${shape.lineColor}`;
    ctx.lineWidth = shape.lineWidth;
    ctx.setLineDash((LINE_DASHES[shape.lineDash ?? ''] ?? []).map(v => v * shape.lineWidth));
    ctx.lineJoin = 'round';
    ctx.stroke();
    if (shape.headArrow && geometry.start) arrowHead(ctx, geometry.start, false, shape.lineWidth, `#${shape.lineColor}`);
    if (shape.tailArrow && geometry.end) arrowHead(ctx, geometry.end, true, shape.lineWidth, `#${shape.lineColor}`);
  }
  ctx.restore();

  const runs = styledRuns(shape.runs ?? [], '#000000');
  if (runs.length > 0) {
    const box = geometry.text;
    const cache = new TextCache(ctx);
    const measure = (font: string, text: string) => cache.width(font, text);
    const lines = layoutRichText(runs, Math.max(1, box.w - shape.left - shape.right), true, measure, 0, { hangingPunctuation: true });
    drawRichLines(ctx, lines, measure, box, shape.alignH, shape.alignV,
      { left: shape.left, right: shape.right, top: shape.top, bottom: shape.bottom });
  }
  ctx.restore();
}
