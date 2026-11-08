import { RichDraft, type FontPatch, type Run } from './draft';
import { fontCss } from '../stores/styles';

// A text-only DOM editor with styled spans. Input methods own the DOM during
// composition; normalization and rendering wait until compositionend.
function domText(node:Node):string {
 if(node.nodeType===Node.TEXT_NODE)return node.textContent??'';
 if(node instanceof HTMLElement && node.dataset.editorTail!==undefined)return '';
 if(node.nodeName==='BR')return '\n';
 // WebKit leaves a placeholder BR after deleting the last character.
 // Explicit newlines are text nodes, so the placeholder is an empty value.
 if((node.nodeName==='DIV' || node.nodeName==='P') && node.childNodes.length===1 && node.firstChild?.nodeName==='BR')return '';
 let text='';
 for(const child of node.childNodes){
  const block=child.nodeName==='DIV' || child.nodeName==='P';
  if(block && text && !text.endsWith('\n'))text+='\n';
  text+=domText(child);
 }
 return text;
}
export class RichEditor {
 draft=new RichDraft('',{name:'Calibri',size:11});
 private start=0;private end=0;
 private before:{start:number;end:number}|null=null;
 private composition:{start:number;end:number}|null=null;
 private composing=false;
 private zoom=1;
 constructor(readonly element:HTMLDivElement,private changed:()=>void,private plainFormulas=true){
  element.addEventListener('beforeinput',this.beforeInput);
  element.addEventListener('paste',this.paste);
 }
 destroy(){this.element.removeEventListener('beforeinput',this.beforeInput);this.element.removeEventListener('paste',this.paste);}
 attach(draft:RichDraft,preserveDOM=false){this.draft=draft;if(!preserveDOM)this.render();}
 get value(){return this.draft.text;}
 set value(text:string){this.draft.reconcile(text);this.render();}
 get style(){return this.element.style;}
 get scrollHeight(){return this.element.scrollHeight;}
 getBoundingClientRect(){return this.element.getBoundingClientRect();}
 addEventListener(...args:Parameters<HTMLDivElement['addEventListener']>){this.element.addEventListener(...args);}
 dispatchEvent(event:Event){return this.element.dispatchEvent(event);}
 focus(options?:FocusOptions){this.element.focus(options);}
 get selectionStart(){this.captureSelection();return this.start;}
 get selectionEnd(){this.captureSelection();return this.end;}
 captureSelection():boolean{
  const selection=window.getSelection();
  if(!selection?.rangeCount || !this.element.contains(selection.anchorNode) || !this.element.contains(selection.focusNode))return false;
  const range=selection.getRangeAt(0),prefix=document.createRange();prefix.selectNodeContents(this.element);prefix.setEnd(range.startContainer,range.startOffset);
  this.start=domText(prefix.cloneContents()).length;
  prefix.setEnd(range.endContainer,range.endOffset);this.end=domText(prefix.cloneContents()).length;
  this.start=Math.min(this.value.length,this.start);this.end=Math.min(this.value.length,this.end);
  this.draft.moveCaret(this.start,this.end);return true;
 }
 setSelectionRange(start:number,end:number){
  this.start=Math.max(0,Math.min(this.value.length,start));this.end=Math.max(this.start,Math.min(this.value.length,end));
  if(document.activeElement!==this.element)return;
  const locate=(offset:number):[Node,number]=>{
   const walker=document.createTreeWalker(this.element,NodeFilter.SHOW_TEXT);let node:Node|null;
   while((node=walker.nextNode())){const length=node.textContent?.length??0;if(offset<=length)return[node,offset];offset-=length;}
   return[this.element,this.element.childNodes.length];
  };
  const [a,ao]=locate(this.start),[b,bo]=locate(this.end),range=document.createRange();range.setStart(a,ao);range.setEnd(b,bo);
  const selection=window.getSelection();selection?.removeAllRanges();selection?.addRange(range);
 }
 select(){this.setSelectionRange(0,this.value.length);}
 setRangeText(text:string,start:number,end:number,_mode?:string){
  this.draft.replace(start,end,text);this.render();this.setSelectionRange(this.draft.start,this.draft.end);
 }
 scale(value:number){if(value===this.zoom)return;this.captureSelection();this.zoom=value;this.render();}
 render(){
  if(this.composing)return;
  const fragment=document.createDocumentFragment();
  const runs:Run[]=this.plainFormulas && this.value.startsWith('=')?[{text:this.value,font:this.draft.base}]:this.draft.runs;
  for(const run of runs){
   const span=document.createElement('span');span.textContent=run.text;
   // The formula bar keeps a readable fixed size; the cell editor previews
   // the actual point size, and both edit the same font attributes.
   const font=this.element.classList.contains('formula')?{...run.font,size:9.75}:run.font;
   span.style.font=fontCss(font,this.zoom).css;
   span.style.color=run.font.color?`#${run.font.color}`:'';
   span.style.textDecoration=[run.font.underline?'underline':'',run.font.strike?'line-through':''].filter(Boolean).join(' ')||'none';
   fragment.append(span);
  }
  if(!this.value || this.value.endsWith('\n')){const tail=document.createElement('br');tail.dataset.editorTail='';fragment.append(tail);}
  this.element.replaceChildren(fragment);this.setSelectionRange(this.start,this.end);
 }
 readInput(){
  if(this.composing)return;
  // Read caret offsets against the new DOM before updating the draft length.
  const selection=window.getSelection();let start=0,end=0;
  if(selection?.rangeCount && this.element.contains(selection.anchorNode)){
   const range=selection.getRangeAt(0),prefix=document.createRange();prefix.selectNodeContents(this.element);prefix.setEnd(range.startContainer,range.startOffset);start=domText(prefix.cloneContents()).length;
   prefix.setEnd(range.endContainer,range.endOffset);end=domText(prefix.cloneContents()).length;
  }
  this.draft.reconcile(domText(this.element),this.before?.start,this.before?.end);this.before=null;
  this.start=start;this.end=end;this.render();
 }
 compositionStart(){this.captureSelection();this.composing=true;this.composition={start:this.start,end:this.end};this.before=this.composition;}
 compositionEnd(){this.composing=false;this.before=this.composition;this.composition=null;this.readInput();}
 get isComposing(){return this.composing;}
 format(patch:FontPatch){this.captureSelection();this.draft.format(this.start,this.end,patch);this.render();}
 history(redo=false){
  this.captureSelection();if(!(redo?this.draft.redo():this.draft.undo()))return;
  this.start=this.draft.start;this.end=this.draft.end;this.focus({preventScroll:true});this.render();this.changed();
 }
 private beforeInput=(event:InputEvent)=>{
  if(this.composing || event.isComposing)return;
  this.captureSelection();this.before={start:this.start,end:this.end};
  if(event.inputType==='historyUndo' || event.inputType==='historyRedo'){event.preventDefault();this.history(event.inputType==='historyRedo');}
  else if(event.inputType==='insertParagraph' || event.inputType==='insertLineBreak'){
   event.preventDefault();this.setRangeText('\n',this.start,this.end);this.changed();
  }
 };
 private paste=(event:ClipboardEvent)=>{
  // Idle paste belongs to the grid; never import foreign HTML into the editor.
  if(!this.element.classList.contains('active') && !this.element.classList.contains('formula'))return;
  event.preventDefault();this.captureSelection();this.setRangeText((event.clipboardData?.getData('text/plain')??'').replace(/\r\n?/g,'\n'),this.start,this.end);this.changed();
 };
}
