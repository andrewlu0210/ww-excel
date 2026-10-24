// Lightweight formula scanning for the editor: reference highlighting,
// point mode, F4 and parameter hints. The real parser lives in Go; this
// only needs token positions, so it is deliberately forgiving.

import { cellAddress } from '../grid/layout';
import type { CellPos, Range } from '../grid/selection';

export const MAX_ROWS = 1_048_576;
export const MAX_COLS = 16_384;

export interface RefToken {
  start: number; // offset in the text, including the sheet prefix
  end: number;
  sheet: string | null;
  range: Range;
  kind: 'cell' | 'col' | 'row';
}

const CELL = /^(\$?)([A-Za-z]{1,3})(\$?)(\d+)(?::(\$?)([A-Za-z]{1,3})(\$?)(\d+))?/;
const COLS = /^(\$?)([A-Za-z]{1,3}):(\$?)([A-Za-z]{1,3})/;
const ROWS = /^(\$?)(\d+):(\$?)(\d+)/;
const WORD = /^[A-Za-z_\\\u0080-￿][\w.\u0080-￿]*/;

function colIndex(letters: string): number {
  let n = 0;
  for (const ch of letters.toUpperCase()) n = n * 26 + (ch.charCodeAt(0) - 64);
  return n - 1;
}

const isWordChar = (ch: string | undefined) => ch !== undefined && /[\w.$\u0080-￿]/.test(ch);

function normalize(r0: number, c0: number, r1: number, c1: number): Range {
  return { r0: Math.min(r0, r1), c0: Math.min(c0, c1), r1: Math.max(r0, r1), c1: Math.max(c0, c1) };
}

// Tries to read a reference at text[i]; returns its length and area.
function readRef(text: string, i: number): { len: number; range: Range; kind: RefToken['kind'] } | null {
  const rest = text.slice(i);
  let m = COLS.exec(rest);
  if (m && !isWordChar(rest[m[0].length])) {
    const c0 = colIndex(m[2]);
    const c1 = colIndex(m[4]);
    if (c0 < MAX_COLS && c1 < MAX_COLS) {
      return { len: m[0].length, range: normalize(0, c0, MAX_ROWS - 1, c1), kind: 'col' };
    }
  }
  m = ROWS.exec(rest);
  if (m && !isWordChar(rest[m[0].length])) {
    const r0 = Number(m[2]) - 1;
    const r1 = Number(m[4]) - 1;
    if (r0 >= 0 && r1 >= 0 && r0 < MAX_ROWS && r1 < MAX_ROWS) {
      return { len: m[0].length, range: normalize(r0, 0, r1, MAX_COLS - 1), kind: 'row' };
    }
  }
  m = CELL.exec(rest);
  if (m) {
    const next = rest[m[0].length];
    if (isWordChar(next) || next === '(') return null; // e.g. LOG10(
    const c0 = colIndex(m[2]);
    const r0 = Number(m[4]) - 1;
    const c1 = m[6] ? colIndex(m[6]) : c0;
    const r1 = m[8] ? Number(m[8]) - 1 : r0;
    if (Math.max(c0, c1) >= MAX_COLS || Math.max(r0, r1) >= MAX_ROWS || Math.min(r0, r1) < 0) return null;
    return { len: m[0].length, range: normalize(r0, c0, r1, c1), kind: 'cell' };
  }
  return null;
}

// Skips a string literal starting at text[i] (a double quote).
function skipString(text: string, i: number): number {
  let j = i + 1;
  while (j < text.length) {
    if (text[j] === '"') {
      if (text[j + 1] === '"') {
        j += 2;
        continue;
      }
      return j + 1;
    }
    j++;
  }
  return j;
}

// All references in a formula ("=..."), in order.
export function findRefs(text: string): RefToken[] {
  const out: RefToken[] = [];
  if (!text.startsWith('=')) return out;
  let i = 1;
  while (i < text.length) {
    const ch = text[i];
    if (ch === '"') {
      i = skipString(text, i);
      continue;
    }
    if (isWordChar(text[i - 1]) && ch !== '$') {
      i++;
      continue;
    }
    let sheet: string | null = null;
    let start = i;
    let at = i;
    if (ch === "'") {
      const close = text.indexOf("'!", i + 1);
      if (close < 0) break;
      sheet = text.slice(i + 1, close).replace(/''/g, "'");
      at = close + 2;
    } else {
      const w = WORD.exec(text.slice(i));
      if (w && text[i + w[0].length] === '!') {
        sheet = w[0];
        at = i + w[0].length + 1;
      }
    }
    const ref = readRef(text, at);
    if (ref) {
      out.push({ start, end: at + ref.len, sheet, range: ref.range, kind: ref.kind });
      i = at + ref.len;
      continue;
    }
    // Skip the rest of a word (function names, TRUE, names).
    const w = WORD.exec(text.slice(i));
    i += w ? w[0].length : 1;
    start = i;
  }
  return out;
}

