import { describe, expect, it } from 'vitest';
import { RichDraft } from './draft';
const base={name:'Arial',size:11,color:'000000'};
const red={...base,color:'FF0000',bold:true};
const imported=()=>new RichDraft('標題內文',base,[{text:'標題',font:red},{text:'內文',font:base}]);
describe('rich editor draft',()=>{
 it('formats only a selection across runs and keeps explicit false',()=>{
  const d=imported();d.format(1,3,{bold:false,italic:true,fontColor:'0000FF'});
  expect(d.runs).toEqual([{text:'標',font:red},{text:'題內',font:{...base,bold:false,italic:true,color:'0000FF'}},{text:'文',font:base}]);
 });
 it('inherits left on insertion, first removed on replacement and first font at start',()=>{
  const d=imported();d.replace(2,2,'新');expect(d.runs[0]).toEqual({text:'標題新',font:red});
  d.replace(2,4,'換');expect(d.runs[0].text).toBe('標題換');
  d.replace(0,0,'首');expect(d.runs[0].font).toEqual(red);
 });
 it('retains Unicode, blank lines and styles across a composition replacement',()=>{
  const d=imported();d.reconcile('標題中',2,4);d.reconcile('標題中文🙂\n',2,3);
  expect(d.text).toBe('標題中文🙂\n');expect(d.runs[0].text).toBe('標題');expect(d.runs[1].font).toEqual(base);
 });
 it('keeps a chosen insertion font until the caret moves',()=>{
  const d=imported();d.format(2,2,{fontColor:'0000FF'});d.replace(2,2,'🙂');d.replace(4,4,'新');
  expect(d.runs[1].text).toBe('🙂新');expect(d.runs[1].font.color).toBe('0000FF');
  d.moveCaret(1,1);d.replace(1,1,'甲');expect(d.runs[0].text).toBe('標甲題');
 });
 it('undoes text and formatting in the same order and invalidates redo after new input',()=>{
  const d=imported();d.format(0,2,{bold:false});d.replace(4,4,'新');
  expect(d.undo()).toBe(true);expect(d.text).toBe('標題內文');expect(d.runs[0].font.bold).toBe(false);
  d.undo();expect(d.runs[0].font.bold).toBe(true);d.redo();expect(d.runs[0].font.bold).toBe(false);
  d.replace(4,4,'改');expect(d.redo()).toBe(false);
 });
 it('does not create a history entry for unchanged text or formatting',()=>{
  const d=imported();d.reconcile(d.text);d.format(0,2,{bold:true});expect(d.undo()).toBe(false);
 });
 it('normalizes offsets around surrogate pairs',()=>{
  const d=new RichDraft('甲🙂乙',base);d.format(2,3,{bold:true});
  expect(d.runs.map(r=>r.text)).toEqual(['甲','🙂','乙']);d.replace(2,3,'換');expect(d.text).toBe('甲換乙');
 });
});
