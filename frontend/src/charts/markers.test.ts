import {it,expect} from 'vitest';
import {drawChartMarker} from './markers';
it.each(['auto','circle','square','diamond','triangle','x','plus','star','dash','dot'])('draws %s marker without moving chart coordinates',symbol=>{
 const commands:string[]=[];const ctx=new Proxy({fillStyle:'#FF0000'},{get(t,k){return k in t?t[k as keyof typeof t]:(...args:unknown[])=>{commands.push(`${String(k)}:${args.join(',')}`)}}}) as unknown as CanvasRenderingContext2D;
 drawChartMarker(ctx,20,30,3,symbol);expect(commands.some(c=>c.startsWith('fill:')||c.startsWith('stroke:'))).toBe(true);
 if(symbol==='square')expect(commands).toContain('rect:17,27,6,6');if(symbol==='star')expect(commands.filter(c=>c.startsWith('lineTo:'))).toHaveLength(9);
});
it('does not draw the none marker',()=>{let calls=0;const ctx=new Proxy({},{get:()=>()=>calls++}) as CanvasRenderingContext2D;drawChartMarker(ctx,0,0,3,'none');expect(calls).toBe(0);});
