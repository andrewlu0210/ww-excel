// UI translation. Source strings are the Traditional Chinese text in the
// code; Go owns the translation catalogs and sends the selected table
// at startup. The language is fixed for the page's lifetime: changing it
// reloads the window.
let dict: Record<string, string> = {};
let current = 'zh-TW';

interface Pattern {
  re: RegExp;
  en: string;
}
let patterns: Pattern[] = [];

const VERB = /%[sdvqcw]/g;

export function setLanguage(lang: string, table: Record<string, string> | null): void {
  current = lang;
  dict = lang !== 'zh-TW' && table ? table : {};
  patterns = Object.entries(dict)
    .filter(([zh]) => /%[sdvqcw]/.test(zh))
    // Most specific first: "%s（位置 %d）" must not win over longer keys.
    .sort((a, b) => b[0].replace(VERB, '').length - a[0].replace(VERB, '').length)
    .map(([zh, en]) => ({
      re: new RegExp('^' + zh.split(/%[sdvqcw]/).map(escape).join('(.+?)') + '$', 's'),
      en: en.replace(VERB, '%s'),
    }));
  if (typeof document !== 'undefined') document.documentElement.lang = lang;
}

function escape(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

export function language(): string {
  return current;
}

// t translates s; %s / %d placeholders are filled from args in order.
// A "context|" prefix tells apart keys with the same Chinese text (the
// Chinese UI shows the part after "|").
export function t(s: string, ...args: (string | number)[]): string {
  let out = dict[s] ?? s.slice(s.indexOf('|') + 1);
  for (const a of args) out = out.replace(/%[sd]/, String(a));
  return out;
}

// tr translates a finished message, e.g. an error from Go that was
// formatted from a key with placeholders.
export function tr(msg: string): string {
  if (current === 'zh-TW' || !msg) return msg;
  const exact = dict[msg];
  if (exact !== undefined) return exact;
  for (const p of patterns) {
    const m = p.re.exec(msg);
    if (!m) continue;
    let out = p.en;
    for (const arg of m.slice(1)) out = out.replace('%s', () => tr(arg));
    return out;
  }
  return msg;
}
