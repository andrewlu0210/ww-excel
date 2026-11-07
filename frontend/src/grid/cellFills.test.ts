import { describe, expect, it } from 'vitest';
import { FILL_PATTERNS, patternInk, fillPosition, fillColor, validCellFill, representativeFill } from './cellFills';
import { createFormatDraft, formatDraftPatch } from '../components/cellFormatDraft';

describe('advanced fill rendering', () => {
  it('keeps each gray pattern density distinct and tiles continuous', () => {
    for (const [pattern,count] of [['darkGray',48],['mediumGray',32],['lightGray',16],['gray125',8],['gray0625',4]] as const) {
      let ink=0; for(let y=0;y<8;y++) for(let x=0;x<8;x++) {if(patternInk(pattern,x,y)) ink++;expect(patternInk(pattern,x+8,y+8)).toBe(patternInk(pattern,x,y));}
      expect(ink).toBe(count);
    }
    expect(FILL_PATTERNS).toHaveLength(17);
  });
  it('uses angle and path extents, interpolating arbitrary stop positions', () => {
    expect(fillPosition({kind:'linear',degree:0},0,.5)).toBe(0);
    expect(fillPosition({kind:'linear',degree:90},.5,1)).toBeCloseTo(1);
    expect(fillPosition({kind:'linear',degree:37},.5,1,280,42)).toBeLessThan(.6);
    expect(fillPosition({kind:'path',left:.5,right:.5,top:.5,bottom:.5},.5,.5)).toBe(0);
    expect(fillPosition({kind:'path',left:.5,right:.5,top:.5,bottom:.5},1,.5)).toBe(1);
    const stops=[{position:0,color:'FF0000'},{position:.25,color:'00FF00'},{position:1,color:'0000FF'}];
    expect(fillColor(stops,.25)).toEqual([0,255,0]);expect(fillColor(stops,.625)).toEqual([0,128,128]);
  });
  it('submits advanced fills and diagonals only when edited', () => {
    const initial=createFormatDraft(null),draft={...initial,fillSpec:{kind:'pattern',pattern:'darkGrid',foreground:'000000',background:'FFFFFF'},diagonal:{style:'double',color:'FF0000'},diagonalUp:true};
    const patch=formatDraftPatch(initial,draft,{mixed:[],edited:['fillSpec','diagonal','diagonalUp']});
    expect(patch.fillSpec?.kind).toBe('pattern');expect(patch.diagonalUp).toBe(true);expect(patch.diagonal?.style).toBe('double');expect(patch.fill).toBeUndefined();
    expect(formatDraftPatch(initial,draft,{mixed:[],edited:[]})).toEqual({});
  });
});

it('rejects invalid gradient bounds and stop colors before submission',()=>{
 expect(validCellFill({kind:'path',left:.9,right:.1,stops:JSON.stringify([{position:0,color:'FFFFFF'},{position:1,color:'000000'}])})).toBe(false);
 expect(validCellFill({kind:'linear',degree:45,stops:JSON.stringify([{position:0,color:'bad'},{position:1,color:'000000'}])})).toBe(false);
});

it('uses the fill brightness for automatic text contrast in dark mode',()=>{
 expect(representativeFill({kind:'pattern',background:'FFF2CC'})).toBe('#FFF2CC');
 expect(representativeFill({kind:'linear',stops:JSON.stringify([{position:0,color:'FFFFFF'},{position:1,color:'000000'}])})).toBe('#808080');
});
