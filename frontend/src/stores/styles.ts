// Workbook style table, synced from Go: styles arrive with block fetches,
// layout fetches and diff events, and are never removed while a document
// is open. Render info (CSS font, colors) is derived once per style.
import type { StyleDTO } from '../bridge/sheet';
import { ensureFont, onFontsLoaded } from './fontLoader';
import { fontFamilies } from './fonts';

export interface StyleRender {
  dto: StyleDTO;
  font: string; // CSS font for canvas
  fontPx: number;
  color: string | null; // explicit text color, null = theme default
  fill: string | null;
}

const PT_TO_PX = 4 / 3;
const FALLBACK = `"PingFang TC", "Heiti TC", -apple-system, sans-serif`;

export function fontCss(f: StyleDTO['font'], zoom = 1): { css: string; px: number } {
  const px = Math.max(1, (f.size || 11) * PT_TO_PX * zoom);
  // Each family is tried in turn, per character: Chinese text in a Latin
  // font falls through to a Chinese one, as in Excel.
  const names = f.name ? fontFamilies(f.name).map((n) => `"${n.replace(/"/g, '')}"`) : [];
  const family = names.length > 0 ? `${names.join(', ')}, ${FALLBACK}` : FALLBACK;
  return { css: `${f.italic ? 'italic ' : ''}${f.bold ? '700 ' : ''}${px.toFixed(2)}px ${family}`, px };
}

export class StyleStore {
  private list: (StyleRender | undefined)[] = [];
  private version = 0;
  private listeners = new Set<() => void>();

  // Calls fn whenever styles arrive; returns an unsubscribe function.
  subscribe(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  // Number of contiguous styles known from ID 0, sent to Go so it only
  // returns newer ones.
  get known(): number {
    let n = 0;
    while (this.list[n]) n++;
    return n;
  }

  get changes(): number {
    return this.version;
  }

  constructor() {
    // A font arriving later changes how cells look: redraw.
    onFontsLoaded(() => {
      this.version++;
      for (const fn of this.listeners) fn();
    });
  }

  merge(styles: readonly StyleDTO[] | null | undefined): void {
    if (!styles || styles.length === 0) return;
    for (const s of styles) {
      if (s.font.name) for (const f of fontFamilies(s.font.name)) void ensureFont(f);
      const { css, px } = fontCss(s.font);
      this.list[s.id] = {
        dto: s,
        font: css,
        fontPx: px,
        color: s.font.color ? `#${s.font.color}` : null,
        fill: s.fill ? `#${s.fill}` : null,
      };
    }
    this.version++;
    for (const fn of this.listeners) fn();
  }

  get(id: number): StyleRender | undefined {
    return this.list[id] ?? this.list[0];
  }

  reset(): void {
    this.list = [];
    this.version++;
  }
}

export const styleStore = new StyleStore();
