<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { t } from '../i18n';
  import type { ObjectDTO, FormatPatch, TextRunDTO } from '../bridge/sheet';
  import { RichDraft, type Font } from '../editor/draft';
  import { RichEditor } from '../editor/editable';

  interface Props {
    object:ObjectDTO;
    rect:{x:number;y:number;w:number;h:number};
    scale:number;
    onfont:(font:Font)=>void;
    oncommit:()=>void;
    oncancel:()=>void;
  }
  let {object,rect,scale,onfont,oncommit,oncancel}:Props=$props();
  const shape=untrack(()=>object.shape!);
  const initial=shape.runs??[];
  const base=initial[0]?.font??{name:'Calibri',size:11,color:'000000'};
  const draft=new RichDraft(initial.map(r=>r.text).join(''),base,initial);
  let control:RichEditor;
  const width=()=> (shape.w || object.w)*scale;
  const height=()=> (shape.h || object.h)*scale;

  function announce(){if(!control)return;onfont(draft.selectionFont(control.selectionStart,control.selectionEnd));}
  function attach(node:HTMLDivElement){
    control=new RichEditor(node,announce,false);control.attach(draft);control.scale(scale);
    return{destroy:()=>control.destroy()};
  }
  $effect(()=>{const zoom=scale;if(control)control.scale(zoom);});
  export function format(patch:FormatPatch){if(control.isComposing)return;control.format(patch);control.focus({preventScroll:true});control.setSelectionRange(draft.start,draft.end);announce();}
  export function history(redo=false){if(!control.isComposing)control.history(redo);announce();}
  export function selectAll(){control.select();announce();}
  export function runs():TextRunDTO[]{return draft.runs.length?draft.runs:[{text:'',font:draft.fontAt(0,true)}];}
  export function composing(){return control.isComposing;}
  export function selectText(start:number,end:number){control.focus();control.setSelectionRange(start,end);announce();}
  function input(){control.readInput();if(!control.isComposing)announce();}
  function key(e:KeyboardEvent){
    e.stopPropagation();if(e.isComposing || control.isComposing)return;
    if(e.key==='Escape'){e.preventDefault();oncancel();return;}
    if(e.key==='Tab' || e.key==='Enter' && (e.metaKey || e.ctrlKey)){e.preventDefault();oncommit();return;}
    if(e.metaKey || e.ctrlKey){
      const key=e.key.toLowerCase();
      if(key==='z'){e.preventDefault();history(e.shiftKey);}
      else if(['b','i','u'].includes(key)){e.preventDefault();const attr=key==='b'?'bold':key==='i'?'italic':'underline';const font=draft.selectionFont(control.selectionStart,control.selectionEnd);format({[attr]:!font[attr]});}
    }
  }
  onMount(()=>{
    control.focus({preventScroll:true});control.setSelectionRange(draft.text.length,draft.text.length);announce();
    const changed=()=>{if(control.captureSelection())announce();};document.addEventListener('selectionchange',changed);
    return()=>document.removeEventListener('selectionchange',changed);
  });
</script>

<div class="shape-text-editor"
  style:left="{rect.x+(rect.w-width())/2}px" style:top="{rect.y+(rect.h-height())/2}px"
  style:width="{width()}px" style:height="{height()}px"
  style:transform="rotate({shape.rot || 0}deg)"
  style:padding="{shape.top*scale}px {shape.right*scale}px {shape.bottom*scale}px {shape.left*scale}px"
  style:background={shape.fill?`#${shape.fill}`:'var(--grid-bg)'}
  style:justify-content={shape.alignV==='center'?'center':shape.alignV==='bottom'?'flex-end':'flex-start'}>
  <div use:attach class="shape-text-input active" contenteditable="true" role="textbox" tabindex="0"
    aria-label={t('文字方塊編輯器')} aria-multiline="true" spellcheck="false" autocapitalize="off"
    style:text-align={shape.alignH==='center'?'center':shape.alignH==='right'?'right':'left'}
    oninput={input} onkeydown={key} onkeyup={announce} onpointerup={announce}
    oncompositionstart={()=>control.compositionStart()} oncompositionend={()=>{control.compositionEnd();announce();}}></div>
</div>

<style>
  .shape-text-editor {position:absolute;z-index:4;box-sizing:border-box;display:flex;flex-direction:column;border:1px solid var(--accent);overflow:hidden;box-shadow:0 1px 5px rgb(0 0 0 / .15);transform-origin:center;}
  .shape-text-input {width:100%;min-height:1em;max-height:100%;flex-shrink:0;overflow:auto;box-sizing:border-box;outline:none;cursor:text;white-space:pre-wrap;overflow-wrap:anywhere;line-height:1.2;-webkit-user-select:text;user-select:text;}
</style>
