import {describe,it,expect} from 'vitest';
import {chartLabelPositions} from './labelPositions';
describe('Excel label positions',()=>{
 it('limits line and scatter positions to point directions',()=>{for(const kind of ['line','scatter'])expect(chartLabelPositions(kind)).toEqual(['','ctr','t','b','l','r']);});
 it('omits outside-end for stacked bars',()=>{expect(chartLabelPositions('column')).toContain('outEnd');expect(chartLabelPositions('bar',true)).not.toContain('outEnd');});
 it('uses automatic position for doughnut and area charts',()=>{for(const kind of ['doughnut','area'])expect(chartLabelPositions(kind)).toEqual(['']);});
 it('retains pie-specific positions',()=>{expect(chartLabelPositions('pie')).toContain('bestFit');expect(chartLabelPositions('pie')).not.toContain('inBase');});
});
