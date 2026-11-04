<script lang="ts">
  import { untrack } from 'svelte';
  import { t } from '../i18n';
  import type { PivotInfoDTO, Pivot } from '../bridge/sheet';
  let { entries, initial, onapply, onclose }: { entries: PivotInfoDTO[]; initial: string; onapply: (pivot: Pivot) => Promise<string | null>; onclose: () => void } = $props();
  let selected = $state('');
  $effect.pre(() => { selected = initial; });
  let info = $derived(entries.find(e => e.pivot.Name === selected) ?? entries[0]);
  let draft = $state<Pivot>(JSON.parse(JSON.stringify(untrack(() => info.pivot))));
  let error = $state('');
  let busy = $state(false);
  let filterField = $state('');
  $effect.pre(() => { draft = JSON.parse(JSON.stringify(info.pivot)); error = info.error; filterField = ''; });
  const areas = [{key:'Rows',label:t('列欄位')}, {key:'Columns',label:t('欄欄位')}, {key:'Pages',label:t('報表篩選')}] as const;
  let used = $derived([...(draft.Rows ?? []), ...(draft.Columns ?? []), ...(draft.Pages ?? [])]);
  const functions = [{value:'sum',label:t('加總')},{value:'average',label:t('平均值')},{value:'count',label:t('筆數')}];
  function move(area: 'Rows' | 'Columns' | 'Pages', index: number, delta: number) {
    const list = [...(draft[area] ?? [])];
    [list[index],list[index+delta]] = [list[index+delta],list[index]];
    draft[area] = list;
  }
  function remove(area: 'Rows' | 'Columns' | 'Pages', index: number) {
    const field = (draft[area] ?? [])[index];
    draft[area] = (draft[area] ?? []).filter((_,i) => i !== index);
    delete draft.Filters?.[field];
    if (filterField === field) filterField = '';
  }
  function addValue(field: string) {
    if (!field) return;
    const captions = new Set((draft.Values ?? []).map(v=>v.Caption));
    let caption = t('加總') + ' - ' + field;
    while (captions.has(caption)) caption += ' 2';
    draft.Values = [...(draft.Values ?? []), { Field:field, Function:'sum', Caption:caption, Format:'' }];
  }
  function toggle(value: string, checked: boolean) {
    const items = info.items?.[filterField] ?? [];
    const selected = new Set(draft.Filters?.[filterField] ?? items);
    if (checked) selected.add(value); else selected.delete(value);
    draft.Filters = {...draft.Filters, [filterField]:[...selected]};
  }
  async function apply() {
    busy=true; error='';
    try { error = await onapply(draft) ?? ''; } catch (e) { error=String(e); }
    finally { busy=false; }
  }
</script>

