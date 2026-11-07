import { describe, expect, it } from 'vitest';
import { createFormatDraft, formatDraftPatch } from './cellFormatDraft';
import type { StyleDTO } from '../bridge/sheet';
const style: StyleDTO = {
  id: 4, font: { name: 'Arial', size: 16, bold: true, color: 'C00000' },
  fill: 'FFFFFF', alignH: 'right', alignV: 'top', wrap: true, numFmt: '0.00%',
  left: { style: 'double', color: '123456' }, right: {}, top: {}, bottom: {},
};
describe('cell format dialog draft', () => {
  it('initializes supported properties without changing the source', () => {
    const draft = createFormatDraft(style);
    expect(draft).toMatchObject({ fontName: 'Arial', fontSize: 16, bold: true, fontColor: 'C00000', fill: 'FFFFFF', alignH: 'right', alignV: 'top', wrap: true, numFmt: '0.00%' });
    draft.bold = false;
    expect(style.font.bold).toBe(true);
  });
  it('keeps untouched properties out of the patch', () => {
    const initial = createFormatDraft(style);
    expect(formatDraftPatch(initial, { ...initial })).toEqual({});
    expect(formatDraftPatch(initial, { ...initial, fill: '' })).toEqual({ fill: '' });
  });
  it('combines cross-tab changes including explicit false and empty values', () => {
    const initial = createFormatDraft(style);
    expect(formatDraftPatch(initial, { ...initial, bold: false, wrap: false, numFmt: '', alignH: '', border: 'none', fill: '' })).toEqual({ bold: false, wrap: false, numFmt: '', alignH: '', border: 'none', borderColor: '', fill: '' });
  });
  it('only sends border color with a selected preset', () => {
    const initial = createFormatDraft(style);
    expect(formatDraftPatch(initial, { ...initial, borderColor: 'FF0000' })).toEqual({});
    expect(formatDraftPatch(initial, { ...initial, border: 'outside', borderColor: 'FF0000' })).toEqual({ border: 'outside', borderColor: 'FF0000' });
  });
  it('provides empty-cell defaults and preserves custom font sizes', () => {
    expect(createFormatDraft(null)).toMatchObject({ fontName: 'Calibri', fontSize: 11, fill: '', numFmt: '', border: '' });
    expect(createFormatDraft({ ...style, font: { name: 'Custom', size: 13.5 } }).fontSize).toBe(13.5);
  });
});

it('only sends explicitly edited mixed fields, including the initial value', () => {
  const initial = createFormatDraft(style);
  expect(formatDraftPatch(initial, { ...initial, fill: 'FFFFFF', bold: false }, { mixed: ['fill', 'bold'], edited: ['fill'] })).toEqual({ fill: 'FFFFFF' });
  expect(formatDraftPatch(initial, { ...initial }, { mixed: ['fill'], edited: [] })).toEqual({});
  expect(formatDraftPatch(initial, { ...initial }, { mixed: [], edited: ['fill'] })).toEqual({});
  expect(formatDraftPatch(initial, { ...initial, fill: '' }, { mixed: ['fill'], edited: ['fill'] })).toEqual({ fill: '' });
});
