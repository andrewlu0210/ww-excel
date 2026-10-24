// Loads fonts WebKit does not offer to pages. macOS's downloadable fonts
// (楷體, 標楷體, 儷宋...) are installed but hidden from web content, so
// they are fetched from Go (/fonts/<family>.ttf, read through CoreText)
// and registered with the FontFace API.

const SAMPLE = '中文永 AgW 123';
const results = new Map<string, Promise<boolean>>();
let measure: CanvasRenderingContext2D | null = null;

// Bumped when a font finishes loading: cached text widths are stale.
export let fontEpoch = 0;
const listeners = new Set<() => void>();

export function onFontsLoaded(fn: () => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

// usable reports whether text set in family differs from the generic
// fallbacks, i.e. the page can use the font.
export function usable(family: string): boolean {
  measure ??= document.createElement('canvas').getContext('2d');
  if (!measure) return true;
  const width = (font: string) => {
    measure!.font = font;
    return measure!.measureText(SAMPLE).width;
  };
  const q = `"${family.replace(/"/g, '')}"`;
  return width(`20px ${q}, monospace`) !== width('20px monospace') || width(`20px ${q}, serif`) !== width('20px serif');
}

// ensureFont makes family usable if Go can provide it. Resolves to
// whether the font is usable.
export function ensureFont(family: string): Promise<boolean> {
  let p = results.get(family);
  if (!p) {
    p = load(family);
    results.set(family, p);
  }
  return p;
}

async function load(family: string): Promise<boolean> {
  if (typeof document === 'undefined' || usable(family)) return true;
  try {
    const res = await fetch(`/fonts/${encodeURIComponent(family)}.ttf`);
    if (!res.ok) return false;
    const face = new FontFace(family, await res.arrayBuffer());
    await face.load();
    document.fonts.add(face);
    fontEpoch++;
    for (const fn of listeners) fn();
    return true;
  } catch {
    return false;
  }
}
