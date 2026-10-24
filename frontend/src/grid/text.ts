// Text measurement and wrapping for cell rendering, cached because the
// same strings are drawn every frame.

export type Measure = (text: string) => number;

// Breaks text into lines that fit width: at explicit line breaks, at
// spaces for Latin text, and anywhere in CJK text (as Excel does).
export function wrapText(text: string, width: number, measure: Measure): string[] {
  const out: string[] = [];
  for (const para of text.split('\n')) {
    if (para === '') {
      out.push('');
      continue;
    }
    let line = '';
    let lastSpace = -1; // index in line after which we can break
    for (const ch of para) {
      const next = line + ch;
      if (line !== '' && measure(next) > width) {
        if (ch === ' ') {
          out.push(line);
          line = '';
          lastSpace = -1;
          continue;
        }
        if (lastSpace > 0 && !isCJK(ch)) {
          out.push(line.slice(0, lastSpace).trimEnd());
          line = line.slice(lastSpace) + ch;
        } else {
          out.push(line);
          line = ch;
        }
        lastSpace = -1;
        continue;
      }
      line = next;
      if (ch === ' ' || isCJK(ch)) lastSpace = line.length;
    }
    out.push(line);
  }
  return out;
}

function isCJK(ch: string): boolean {
  const c = ch.codePointAt(0) ?? 0;
  return (c >= 0x2e80 && c <= 0x9fff) || (c >= 0xac00 && c <= 0xd7af) || (c >= 0xf900 && c <= 0xfaff) || (c >= 0xff00 && c <= 0xffef);
}

// A bounded cache keyed by font + text.
export class TextCache {
  private widths = new Map<string, number>();
  private wraps = new Map<string, string[]>();

  constructor(private ctx: CanvasRenderingContext2D) {}

  width(font: string, text: string): number {
    const key = font + '\u0000' + text;
    let w = this.widths.get(key);
    if (w === undefined) {
      this.ctx.font = font;
      w = this.ctx.measureText(text).width;
      if (this.widths.size > 50_000) this.widths.clear();
      this.widths.set(key, w);
    }
    return w;
  }

  wrap(font: string, text: string, width: number): string[] {
    const key = `${font}\u0000${Math.round(width)}\u0000${text}`;
    let lines = this.wraps.get(key);
    if (!lines) {
      lines = wrapText(text, width, (t) => this.width(font, t));
      if (this.wraps.size > 5_000) this.wraps.clear();
      this.wraps.set(key, lines);
    }
    return lines;
  }
}

export const LINE_HEIGHT = 1.22; // line height as a multiple of font px
