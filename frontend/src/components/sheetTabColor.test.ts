import {describe,it,expect} from 'vitest';
import {sheetTabStyle} from './sheetTabColor';
describe('worksheet tab colors',()=>{
 it('uses readable text on dark and light imported tabs',()=>{
  expect(sheetTabStyle('000000')).toContain('--sheet-tab-text:#FFFFFF');
  expect(sheetTabStyle('FFFFFF')).toContain('--sheet-tab-text:#000000');
  expect(sheetTabStyle('00AA00')).toContain('--sheet-tab-color:#00AA00');
 });
 it('keeps a colored edge and a lighter active background',()=>{
  expect(sheetTabStyle('000000')).toContain('--sheet-tab-active-bg:#d9d9d9');
  expect(sheetTabStyle('123abc')).toContain('--sheet-tab-color:#123ABC');
 });
 it('uses the default appearance for absent or invalid metadata',()=>{
  for(const color of [null,undefined,'','123','GGGGGG','FF123456','red','123456;background:red'])expect(sheetTabStyle(color)).toBe('');
 });
});
