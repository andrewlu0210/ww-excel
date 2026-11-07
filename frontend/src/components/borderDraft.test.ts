import { describe, expect, it } from 'vitest';
import { borderPresetDraft, toggleBorderEdge, borderLineForPreview } from './borderDraft';
import { createFormatDraft, formatDraftPatch } from './cellFormatDraft';

describe('border editing draft', () => {
  const red = { style: 'double', color: 'FF0000' };
  it('cycles an edge through apply, clear and leave unchanged', () => {
    const set = toggleBorderEdge({}, 'top', red);
    expect(set).toEqual({ top: red });
    const cleared = toggleBorderEdge(set, 'top', red);
    expect(cleared).toEqual({ top: {} });
    expect(toggleBorderEdge(cleared, 'top', red)).toEqual({});
  });
  it('repaints an edge when the brush changes, preserving other edges', () => {
    const original = { top: red, left: {} };
    expect(toggleBorderEdge(original, 'top', { style: 'thin', color: '' })).toEqual({ top: { style: 'thin', color: '' }, left: {} });
    expect(original.top).toEqual(red);
  });
  it('presets preserve edges they do not modify and explicitly clear all six', () => {
    expect(borderPresetDraft({ insideH: red }, 'outside', red)).toEqual({ top: red, bottom: red, left: red, right: red, insideH: red });
    expect(borderPresetDraft({}, 'inside', red)).toEqual({ insideH: red, insideV: red });
    expect(borderPresetDraft({}, 'none', red)).toEqual({ top: {}, bottom: {}, left: {}, right: {}, insideH: {}, insideV: {} });
  });
  it('submits only edited edges, including explicit clears', () => {
    const initial = createFormatDraft(null);
    expect(formatDraftPatch(initial, { ...initial, borders: { left: {}, insideH: red }, borderStyle: 'double', borderColor: 'FF0000' })).toEqual({ borders: { left: {}, insideH: red } });
    expect(formatDraftPatch(initial, { ...initial, borderStyle: 'double', borderColor: 'FF0000' })).toEqual({});
  });
  it('previews unchanged original lines and explicit removal separately', () => {
    expect(borderLineForPreview({}, 'left', red)).toEqual(red);
    expect(borderLineForPreview({ left: {} }, 'left', red)).toEqual({});
  });
});

import { initialBorderBrush } from './borderDraft';

it('reopens an existing colored border with its saved brush color and style', () => {
  expect(initialBorderBrush({id:1,font:{name:'Arial',size:11},left:{style:'double',color:'FF0000'},right:{},top:{},bottom:{}})).toEqual({style:'double',color:'FF0000',mixed:false,styleMixed:false});
});
it('uses range summary colors and does not represent mixed borders as automatic', () => {
  expect(initialBorderBrush(null,{top:{style:'thin',color:'FF0000'},bottom:{style:'double',color:'0070C0'}})).toEqual({style:'thin',color:'',mixed:true,styleMixed:true});
  expect(initialBorderBrush(null,{},['top']).mixed).toBe(true);
  expect(initialBorderBrush(null,{top:{style:'thin'}})).toEqual({style:'thin',color:'',mixed:false,styleMixed:false});
});

it('submits a color-only edit while preserving explicit per-edge color choices', () => {
  const initial=createFormatDraft(null),draft={...initial,borderColor:'FF0000'};
  expect(formatDraftPatch(initial,draft,{mixed:[],edited:[],borderColorEdited:true})).toEqual({recolorBorders:'FF0000'});
  expect(formatDraftPatch(initial,{...draft,borderColor:''},{mixed:[],edited:[],borderColorEdited:true,borderColorMixed:true})).toEqual({recolorBorders:''});
  expect(formatDraftPatch(initial,{...draft,borders:{top:{style:'double',color:'0070C0'}}},{mixed:[],edited:[],borderColorEdited:true})).toEqual({borders:{top:{style:'double',color:'0070C0'}}});
  expect(formatDraftPatch(initial,draft,{mixed:[],edited:[],borderColorEdited:false})).toEqual({});
  expect(formatDraftPatch(initial,{...initial},{mixed:[],edited:[],borderColorEdited:true})).toEqual({});
});


import { previewBorderBrush } from './borderDraft';
it('updates outside lines when style and color are picked after the preset',()=>{
 const thin={style:'thin',color:''},red={style:'double',color:'FF0000'};
 const targets=['top','bottom','left','right'] as const;
 const first=borderPresetDraft({},'outside',thin);
 const preview=previewBorderBrush(first,[...targets],null,red);
 for(const edge of targets) expect(preview.value[edge]).toEqual(red);
 expect(preview.baseline).toEqual(first);
 expect(first.top).toEqual(thin);
});
it('keeps red outside and blue inside when the next target uses a different brush',()=>{
 const red={style:'double',color:'FF0000'},blue={style:'dotted',color:'0070C0'};
 const outside=borderPresetDraft({},'outside',red);
 const provisional=previewBorderBrush(outside,['top','bottom','left','right'],null,blue);
 const final=borderPresetDraft(provisional.baseline ?? provisional.value,'inside',blue);
 expect(final.top).toEqual(red);expect(final.bottom).toEqual(red);
 expect(final.insideH).toEqual(blue);expect(final.insideV).toEqual(blue);
});
it('does not recreate explicitly cleared lines when the brush changes',()=>{
 const draft={top:{}};expect(previewBorderBrush(draft,['top'],null,{style:'double',color:'FF0000'})).toEqual({value:draft,baseline:null});
});
it('submits style-only changes for existing lines and combines them with color',()=>{
 const initial=createFormatDraft(null),draft={...initial,borderStyle:'double',borderColor:'FF0000'};
 expect(formatDraftPatch(initial,draft,{mixed:[],edited:[],borderStyleEdited:true})).toEqual({restyleBorders:'double'});
 expect(formatDraftPatch(initial,draft,{mixed:[],edited:[],borderStyleEdited:true,borderColorEdited:true})).toEqual({restyleBorders:'double',recolorBorders:'FF0000'});
});
