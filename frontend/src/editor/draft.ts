import type { FontDTO, TextRunDTO, FormatPatch } from '../../bindings/ww-excel/internal/service/models';
export type Run = Pick<TextRunDTO,'text'|'font'>;
export type Font = Pick<FontDTO,'name'|'size'|'color'|'bold'|'italic'|'underline'|'strike'>;
export type FontPatch = Pick<FormatPatch,'fontName'|'fontSize'|'fontColor'|'bold'|'italic'|'underline'|'strike'>;
interface Snapshot { runs:Run[]; explicit:boolean; start:number; end:number; pending:Font|null; pendingAt:number }
const same=(a:Font,b:Font)=>a.name===b.name && a.size===b.size && (a.color||'')===(b.color||'') && !!a.bold===!!b.bold && !!a.italic===!!b.italic && !!a.underline===!!b.underline && !!a.strike===!!b.strike;
const compact=(runs:Run[]):Run[]=>{
 const out:Run[]=[];
 for(const r of runs){if(!r.text)continue;const last=out.at(-1);if(last && same(last.font,r.font))last.text+=r.text;else out.push({text:r.text,font:{...r.font}});}
 return out;
};
// DOM offsets use UTF-16. Never split a surrogate pair when slicing a run.
function boundary(text:string,offset:number,end=false):number{
 offset=Math.max(0,Math.min(text.length,offset));
 if(offset>0 && offset<text.length && /[\uDC00-\uDFFF]/.test(text[offset]) && /[\uD800-\uDBFF]/.test(text[offset-1]))offset+=end?1:-1;
 return offset;
}
function slice(runs:Run[],start:number,end:number):Run[]{
 const out:Run[]=[];let at=0;
 for(const run of runs){const from=Math.max(0,start-at),to=Math.min(run.text.length,end-at);if(to>from)out.push({text:run.text.slice(from,to),font:run.font});at+=run.text.length;}
 return out;
}
export class RichDraft {
 runs:Run[];explicit:boolean;start=0;end=0;
 private past:Snapshot[]=[];private future:Snapshot[]=[];
 private pending:Font|null=null;private pendingAt=-1;
 constructor(text:string,readonly base:Font,runs:Run[]=[]){
  this.explicit=runs.length>0 && runs.map(r=>r.text).join('')===text;
  this.runs=this.explicit?compact(runs):text?[{text,font:{...base}}]:[];
 }
 get text(){return this.runs.map(r=>r.text).join('');}
 private snapshot():Snapshot{return{runs:this.runs.map(r=>({text:r.text,font:{...r.font}})),explicit:this.explicit,start:this.start,end:this.end,pending:this.pending?{...this.pending}:null,pendingAt:this.pendingAt};}
 private restore(s:Snapshot){this.runs=s.runs;this.explicit=s.explicit;this.start=s.start;this.end=s.end;this.pending=s.pending;this.pendingAt=s.pendingAt;}
 private record(){this.past.push(this.snapshot());if(this.past.length>200)this.past.shift();this.future=[];}
 undo(){const previous=this.past.pop();if(!previous)return false;this.future.push(this.snapshot());this.restore(previous);return true;}
 redo(){const next=this.future.pop();if(!next)return false;this.past.push(this.snapshot());this.restore(next);return true;}
 moveCaret(start:number,end:number){if(start!==this.pendingAt || start!==end){this.pending=null;this.pendingAt=-1;}this.start=start;this.end=end;}
 fontAt(offset:number,left=false):Font{
  if(this.pending && offset===this.pendingAt)return this.pending;
  let at=0;for(const r of this.runs){at+=r.text.length;if(offset<at || left && offset===at)return r.font;}
  return this.runs.at(-1)?.font??this.base;
 }
 selectionFont(start:number,end:number):Font{
  if(start===end)return {...this.fontAt(start,true)};
  const selected=slice(this.runs,start,end),font={...(selected[0]?.font??this.base)};
  for(const key of ['bold','italic','underline','strike'] as const)font[key]=selected.every(r=>!!r.font[key]);
  return font;
 }
 format(start:number,end:number,patch:FontPatch):void{
  const text=this.text;start=boundary(text,start);end=boundary(text,end,true);
  const apply=(font:Font):Font=>{
   const next={...font};for(const key of ['bold','italic','underline','strike'] as const)if(patch[key]!=null)next[key]=patch[key]!;
   if(patch.fontName!=null)next.name=patch.fontName;if(patch.fontSize!=null && patch.fontSize>0)next.size=patch.fontSize;if(patch.fontColor!=null)next.color=patch.fontColor;return next;
  };
  this.start=start;this.end=end;
  if(start===end){const font=this.fontAt(start,true),next=apply(font);if(same(font,next))return;this.record();this.pending=next;this.pendingAt=start;return;}
  const selected=slice(this.runs,start,end);if(selected.every(r=>same(r.font,apply(r.font))))return;
  this.record();this.runs=compact([...slice(this.runs,0,start),...selected.map(r=>({text:r.text,font:apply(r.font)})),...slice(this.runs,end,text.length)]);this.explicit=true;this.pending=null;this.pendingAt=-1;
 }
 replace(start:number,end:number,text:string):void{
  const old=this.text;start=boundary(old,start);end=boundary(old,end,true);
  if(old.slice(start,end)===text){this.start=this.end=start+text.length;return;}
  this.start=start;this.end=end;this.record();
  const font=this.pending && this.pendingAt===start && start===end?this.pending:this.fontAt(start,start===end);
  this.runs=compact([...slice(this.runs,0,start),{text,font},...slice(this.runs,end,old.length)]);
  if(this.pending && this.pendingAt===start){this.explicit=true;this.pendingAt=start+text.length;}else{this.pending=null;this.pendingAt=-1;}
  this.start=this.end=start+text.length;
 }
 // Prefer the native pre-input selection: repeated letters must retain the
 // format at the actual insertion point, rather than an ambiguous text diff.
 reconcile(text:string,start?:number,end?:number):void{
  const old=this.text;if(text===old)return;
  if(start!=null && end!=null && text.startsWith(old.slice(0,start)) && text.endsWith(old.slice(end)) && text.length>=start+old.length-end){this.replace(start,end,text.slice(start,text.length-(old.length-end)));return;}
  let from=0,toOld=old.length,toNew=text.length;
  while(from<toOld && from<toNew && old[from]===text[from])from++;
  from=boundary(old,from);
  while(toOld>from && toNew>from && old[toOld-1]===text[toNew-1]){toOld--;toNew--;}
  toOld=boundary(old,toOld,true);toNew=boundary(text,toNew,true);
  this.replace(from,toOld,text.slice(from,toNew));
 }
}
