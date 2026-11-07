import type { FormatPatch, StyleDTO } from '../bridge/sheet';
import type { BorderEdits } from './borderDraft';

export function createFormatDraft(style: StyleDTO | null) {
  return {
    numFmt: style?.numFmt ?? '',
    fontName: style?.font.name || 'Calibri', fontSize: style?.font.size || 11,
    bold: style?.font.bold ?? false, italic: style?.font.italic ?? false,
    underline: style?.font.underline ?? false, strike: style?.font.strike ?? false,
    fillSpec: { ...style?.fillSpec }, diagonal: {...style?.diagonal}, diagonalUp: style?.diagonalUp ?? false, diagonalDown: style?.diagonalDown ?? false,
    fontColor: style?.font.color ?? '', fill: style?.fill ?? '',
    alignH: style?.alignH ?? '', alignV: style?.alignV ?? '', wrap: style?.wrap ?? false,
    border: '', borderColor: '', borderStyle: 'thin', borders: {} as BorderEdits,
  };
}
export type CellFormatDraft = ReturnType<typeof createFormatDraft>;

// Send only edited properties; opening the dialog must not flatten a selection.
export type EditableFormatField = Exclude<keyof CellFormatDraft, 'border' | 'borderColor' | 'borderStyle' | 'borders'>;
export function formatDraftPatch(initial: CellFormatDraft, draft: CellFormatDraft, edits?: { mixed: string[]; edited: string[]; borderColorEdited?: boolean; borderColorMixed?: boolean; borderStyleEdited?: boolean; borderStyleMixed?: boolean }): FormatPatch {
  const patch: Record<string, unknown> = {};
  for (const key of Object.keys(initial) as (keyof CellFormatDraft)[]) {
    if (key === 'border' || key === 'borderColor' || key === 'borderStyle' || key === 'borders') continue;
    if (edits && !edits.edited.includes(key)) continue;
    if (JSON.stringify(draft[key]) !== JSON.stringify(initial[key]) || edits?.mixed.includes(key)) patch[key] = draft[key];
  }
  if (draft.border) {
    patch.border = draft.border;
    patch.borderColor = draft.borderColor;
  }
  const borders = Object.fromEntries(Object.entries(draft.borders).filter(([, line]) => line !== undefined));
  if (Object.keys(borders).length) patch.borders = borders;
  if (patch.fill !== undefined && !draft.fillSpec.kind) delete patch.fillSpec;
  else if (patch.fillSpec && draft.fillSpec.kind) delete patch.fill;
  if (edits?.borderColorEdited && (draft.borderColor !== initial.borderColor || edits.borderColorMixed) && !Object.keys(borders).length && !edits.edited.some(key => ['diagonal','diagonalUp','diagonalDown'].includes(key))) patch.recolorBorders = draft.borderColor;
  if (edits?.borderStyleEdited && (draft.borderStyle !== initial.borderStyle || edits.borderStyleMixed) && !Object.keys(borders).length && !edits.edited.some(key => ['diagonal','diagonalUp','diagonalDown'].includes(key))) patch.restyleBorders = draft.borderStyle;
  return patch as FormatPatch;
}
