<script lang="ts">
 import type { FillSpec } from '../../bindings/ww-excel/internal/model/models';
 import { FILL_PATTERNS, fillStops } from '../grid/cellFills';
 import { t } from '../i18n';
 let {value, mixed=false, onchange}: {value: FillSpec; mixed?: boolean; onchange:(spec:FillSpec)=>void}=$props();
 const labels=['深灰','中灰','淺灰','12.5% 灰','6.25% 灰','深水平線','深垂直線','深向下斜線','深向上斜線','深格線','深斜格線','淺水平線','淺垂直線','淺向下斜線','淺向上斜線','淺格線','淺斜格線'];
 const stops=$derived(fillStops(value));
 function kind(kind:string) {
  if(kind==='pattern') onchange({kind,pattern:'darkDown',foreground:'000000',background:'FFFFFF'});
  else if(kind) onchange({kind,degree:0,left:.5,right:.5,top:.5,bottom:.5,stops:JSON.stringify([{position:0,color:'FFFFFF'},{position:1,color:'0070C0'}])});
  else onchange({});
 }
 function stop(index:number,property:'position'|'color',v:number|string) {onchange({...value,stops:JSON.stringify(stops.map((s,i)=>i===index?{...s,[property]:v}:s))});}
 function addStop() {if(stops.length<16) onchange({...value,stops:JSON.stringify([...stops,{position:.5,color:'FFFFFF'}].sort((a,b)=>a.position-b.position))});}
</script>
<label>{t('填滿類型')}<select aria-label={t('填滿類型')} value={mixed?'__mixed__':value.kind || ''} onchange={e=>kind(e.currentTarget.value)}>
 {#if mixed}<option value="__mixed__" disabled>{t('混合')}</option>{/if}
 <option value="">{t('單色／無填滿')}</option><option value="pattern">{t('圖樣填滿')}</option><option value="linear">{t('線性漸層')}</option><option value="path">{t('矩形路徑漸層')}</option>
</select></label>
{#if value.kind==='pattern'}
 <label>{t('圖樣')}<select aria-label={t('圖樣')} value={value.pattern} onchange={e=>onchange({...value,pattern:e.currentTarget.value})}>{#each FILL_PATTERNS as p,i}<option value={p}>{t(labels[i])}</option>{/each}</select></label>
 <label>{t('前景色')}<input aria-label={t('前景色')} type="color" value={`#${value.foreground || '000000'}`} oninput={e=>onchange({...value,foreground:e.currentTarget.value.slice(1).toUpperCase()})}/></label>
 <label>{t('背景色')}<input aria-label={t('背景色')} type="color" value={`#${value.background || 'FFFFFF'}`} oninput={e=>onchange({...value,background:e.currentTarget.value.slice(1).toUpperCase()})}/></label>
{:else if value.kind==='linear' || value.kind==='path'}
 {#if value.kind==='linear'}<label>{t('角度')}<input aria-label={t('角度')} type="number" min="0" max="360" step="1" value={value.degree || 0} oninput={e=>onchange({...value,degree:+e.currentTarget.value})}/></label>
 {:else}<p>{t('漸層起始矩形（0 至 1）')}</p><div class="extent">{#each ['left','right','top','bottom'] as key}<label>{t(({left:'左',right:'右',top:'上',bottom:'下'} as Record<string,string>)[key])}<input aria-label={`gradient-${key}`} type="number" min="0" max="1" step="0.05" value={value[key as 'left'] ?? .5} oninput={e=>onchange({...value,[key]:+e.currentTarget.value})}/></label>{/each}</div>{/if}
 <p>{t('漸層色彩節點')}</p>
 {#each stops as s,i}<div class="stop"><label>{t('位置')}<input aria-label={`${t('位置')} ${i+1}`} type="number" min="0" max="1" step="0.05" value={s.position} oninput={e=>stop(i,'position',+e.currentTarget.value)}/></label><label>{t('色彩')}<input aria-label={`${t('色彩')} ${i+1}`} type="color" value={`#${s.color}`} oninput={e=>stop(i,'color',e.currentTarget.value.slice(1).toUpperCase())}/></label><button type="button" disabled={stops.length<=2} onclick={()=>onchange({...value,stops:JSON.stringify(stops.filter((_,j)=>j!==i))})}>{t('移除')}</button></div>{/each}
 <button type="button" disabled={stops.length>=16} onclick={addStop}>{t('新增色彩節點')}</button>
{/if}
<style>
 label {display:flex;gap:8px;align-items:center;flex-wrap:wrap;} select,input,button {font:inherit;color:inherit;background:var(--input-bg);border:1px solid var(--chrome-border);border-radius:4px;padding:5px;} input[type=number]{width:80px;}input[type=color]{width:48px;height:32px;padding:2px;}.stop,.extent{display:flex;gap:8px;align-items:center;flex-wrap:wrap;}p{margin:4px 0;}button{cursor:pointer;}button:disabled{opacity:.45;}
</style>
