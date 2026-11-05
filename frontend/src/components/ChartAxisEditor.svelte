<script lang="ts">
  import {t} from '../i18n';
  import type {ChartDTO, ChartAxis} from '../bridge/sheet';
  let {chart=$bindable()}:{chart:ChartDTO}=$props();
  type Key='valueAxis'|'categoryAxis'|'secondaryAxis';
  let selected=$state<Key>('valueAxis');
  const choices=$derived([
    {key:'valueAxis' as Key,name:chart.type==='bar'?t('水平數值軸'):chart.type==='combo'?t('左側數值軸'):t('垂直數值軸')},
    {key:'categoryAxis' as Key,name:chart.type==='scatter'?t('水平數值軸'):chart.type==='bar'?t('垂直類別軸'):t('水平類別軸')},
    ...(chart.type==='combo'&&chart.series?.some(s=>s.secondary)?[{key:'secondaryAxis' as Key,name:t('右側數值軸')}]:[]),
  ]);
  const key=$derived(choices.some(c=>c.key===selected)?selected:'valueAxis');
  const axis=$derived(chart[key]);
  const numeric=$derived(key!=='categoryAxis'||chart.type==='scatter');
  function patch(values:Partial<ChartAxis>) {
    const defaults:ChartAxis={format:'General',majorUnit:0,lineColor:'',lineHidden:false,hidden:false,majorTick:'none',gridLines:key==='valueAxis'||chart.type==='scatter',hideLabels:false};
    chart[key]={...defaults,...chart[key],...values};
  }
  function bound(which:'minimum'|'maximum',value:string) {patch({[which]:value.trim()===''?undefined:Number(value)});}
</script>
{#if !['pie','doughnut'].includes(chart.type)}
<fieldset class="axis-editor">
  <legend>{t('座標軸設定')}</legend>
  <label>{t('編輯座標軸')}<select aria-label={t('編輯座標軸')} bind:value={selected}>{#each choices as c}<option value={c.key}>{c.name}</option>{/each}</select></label>
  <div class="settings">
    {#if numeric}
      <label>{t('最小值')}<input aria-label={t('座標軸最小值')} type="number" step="any" placeholder={t('自動')} value={axis?.minimum ?? ''} oninput={e=>{if(!e.currentTarget.validity.badInput)bound('minimum',e.currentTarget.value);}}/></label>
      <label>{t('最大值')}<input aria-label={t('座標軸最大值')} type="number" step="any" placeholder={t('自動')} value={axis?.maximum ?? ''} oninput={e=>{if(!e.currentTarget.validity.badInput)bound('maximum',e.currentTarget.value);}}/></label>
      <label>{t('主要刻度間距')}<input aria-label={t('主要刻度間距')} type="number" min="0" step="any" placeholder={t('自動')} value={axis?.majorUnit || ''} oninput={e=>{if(!e.currentTarget.validity.badInput)patch({majorUnit:e.currentTarget.value===''?0:e.currentTarget.valueAsNumber});}}/></label>
      <button type="button" onclick={()=>patch({minimum:undefined,maximum:undefined,majorUnit:0})}>{t('恢復自動範圍')}</button>
    {/if}
    <label>{t('數值格式')}<input aria-label={t('座標軸數值格式')} type="text" value={axis?.format || 'General'} oninput={e=>patch({format:e.currentTarget.value})}/></label>
    <label>{t('字型')}<input aria-label={t('座標軸字型')} type="text" value={axis?.fontName ?? ''} placeholder={t('預設')} oninput={e=>patch({fontName:e.currentTarget.value})}/></label>
    <label>{t('字型大小')}<input aria-label={t('座標軸字型大小')} type="number" min="1" max="409" step="1" value={axis?.fontSize || 9} onchange={e=>patch({fontSize:e.currentTarget.valueAsNumber})}/></label>
    <label>{t('軸線色彩')}<input aria-label={t('軸線色彩')} type="color" value={'#'+(axis?.lineColor || '595959')} oninput={e=>patch({lineColor:e.currentTarget.value.slice(1),lineHidden:false})}/></label>
    <label>{t('刻度線')}<select aria-label={t('座標軸刻度線')} value={axis?.majorTick || 'none'} onchange={e=>patch({majorTick:e.currentTarget.value})}><option value="none">{t('不顯示')}</option><option value="in">{t('內側')}</option><option value="out">{t('外側')}</option><option value="cross">{t('交叉')}</option></select></label>
  </div>
  <div class="checks">
    <label><input aria-label={t('顯示座標軸')} type="checkbox" checked={!axis?.hidden} onchange={e=>patch({hidden:!e.currentTarget.checked})}/>{t('顯示座標軸')}</label>
    <label><input aria-label={t('顯示刻度標籤')} type="checkbox" checked={!axis?.hideLabels} onchange={e=>patch({hideLabels:!e.currentTarget.checked})}/>{t('顯示刻度標籤')}</label>
    <label><input aria-label={t('顯示軸線')} type="checkbox" checked={!axis?.lineHidden} onchange={e=>patch({lineHidden:!e.currentTarget.checked})}/>{t('顯示軸線')}</label>
    <label><input aria-label={t('主要格線')} type="checkbox" checked={axis?.gridLines ?? (key==='valueAxis'||chart.type==='scatter')} onchange={e=>patch({gridLines:e.currentTarget.checked})}/>{t('主要格線')}</label>
  </div>
</fieldset>
{/if}
<style>
  .axis-editor{min-width:0;gap:10px;}
  label{display:flex;align-items:center;justify-content:space-between;gap:8px;}
  .settings{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px 18px;}
  .checks{display:flex;flex-wrap:wrap;gap:12px;}
  input:not([type='checkbox']):not([type='color']),select{width:150px;min-width:0;}
  input[type='number']{color:var(--chrome-text);background:var(--input-bg);border:1px solid var(--chrome-border);border-radius:4px;padding:4px;font:inherit;}
  input[type='color']{width:36px;height:25px;padding:1px;}
</style>
