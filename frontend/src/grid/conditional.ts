export interface ConditionalIcon { set: string; index: number }
export interface DataBar { len: number; color: string; solid?: boolean; borderColor?: string }

// Icon IDs follow OOXML order, from the lowest to the highest state.
export function drawConditionalIcon(ctx: CanvasRenderingContext2D, icon: ConditionalIcon, x: number, y: number, size: number): boolean {
  const colors = icon.set === '4TrafficLights' ? ['#222222', '#e04040', '#f1c232', '#52a34a']
    : ['3TrafficLights1', '3TrafficLights2'].includes(icon.set) ? ['#e04040', '#f1c232', '#52a34a'] : null;
  if (!colors || !colors[icon.index]) return false;
  ctx.save();
  ctx.fillStyle = colors[icon.index];
  ctx.beginPath();
  ctx.arc(x + size / 2, y + size / 2, size / 2, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
  return true;
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