// Characters after which a reference can be inserted by pointing.
const POINT_AFTER = new Set(['=', '(', ',', '+', '-', '*', '/', '^', '&', '<', '>', ':', ';', '{', '%']);

// Whether a reference can be inserted at caret (point mode): the text is
// a formula and the last non-space character before the caret is an
// operator, "(" or ",".
export function canPoint(text: string, caret: number): boolean {
  if (!text.startsWith('=')) return false;
  let i = caret - 1;
  while (i > 0 && text[i] === ' ') i--;
  return i >= 0 && POINT_AFTER.has(text[i]) && !insideString(text, caret);
}

export function insideString(text: string, caret: number): boolean {
  let inside = false;
  for (let i = 0; i < caret && i < text.length; i++) {
    if (text[i] === '"') inside = !inside;
  }
  return inside;
}

export function refText(anchor: CellPos, focus: CellPos): string {
  if (anchor.row === focus.row && anchor.col === focus.col) return cellAddress(anchor.row, anchor.col);
  const r = normalize(anchor.row, anchor.col, focus.row, focus.col);
  return `${cellAddress(r.r0, r.c0)}:${cellAddress(r.r1, r.c1)}`;
}

// The function call the caret is in and which argument (0-based).
export function callAt(text: string, caret: number): { name: string; arg: number } | null {
  const stack: { name: string | null; arg: number }[] = [];
  let i = 1;
  while (i < caret && i < text.length) {
    const ch = text[i];
    if (ch === '"') {
      i = skipString(text, i);
      continue;
    }
    const w = WORD.exec(text.slice(i));
    if (w && !isWordChar(text[i - 1])) {
      const after = i + w[0].length;
      if (text[after] === '(') {
        stack.push({ name: w[0].toUpperCase(), arg: 0 });
        i = after + 1;
        continue;
      }
      i = after;
      continue;
    }
    if (ch === '(') stack.push({ name: null, arg: 0 });
    else if (ch === ')') stack.pop();
    else if (ch === ',' && stack.length > 0) stack[stack.length - 1].arg++;
    i++;
  }
  for (let k = stack.length - 1; k >= 0; k--) {
    const s = stack[k];
    if (s.name) return { name: s.name, arg: s.arg };
  }
  return null;
}

// The identifier being typed right before the caret, for function
// autocomplete; null unless it starts where a function could.
export function wordAt(text: string, caret: number): { start: number; word: string } | null {
  if (!text.startsWith('=') || insideString(text, caret)) return null;
  const before = text.slice(0, caret);
  const m = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before);
  if (!m) return null;
  const start = caret - m[0].length;
  if (!canPoint(text, start)) return null;
  if (isWordChar(text[caret])) return null;
  return { start, word: m[0] };
}

// F4 / ⌘T: cycles the reference at the caret through
// A1 -> $A$1 -> A$1 -> $A1 -> A1. Returns the new text and caret.
export function toggleAbsolute(text: string, caret: number): { text: string; caret: number } | null {
  const refs = findRefs(text);
  const ref = refs.find((r) => caret >= r.start && caret <= r.end) ?? refs.filter((r) => r.end <= caret).pop();
  if (!ref || ref.kind !== 'cell') return null;
  const raw = text.slice(ref.start, ref.end);
  const bang = raw.lastIndexOf('!');
  const prefix = bang >= 0 ? raw.slice(0, bang + 1) : '';
  const body = raw.slice(prefix.length);
  const parts = body.split(':').map((p) => {
    const m = /^(\$?)([A-Za-z]{1,3})(\$?)(\d+)$/.exec(p)!;
    const state = (m[1] ? 2 : 0) + (m[3] ? 1 : 0); // col$, row$
    // A1(0) -> $A$1(3) -> A$1(1) -> $A1(2) -> A1(0)
    const next = { 0: 3, 3: 1, 1: 2, 2: 0 }[state] as number;
    return `${next & 2 ? '$' : ''}${m[2].toUpperCase()}${next & 1 ? '$' : ''}${m[4]}`;
  });
  const replaced = prefix + parts.join(':');
  return {
    text: text.slice(0, ref.start) + replaced + text.slice(ref.end),
    caret: ref.start + replaced.length,
  };
}


// Quotes a sheet name for a reference when needed: 'My Sheet'!A1.
export function quoteSheet(name: string): string {
  if (/^[\p{L}_][\p{L}\p{N}_.]*$/u.test(name) && !/^[A-Za-z]{1,3}\d+$/.test(name)) return name;
  return `'${name.replace(/'/g, "''")}'`;
}

// Absolute reference to a range on a sheet: 工作表1!$A$1:$B$5.
export function absoluteRef(sheet: string, r0: number, c0: number, r1: number, c1: number): string {
  const abs = (row: number, col: number) => cellAddress(row, col).replace(/^([A-Z]+)(\d+)$/, '$$$1$$$2');
  const a = abs(r0, c0);
  const ref = r0 === r1 && c0 === c1 ? a : `${a}:${abs(r1, c1)}`;
  return `${quoteSheet(sheet)}!${ref}`;
}
