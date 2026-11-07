// Shared stroke metrics for the canvas grid and border editor preview.
export const BORDER_WIDTH: Record<string, number> = { medium: 2, thick: 3, double: 3, mediumDashed: 2, mediumDashDot: 2, mediumDashDotDot: 2 };
export const BORDER_DASH: Record<string, number[]> = {
  dashed: [3, 2],
  dotted: [1, 1],
  hair: [1, 1],
  mediumDashed: [6, 2],
  dashDot: [6, 2, 2, 2],
  mediumDashDot: [6, 2, 2, 2],
  dashDotDot: [6, 2, 2, 2, 2, 2],
  mediumDashDotDot: [6, 2, 2, 2, 2, 2],

};

// Diagonal dashes across an axis-aligned edge, shared by Canvas and SVG.
export function slantedBorderSegments(x0: number, y0: number, x1: number, y1: number): number[][] {
  const dx=x1-x0,dy=y1-y0,length=Math.hypot(dx,dy);
  const segments: number[][] = [];
  if (!length) return segments;
  const ux=dx/length,uy=dy/length;
  const nx=Math.abs(dx)>=Math.abs(dy)?-uy:uy,ny=Math.abs(dx)>=Math.abs(dy)?ux:-ux;
  for (let start=0;start<length;start+=6) {
    const end=Math.min(start+4,length);
    segments.push([x0+ux*start+nx,y0+uy*start+ny,x0+ux*end-nx,y0+uy*end-ny]);
  }
  return segments;
}