<div class="dlg-backdrop">
  <div class="dlg pivot-dialog" role="dialog" aria-modal="true" aria-label={t('樞紐分析表設定')}>
    <h2>{t('樞紐分析表設定')}</h2>
    <label>{t('樞紐分析表')} <select aria-label={t('樞紐分析表')} bind:value={selected} disabled={busy}>{#each entries as entry}<option value={entry.pivot.Name}>{entry.pivot.Name}</option>{/each}</select></label>
    <label>{t('來源資料')} <input class="source" type="text" bind:value={draft.Source} disabled={busy} /></label>
    <p>{t('調整欄位或篩選後按「套用並重新整理」，將使用目前來源資料重新計算。')}</p>
    <div class="areas">
      {#each areas as area}
        <fieldset disabled={busy || !!info.error}>
          <legend>{area.label}</legend>
          {#each draft[area.key] ?? [] as field, i}
            <div class="field-row">
              <span>{field}</span>
              <button aria-label={t('上移')} disabled={i === 0} onclick={() => move(area.key,i,-1)}>↑</button>
              <button aria-label={t('下移')} disabled={i === (draft[area.key] ?? []).length-1} onclick={() => move(area.key,i,1)}>↓</button>
              <button aria-label={t('移除欄位')} onclick={() => remove(area.key,i)}>×</button>
            </div>
          {/each}
          <select aria-label={area.label} value="" onchange={(e) => { const value=e.currentTarget.value; if(value) draft[area.key]=[...(draft[area.key]??[]),value]; e.currentTarget.value=''; }}>
            <option value="">{t('新增欄位…')}</option>
            {#each (info.fields ?? []).filter(f=>!used.includes(f)) as field}<option value={field}>{field}</option>{/each}
          </select>
        </fieldset>
      {/each}
      <fieldset disabled={busy || !!info.error}>
        <legend>{t('值欄位')}</legend>
        {#each draft.Values ?? [] as value, i}
          <div class="value-row">
            <span>{value.Field}</span>
            <select aria-label={t('彙總方式')} value={value.Function} onchange={(e)=>{ const fn=e.currentTarget.value; if(functions.some(f=>value.Caption===f.label+' - '+value.Field)) value.Caption=(functions.find(f=>f.value===fn)?.label ?? fn)+' - '+value.Field; value.Function=fn; if(fn==='count') value.Format='0'; }}>{#each functions as fn}<option value={fn.value}>{fn.label}</option>{/each}</select>
            <button aria-label={t('上移')} disabled={i===0} onclick={() => { const values=[...(draft.Values ?? [])]; [values[i-1],values[i]]=[values[i],values[i-1]]; draft.Values=values; }}>↑</button>
            <button aria-label={t('移除欄位')} onclick={() => draft.Values=(draft.Values ?? []).filter((_,j)=>j!==i)}>×</button>
            <input type="text" aria-label={t('值欄位標題')} bind:value={value.Caption} />
          </div>
        {/each}
        <select aria-label={t('新增值欄位')} value="" onchange={(e)=>{addValue(e.currentTarget.value);e.currentTarget.value='';}}>
          <option value="">{t('新增欄位…')}</option>{#each info.fields as field}<option value={field}>{field}</option>{/each}
        </select>
      </fieldset>
    </div>
    <div class="totals">
      <label><input type="checkbox" bind:checked={draft.Subtotals} disabled={busy}/>{t('顯示小計')}</label>
      <label><input type="checkbox" bind:checked={draft.RowTotals} disabled={busy}/>{t('顯示橫向總計')}</label>
      <label><input type="checkbox" bind:checked={draft.ColTotals} disabled={busy}/>{t('顯示縱向總計')}</label>
    </div>
    <fieldset disabled={busy || !!info.error}>
      <legend>{t('欄位項目篩選')}</legend>
      <select aria-label={t('篩選欄位')} bind:value={filterField}><option value="">{t('選擇欄位…')}</option>{#each used as field}<option value={field}>{field}</option>{/each}</select>
      {#if filterField}
        <div class="filter-actions"><button onclick={()=>{delete draft.Filters?.[filterField];}}>{t('全選／清除篩選')}</button><button onclick={()=>draft.Filters={...draft.Filters,[filterField]:[]}}>{t('全部取消')}</button></div>
        <div class="items">{#each info.items?.[filterField] ?? [] as value}<label><input type="checkbox" checked={!draft.Filters?.[filterField] || (draft.Filters?.[filterField]?.includes(value) ?? false)} onchange={e=>toggle(value,e.currentTarget.checked)} />{value || t('（空白）')}</label>{/each}</div>
      {/if}
    </fieldset>
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="dlg-actions"><button disabled={busy} onclick={onclose}>{t('取消')}</button><button class="primary" disabled={busy || !!info.error || !draft.Values?.length} onclick={() => void apply()}>{busy ? t('處理中…') : t('套用並重新整理')}</button></div>
  </div>
</div>
<svelte:window onkeydown={(e)=>{if(e.key==='Escape'&&!busy){e.preventDefault();onclose();}}}/>
<style>
  .pivot-dialog { width:min(800px,94vw); max-height:90vh; overflow:auto; }
  .source { flex:1; }
  p { margin:0; color:var(--chrome-muted); }
  .areas { display:grid; grid-template-columns:1fr 1fr; gap:10px; }
  .field-row,.value-row,.totals,.filter-actions { display:flex; align-items:center; gap:6px; }
  .field-row span,.value-row span { flex:1; min-width:0; overflow-wrap:anywhere; }
  .field-row button,.value-row button { padding:2px 7px; }
  .value-row { flex-wrap:wrap; }
  .value-row input { width:100%; }
  .items { max-height:130px; overflow:auto; display:flex; flex-wrap:wrap; gap:5px 20px; }
  .error { color:var(--error-fg); }
</style>
