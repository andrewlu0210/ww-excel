import type { ShapeDTO } from '../../bindings/ww-excel/internal/service/models';
import { TextCache } from '../grid/text';
import { drawRichLines, layoutRichText, styledRuns } from '../grid/richtext';

export function drawShape(ctx: CanvasRenderingContext2D, width: number, height: number, shape: ShapeDTO): void {
  ctx.save();
  ctx.beginPath();
  ctx.rect(0, 0, width, height);
  ctx.clip();
  ctx.beginPath();
  const r = Math.min(shape.radius, width / 2, height / 2);
  ctx.moveTo(r, 0); ctx.arcTo(width, 0, width, height, r); ctx.arcTo(width, height, 0, height, r);
  ctx.arcTo(0, height, 0, 0, r); ctx.arcTo(0, 0, width, 0, r); ctx.closePath();
  if (shape.fill) { ctx.fillStyle = `#${shape.fill}`; ctx.fill(); }
  if (shape.lineColor && shape.lineWidth > 0) { ctx.strokeStyle = `#${shape.lineColor}`; ctx.lineWidth = shape.lineWidth; ctx.stroke(); }
  const cache = new TextCache(ctx);
  const measure = (font: string, text: string) => cache.width(font, text);
  const runs = styledRuns(shape.runs ?? [], '#000000');
  const lines = layoutRichText(runs, Math.max(1, width - shape.left - shape.right), true, measure, Math.max(1, ...runs.map((r) => r.px)));
  drawRichLines(ctx, lines, measure, { x: 0, y: 0, w: width, h: height }, shape.alignH, shape.alignV,
    { left: shape.left, right: shape.right, top: shape.top, bottom: shape.bottom });
  ctx.restore();
}
