import type { TextRunDTO } from '../../bindings/ww-excel/internal/service/models';
import { fontCss } from '../stores/styles';
import { ensureFont } from '../stores/fontLoader';
import { fontFamilies } from '../stores/fonts';
import { LINE_HEIGHT } from './text';

export interface StyledRun {
  text: string;
  font: string;
  px: number;
  color: string;
  underline?: boolean;
  strike?: boolean;
}
export interface RichLine { runs: StyledRun[]; width: number; px: number }
type Glyph = { text: string; style: StyledRun };
export type RichMeasure = (font: string, text: string) => number;

export function styledRuns(runs: TextRunDTO[], fallback: string): StyledRun[] {
  return runs.map((run) => {
    if (run.font.name) for (const name of fontFamilies(run.font.name)) void ensureFont(name);
    const font = fontCss(run.font);
    return { text: run.text, font: font.css, px: font.px, color: run.font.color ? `#${run.font.color}` : fallback, underline: run.font.underline, strike: run.font.strike };
  });
}

// Word wrapping retains run boundaries, explicit blank lines, and CJK breaks.
export function layoutRichText(runs: StyledRun[], width: number, wrap: boolean, measure: RichMeasure, defaultPx = 14.67): RichLine[] {
  const compact = (glyphs: Glyph[]): RichLine => {
    const spans: StyledRun[] = [];
    let previous: StyledRun | undefined;
    for (const glyph of glyphs) {
      if (previous === glyph.style) spans[spans.length - 1].text += glyph.text;
      else { spans.push({ ...glyph.style, text: glyph.text }); previous = glyph.style; }
    }
    return { runs: spans, width: spans.reduce((sum, r) => sum + measure(r.font, r.text), 0), px: Math.max(defaultPx, ...spans.map((r) => r.px)) };
  };
  const out: RichLine[] = [];
  let line: Glyph[] = [];
  const cjk = (ch: string) => /[\u2e80-\u9fff\uac00-\ud7af\uf900-\ufaff\uff00-\uffef]/u.test(ch);
  const push = (glyphs: Glyph[]) => out.push(compact(glyphs));
  for (const run of runs) {
    for (let ch of run.text.replace(/\r\n?/g, '\n')) {
      if (ch === '\n' && wrap) { push(line); line = []; continue; }
      if (ch === '\n') ch = ' ';
      const glyph = { text: ch, style: run };
      if (wrap && line.length && compact([...line, glyph]).width > width) {
        if (ch === ' ') { push(line); line = []; continue; }
        let at = -1;
        for (let i = 0; i < line.length; i++) if (line[i].text === ' ' || cjk(line[i].text)) at = i + 1;
        if (at > 0 && !cjk(ch)) {
          const before = line.slice(0, at);
          while (before.at(-1)?.text === ' ') before.pop();
          // Leading indentation alone must not create an empty wrapped line.
          if (before.length) { push(before); line = line.slice(at); }
          else { push(line); line = []; }
        } else { push(line); line = []; }
      }
      line.push(glyph);
    }
  }
  push(line);
  return out;
}

export function drawRichLines(ctx: CanvasRenderingContext2D, lines: RichLine[], measure: RichMeasure,
  box: { x: number; y: number; w: number; h: number }, alignH: string, alignV: string,
  pad: { left: number; right: number; top: number; bottom: number }): void {
  const total = lines.reduce((sum, line) => sum + line.px * LINE_HEIGHT, 0);
  const available = box.h - pad.top - pad.bottom;
  let top = box.y + pad.top + (alignV === 'center' ? (available - total) / 2 : alignV === 'top' ? 0 : available - total);
  ctx.textBaseline = 'alphabetic';
  for (const line of lines) {
    let x = alignH === 'right' ? box.x + box.w - pad.right - line.width : alignH === 'center' ? box.x + (box.w - line.width) / 2 : box.x + pad.left;
    const y = top + line.px * 0.98;
    for (const run of line.runs) {
      ctx.font = run.font;
      ctx.fillStyle = run.color;
      ctx.fillText(run.text, x, y);
      const width = measure(run.font, run.text);
      if (run.underline) ctx.fillRect(x, y + 2, width, Math.max(1, run.px / 14));
      if (run.strike) ctx.fillRect(x, y - run.px * 0.3, width, Math.max(1, run.px / 14));
      x += width;
    }
    top += line.px * LINE_HEIGHT;
  }
}
