// View zoom never changes workbook geometry or stored object sizes.
export const MIN_ZOOM = 10;
export const MAX_ZOOM = 400;

export function normalizeZoom(value: number): number {
  return Number.isFinite(value) ? Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, Math.round(value))) : 100;
}

export function parseZoom(text: string, previous: number): number {
  const value = text.trim().replace(/%$/, '').trim();
  return /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(value) ? normalizeZoom(Number(value)) : previous;
}

// Include headers/frozen panes in logical dimensions and leave room for borders.
export function fitZoom(viewportWidth: number, viewportHeight: number, width: number, height: number): number {
  if (width <= 0 || height <= 0) return 100;
  return normalizeZoom(Math.floor(Math.min((viewportWidth - 4) / width, (viewportHeight - 4) / height) * 100));
}

export class ZoomTransform {
  readonly scale: number;

  constructor(percent: number) {
    this.scale = normalizeZoom(percent) / 100;
  }

  toScreen(value: number): number { return value * this.scale; }
  toLogical(value: number): number { return value / this.scale; }

  rect<T extends { x: number; y: number; w: number; h: number }>(rect: T): T {
    return { ...rect, x: this.toScreen(rect.x), y: this.toScreen(rect.y), w: this.toScreen(rect.w), h: this.toScreen(rect.h) };
  }
}

export class SheetZooms {
  private values = new Map<string, number>();

  get(name: string): number { return this.values.get(name) ?? 100; }
  set(name: string, value: number): void { this.values.set(name, normalizeZoom(value)); }
  clear(): void { this.values.clear(); }

  reconcile(previous: string[], next: string[]): void {
    const removed = previous.filter((name) => !next.includes(name));
    const added = next.filter((name) => !previous.includes(name));
    // Sheet names are unique; a single replacement is a rename (also on undo).
    if (previous.length === next.length && removed.length === 1 && added.length === 1) {
      this.values.set(added[0], this.get(removed[0]));
    }
    for (const name of removed) this.values.delete(name);
  }
}
