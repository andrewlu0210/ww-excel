<script lang="ts">
 import {t} from '../i18n';
 import type {ChartDTO,ChartDataDTO} from '../bridge/sheet';
 import {seriesColor} from '../charts/render';
 let {chart=$bindable(),data}:{chart:ChartDTO;data:ChartDataDTO|null}=$props();
 let selected=$state(0);
 $effect(()=>{if(selected!==0&&selected>=(chart.series?.length ?? 0))selected=0;});
 const series=$derived(chart.series?.[selected]);
 const kind=$derived(chart.type==='combo'?series?.type:chart.type);
 const pie=$derived(['pie','doughnut'].includes(chart.type));
 const markers=[{value:'auto',label:t('自動')},{value:'none',label:t('不顯示')},{value:'circle',label:t('圓形')},{value:'square',label:t('正方形')},{value:'diamond',label:t('菱形')},{value:'triangle',label:t('三角形')},{value:'x',label:'×'},{value:'plus',label:'＋'},{value:'star',label:t('星形')},{value:'dash',label:t('短線')},{value:'dot',label:t('圓點')}];
</script>
<fieldset class="series-editor">
 <legend>{t('數列外觀')}</legend>
 {#if !pie}
  <label>{t('編輯數列')}<select aria-label={t('編輯數列')} bind:value={selected}>{#each chart.series ?? [] as s,i}<option value={i}>{data?.series?.[i]?.name || s.name || t('數列%d',i+1)}</option>{/each}</select></label>
  {#if series}
   <div class="settings">
    <label>{t('數列色彩')}<input aria-label={t('數列色彩')} type="color" value={'#'+(series.color || seriesColor(selected).slice(1))} oninput={e=>series.color=e.currentTarget.value.slice(1)}/></label>
    <button type="button" onclick={()=>series.color=seriesColor(selected).slice(1)}>{t('預設色彩')}</button>
    {#if kind==='line'}<label>{t('折線粗細')}<input aria-label={t('折線粗細')} type="number" min="0.25" max="30" step="0.25" value={Number((series.lineWidth || 2.25).toFixed(3))} onchange={e=>series.lineWidth=e.currentTarget.valueAsNumber}/></label>{/if}
    {#if kind==='line'||kind==='scatter'}<label>{t('資料點標記')}<select aria-label={t('資料點標記')} value={series.marker || 'auto'} onchange={e=>series.marker=e.currentTarget.value}>{#each markers as m}<option value={m.value}>{m.label}</option>{/each}{#if series.marker&&!markers.some(m=>m.value===series.marker)}<option value={series.marker}>{t('保留原有標記')}</option>{/if}</select></label>{/if}
   </div>
  {/if}
 {/if}
 {#if ['column','bar','combo'].includes(chart.type)}
  <label>{t('柱距（%）')}<input aria-label={t('柱距（%）')} type="number" min="0" max="500" step="1" value={chart.gapWidth ?? ''} placeholder={t('自動')} onchange={e=>chart.gapWidth=e.currentTarget.value===''?undefined:e.currentTarget.valueAsNumber}/></label>
 {/if}
 {#if chart.type==='doughnut'}<label>{t('環圈內徑（%）')}<input aria-label={t('環圈內徑（%）')} type="number" min="10" max="90" step="1" value={chart.holeSize || 50} onchange={e=>chart.holeSize=e.currentTarget.valueAsNumber}/></label>{/if}
 {#if pie}<p>{t('各區塊沿用原有配色與資料標籤設定。')}</p>{/if}
</fieldset>
<style>
 .series-editor{min-width:0;gap:10px;}
 label{display:flex;align-items:center;justify-content:space-between;gap:8px;}
 .settings{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px 18px;}
 input[type='number']{width:100px;color:var(--chrome-text);background:var(--input-bg);border:1px solid var(--chrome-border);border-radius:4px;padding:4px;font:inherit;}
 input[type='color']{width:36px;height:25px;padding:1px;}
 p{margin:0;color:var(--chrome-muted);font-size:12px;}
</style>
