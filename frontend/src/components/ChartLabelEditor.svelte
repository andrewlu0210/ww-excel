<script lang="ts">
  import { t } from '../i18n';
  import type { ChartDTO, ChartDataDTO, ChartLabelPoint } from '../bridge/sheet';
  let { chart = $bindable(), data }: { chart:ChartDTO; data:ChartDataDTO|null } = $props();
  let selected = $state(0);
  let pointIndex = $state(-1);
  const series = $derived(chart.series?.[selected]);
  const labels = $derived(series?.labels);
  const enabled=$derived(!!labels&&!labels.disabled&&!!(labels.showValue||labels.showPercent||labels.showCategory||labels.showSeries||Object.values(labels.points ?? {}).some(p=>p?.customText!=null||p?.reference)));
  const pointKey=$derived(`${pointIndex}` as `${number}`);
  const point = $derived(labels?.points?.[pointKey]);
  const points = $derived(Array.from({length:Math.min(10000,data?.series?.[selected]?.values?.length ?? 0)},(_,i)=>i));
  const positions = [
    {value:'',label:t('自動')},{value:'ctr',label:t('置中')},{value:'inEnd',label:t('內側結尾')},
    {value:'inBase',label:t('內側基底')},{value:'outEnd',label:t('外側結尾')},{value:'t',label:t('上方')},
    {value:'b',label:t('下方')},{value:'l',label:t('左方')},{value:'r',label:t('右方')},{value:'bestFit',label:t('最適位置')},
  ];
  function enable(checked:boolean) {
    if(!series)return;
    if(!series.labels)series.labels={format:'General',showValue:true,showPercent:false,fontSize:11,color:'',points:{}};
    series.labels.disabled=!checked;
    if(checked&&!enabled)series.labels.showValue=true;
  }
  function patchPoint(patch:Partial<ChartLabelPoint>) {
    if(!labels||pointIndex<0)return;
    labels.points={...labels.points,[pointKey]:{hidden:false,center:false,...labels.points?.[pointKey],...patch}};
  }
  function manual(checked:boolean) {patchPoint({center:false,layout:checked?{x:.5,y:.5,w:0,h:0,xMode:'edge',yMode:'edge'}:undefined});}
</script>

