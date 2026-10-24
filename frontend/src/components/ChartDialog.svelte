<script lang="ts">
  // 插入 › 圖表… and editing a chart (double-click): type, titles,
  // legend and the series references, with a live preview.
  import { t } from '../i18n';
  import { onMount } from 'svelte';
  import type { ChartDataDTO, ChartDTO } from '../bridge/sheet';
  import { drawChart, LIGHT_THEME } from '../charts/render';
  import { toChartData } from './ObjectLayer.svelte';

  interface Props {
    editing: boolean;
    initial: ChartDTO;
    // Insert mode: rebuild the series from the selection for a type.
    suggest?: (type: string) => Promise<ChartDTO | null>;
    preview: (chart: ChartDTO) => Promise<ChartDataDTO>;
    onapply: (chart: ChartDTO) => Promise<string | null>;
    onclose: () => void;
  }

  let { editing, initial, suggest, preview, onapply, onclose }: Props = $props();

  let chart = $state<ChartDTO>({ type: 'column', title: '', series: [], legend: 'right', xTitle: '', yTitle: '', stacked: false });
  let error = $state<string | null>(null);
  let canvas: HTMLCanvasElement;
  let data = $state<ChartDataDTO | null>(null);

  const types = [
    { value: 'column', label: t('直條圖') },
    { value: 'bar', label: t('橫條圖') },
    { value: 'line', label: t('折線圖') },
    { value: 'area', label: t('區域圖') },
    { value: 'pie', label: t('圓形圖') },
    { value: 'scatter', label: t('XY 散佈圖') },
  ];

  onMount(() => {
    chart = structuredClone($state.snapshot(initial)) as ChartDTO;
    chart.series = chart.series ?? [];
  });

  async function changeType(type: string): Promise<void> {
    if (!editing && suggest) {
      const next = await suggest(type);
      if (next) {
        chart = { ...next, title: chart.title || next.title, legend: chart.legend, series: next.series ?? [] };
        return;
      }
    }
    chart.type = type;
  }

  // Redraw the preview when anything changes.
  let previewToken = 0;
  $effect(() => {
    const snapshot = $state.snapshot(chart) as ChartDTO;
    const token = ++previewToken;
    void preview(snapshot).then((d) => {
      if (token === previewToken) data = d;
    });
  });
  $effect(() => {
    const d = data;
    const snapshot = $state.snapshot(chart) as ChartDTO;
    const ctx = canvas?.getContext('2d');
    if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = 360 * dpr;
    canvas.height = 216 * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    drawChart(ctx, 360, 216, snapshot, toChartData(d ?? undefined), LIGHT_THEME);
  });

  async function apply(): Promise<void> {
    error = await onapply($state.snapshot(chart) as ChartDTO);
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label={t('圖表')} style="width: 720px">
    <h2>{editing ? t('編輯圖表') : t('插入圖表')}</h2>
    <div class="types" role="radiogroup" aria-label={t('圖表類型')}>
      {#each types as t (t.value)}
        <button role="radio" aria-checked={chart.type === t.value} class:on={chart.type === t.value} onclick={() => void changeType(t.value)}>{t.label}</button>
      {/each}
    </div>
    <div class="body">
      <div class="fields">
        <label>{t('標題')} <input type="text" bind:value={chart.title} /></label>
        <label>
          {t('圖例')}
          <select bind:value={chart.legend}>
            <option value="right">{t('右')}</option>
            <option value="bottom">{t('下')}</option>
            <option value="top">{t('上')}</option>
            <option value="none">{t('不顯示')}</option>
          </select>
        </label>
        {#if chart.type !== 'pie'}
          <label>{t('橫軸標題')} <input type="text" bind:value={chart.xTitle} /></label>
          <label>{t('縱軸標題')} <input type="text" bind:value={chart.yTitle} /></label>
        {/if}
        {#if ['column', 'bar', 'area'].includes(chart.type)}
          <label class="check"><input type="checkbox" bind:checked={chart.stacked} /> {t('堆疊')}</label>
        {/if}
      </div>
      <canvas bind:this={canvas} style="width: 360px; height: 216px" aria-label={t('預覽')}></canvas>
    </div>
    <div class="series">
      <div class="head"><span>{t('數列名稱')}</span><span>{chart.type === 'scatter' ? t('X 值') : t('類別')}</span><span>{t('值')}</span><span></span></div>
      {#each chart.series ?? [] as s, i (i)}
        <div class="row">
          <input type="text" bind:value={s.name} />
          <input type="text" bind:value={s.categories} />
          <input type="text" bind:value={s.values} placeholder={t('=工作表1!$B$2:$B$10')} />
          <button aria-label={t('移除數列')} onclick={() => chart.series?.splice(i, 1)}>{t('－')}</button>
        </div>
      {/each}
      <div>
        <button
          onclick={() => (chart.series = [...(chart.series ?? []), { name: '', categories: chart.series?.[0]?.categories ?? '', values: '' }])}
          >{t('＋ 新增數列')}</button
        >
      </div>
    </div>
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="dlg-actions">
      <button onclick={onclose}>{t('取消')}</button>
      <button class="primary" onclick={() => void apply()}>{t('確定')}</button>
    </div>
  </div>
</div>

<style>
  .types {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  .types button.on {
    background: var(--popup-active);
    border-color: var(--accent);
  }
  .body {
    display: flex;
    gap: 16px;
  }
  .fields {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  label {
    justify-content: space-between;
  }
  label.check {
    justify-content: flex-start;
  }
  label input[type='text'],
  label select {
    width: 200px;
  }
  canvas {
    border: 1px solid var(--chrome-border);
  }
  .series {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
  }
  .head,
  .row {
    display: grid;
    grid-template-columns: 1fr 1.4fr 1.4fr 32px;
    gap: 6px;
  }
  .head {
    color: var(--chrome-muted);
  }
  .row input {
    font-family: ui-monospace, Menlo, monospace;
    font-size: 12px;
  }
  .error {
    color: var(--error-fg);
  }
</style>