<fieldset class="label-editor">
  <legend>{t('資料標籤')}</legend>
  <div class="line">
    <label>{t('標籤數列')} <select aria-label={t('標籤數列')} bind:value={selected} onchange={()=>pointIndex=-1}>{#each chart.series ?? [] as s,i}<option value={i}>{data?.series?.[i]?.name || s.name || t('數列%d',i+1)}</option>{/each}</select></label>
    <label><input type="checkbox" aria-label={t('啟用資料標籤')} checked={enabled} disabled={!series} onchange={e=>enable(e.currentTarget.checked)}/>{t('啟用資料標籤')}</label>
  </div>
  {#if labels && enabled}
    <div class="line checks">
      <label><input type="checkbox" bind:checked={labels.showValue}/>{t('數值')}</label>
      <label><input type="checkbox" bind:checked={labels.showPercent}/>{t('百分比')}</label>
      <label><input type="checkbox" bind:checked={labels.showCategory}/>{t('類別名稱')}</label>
      <label><input type="checkbox" bind:checked={labels.showSeries}/>{t('數列名稱')}</label>
      <label><input type="checkbox" bind:checked={labels.leaderLines}/>{t('顯示引導線')}</label>
    </div>
    <div class="settings">
      <label>{t('標籤位置')}<select aria-label={t('標籤位置')} value={labels.position ?? ''} onchange={e=>labels.position=e.currentTarget.value}>{#each positions as p}<option value={p.value}>{p.label}</option>{/each}</select></label>
      <label>{t('分隔符號')}<select aria-label={t('分隔符號')} value={labels.separator || '\n'} onchange={e=>labels.separator=e.currentTarget.value}><option value={'\n'}>{t('換行')}</option><option value=", ">{t('逗號')}</option><option value=" / ">{t('斜線')}</option><option value=" ">{t('空格')}</option></select></label>
      <label>{t('數值格式')}<input aria-label={t('標籤數值格式')} type="text" bind:value={labels.format}/></label>
      {#if labels.showPercent}<label>{t('百分比格式')}<input aria-label={t('標籤百分比格式')} type="text" value={labels.percentFormat || '0%'} onchange={e=>labels.percentFormat=e.currentTarget.value}/></label>{/if}
      <label>{t('字型')}<input aria-label={t('標籤字型')} type="text" value={labels.fontName ?? ''} placeholder={t('預設')} onchange={e=>labels.fontName=e.currentTarget.value}/></label>
      <label>{t('字型大小')}<input aria-label={t('標籤字型大小')} type="number" min="1" max="409" step="1" value={labels.fontSize || 11} onchange={e=>labels.fontSize=e.currentTarget.valueAsNumber}/></label>
      <label>{t('旋轉角度')}<input aria-label={t('標籤旋轉角度')} type="number" min="-90" max="90" value={labels.rotation || 0} onchange={e=>labels.rotation=e.currentTarget.valueAsNumber}/></label>
      <label>{t('文字色彩')}<input aria-label={t('標籤文字色彩')} type="color" value={'#'+(labels.color || '404040')} oninput={e=>labels.color=e.currentTarget.value.slice(1)}/></label>
      <div class="line"><label><input type="checkbox" bind:checked={labels.bold}/>{t('粗體')}</label><label><input type="checkbox" bind:checked={labels.italic}/>{t('斜體')}</label></div>
      <div class="line"><label><input type="checkbox" checked={!!labels.fill} onchange={e=>labels.fill=e.currentTarget.checked?'FFFFFF':''}/>{t('標籤底色')}</label>{#if labels.fill}<input aria-label={t('標籤底色')} type="color" value={'#'+labels.fill} oninput={e=>labels.fill=e.currentTarget.value.slice(1)}/>{/if}</div>
      <div class="line"><label><input type="checkbox" checked={!!labels.border} onchange={e=>labels.border=e.currentTarget.checked?'808080':''}/>{t('標籤外框')}</label>{#if labels.border}<input aria-label={t('標籤外框')} type="color" value={'#'+labels.border} oninput={e=>labels.border=e.currentTarget.value.slice(1)}/>{/if}</div>
    </div>
    <div class="point-head"><label>{t('個別資料標籤')}<select aria-label={t('個別資料標籤')} bind:value={pointIndex}><option value={-1}>{t('選擇資料點…')}</option>{#each points as i}<option value={i}>{i+1} · {data?.categories?.[i] ?? ''}</option>{/each}</select></label></div>
    {#if pointIndex>=0}
      <div class="line checks">
        <label><input aria-label={t('隱藏此標籤')} type="checkbox" checked={point?.hidden ?? false} onchange={e=>patchPoint({hidden:e.currentTarget.checked})}/>{t('隱藏此標籤')}</label>
        <label><input aria-label={t('自訂標籤文字')} type="checkbox" checked={point?.customText!=null || !!point?.reference} onchange={e=>patchPoint({customText:e.currentTarget.checked?(data?.series?.[selected]?.labels?.[pointIndex] ?? ''):undefined,reference:''})}/>{t('自訂標籤文字')}</label>
        <label><input aria-label={t('自訂標籤位置')} type="checkbox" checked={!!point?.layout} onchange={e=>manual(e.currentTarget.checked)}/>{t('自訂標籤位置')}</label>
        {#if chart.type==='doughnut'}<label><input type="checkbox" checked={point?.center ?? false} onchange={e=>patchPoint({center:e.currentTarget.checked,layout:undefined})}/>{t('環圈中央')}</label>{/if}
        <button type="button" onclick={()=>{if(labels.points)delete labels.points[pointKey];}}>{t('重設此標籤')}</button>
      </div>
      {#if point?.customText!=null || point?.reference}
        <textarea aria-label={t('標籤文字')} maxlength="8192" rows="2" value={point.customText ?? data?.series?.[selected]?.labels?.[pointIndex] ?? ''} oninput={e=>patchPoint({customText:e.currentTarget.value,reference:''})}></textarea>
      {/if}
      <div class="settings">
        <label>{t('標籤位置')}<select aria-label={t('個別標籤位置')} value={point?.position ?? ''} onchange={e=>patchPoint({position:e.currentTarget.value,center:false})}>{#each positions as p}<option value={p.value}>{p.label}</option>{/each}</select></label>
        <label>{t('字型大小')}<input aria-label={t('個別標籤字型大小')} type="number" min="1" max="409" value={point?.fontSize || labels.fontSize || 11} onchange={e=>patchPoint({fontSize:e.currentTarget.valueAsNumber})}/></label>
        <label>{t('文字色彩')}<input aria-label={t('個別標籤文字色彩')} type="color" value={'#'+(point?.color || labels.color || '404040')} oninput={e=>patchPoint({color:e.currentTarget.value.slice(1)})}/></label>
        <div class="line"><label><input type="checkbox" checked={point?.bold ?? labels.bold ?? false} onchange={e=>patchPoint({bold:e.currentTarget.checked})}/>{t('粗體')}</label><label><input type="checkbox" checked={point?.italic ?? labels.italic ?? false} onchange={e=>patchPoint({italic:e.currentTarget.checked})}/>{t('斜體')}</label></div>
        {#if point?.layout}
          <label>{t('水平位置（%）')}<input aria-label={t('標籤水平位置')} type="number" min="0" max="100" value={Math.round(point.layout.x*100)} onchange={e=>patchPoint({layout:{...point!.layout!,x:e.currentTarget.valueAsNumber/100,xMode:'edge'}})}/></label>
          <label>{t('垂直位置（%）')}<input aria-label={t('標籤垂直位置')} type="number" min="0" max="100" value={Math.round(point.layout.y*100)} onchange={e=>patchPoint({layout:{...point!.layout!,y:e.currentTarget.valueAsNumber/100,yMode:'edge'}})}/></label>
        {/if}
      </div>
    {/if}
  {/if}
</fieldset>
<style>
  .label-editor { gap:10px; min-width:0; }
  .line { display:flex; align-items:center; gap:12px; flex-wrap:wrap; }
  .checks label { white-space:nowrap; }
  .settings { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:8px 18px; }
  label { justify-content:space-between; }
  .settings input[type='text'],.settings select { width:160px; min-width:0; }
  input[type='number'] { width:75px; color:var(--chrome-text); background:var(--input-bg); border:1px solid var(--chrome-border); border-radius:4px; padding:4px; font:inherit; }
  input[type='color'] { width:36px; height:25px; padding:1px; }
  textarea { width:100%; resize:vertical; color:var(--chrome-text); background:var(--input-bg); border:1px solid var(--chrome-border); font:inherit; user-select:text; }
  .point-head { border-top:1px solid var(--chrome-border); padding-top:8px; }
</style>
