<script lang="ts">
  import { untrack } from 'svelte';
  import { t } from '../i18n';
  import type { FormatPatch, StyleDTO } from '../bridge/sheet';
  import type { Range } from '../grid/selection';
  import { BORDER_WIDTH, BORDER_DASH, slantedBorderSegments } from '../grid/borders';
  import { BORDER_EDGES, borderPresetDraft, toggleBorderEdge, borderLineForPreview, previewBorderBrush, type BorderEdge, type BorderEdits } from './borderDraft';
  interface Props {
    value: BorderEdits;
    lineStyle: string;
    color: string;
    original: StyleDTO | null;
    originalEdges?: FormatPatch['borders'];
    mixedEdges?: string[];
    selection: Range[];
    merges: Range[];
    onclear?: () => void;
    onstylechange?: (style: string) => void;
    onleave?: () => void;
    resetKey?: number;
    styleMixed?: boolean;
  }
  let { value = $bindable(), lineStyle = $bindable(), color, original, originalEdges, mixedEdges = [], selection, merges, onclear, onstylechange, onleave, resetKey = 0, styleMixed = false }: Props = $props();
  const labels: Record<BorderEdge, string> = { top: t('上框線'), bottom: t('下框線'), left: t('左框線'), right: t('右框線'), insideH: t('內部水平框線'), insideV: t('內部垂直框線') };
  const styles = [
    ['thin', t('細實線')], ['medium', t('中實線')], ['thick', t('粗實線')], ['double', t('雙線')],
    ['hair', t('髮絲線')], ['dotted', t('點線')], ['dashed', t('虛線')], ['mediumDashed', t('中虛線')],
    ['dashDot', t('點劃線')], ['mediumDashDot', t('中點劃線')], ['dashDotDot', t('雙點劃線')], ['mediumDashDotDot', t('中雙點劃線')], ['slantDashDot', t('斜點劃線')],
  ];
  let brush = $derived({ style: lineStyle, color });
  let targets = $state<BorderEdge[]>([]);
  let baseline = $state<BorderEdits | null>(null);
  let lastBrush = untrack(() => ({style:lineStyle,color}));
  let lastReset = untrack(() => resetKey);
  $effect(() => {
    const next = {style:lineStyle,color}, reset=resetKey;
    untrack(() => {
      if (reset!==lastReset) {targets=[];baseline=null;lastReset=reset;}
      else if (next.style!==lastBrush.style || next.color!==lastBrush.color) {
        const preview=previewBorderBrush(value,targets,baseline,next);
        value=preview.value;baseline=preview.baseline;
      }
      lastBrush=next;
    });
  });
  function startAction() { if(baseline) value=baseline; baseline=null; }
  function toggle(edge: BorderEdge) {
    startAction();value=toggleBorderEdge(value,edge,brush);targets=[edge];
  }
  function leave() {value={};baseline=null;targets=[];onleave?.();}

  function enabled(edge: BorderEdge): boolean {
    if (!edge.startsWith('inside')) return true;
    return selection.some(r => {
      const singleMerge = merges.some(m => m.r0 <= r.r0 && m.c0 <= r.c0 && m.r1 >= r.r1 && m.c1 >= r.c1);
      return !singleMerge && (edge === 'insideH' ? r.r1 > r.r0 : r.c1 > r.c0);
    });
  }
  function preset(name: 'none' | 'outside' | 'inside' | 'all'): void {
    startAction();
    value = borderPresetDraft(value, name, brush);
    targets = BORDER_EDGES.filter(edge => enabled(edge) && (name === 'outside' ? !edge.startsWith('inside') : name === 'inside' ? edge.startsWith('inside') : true));
    if (name === 'none') onclear?.();
    for (const edge of ['insideH', 'insideV'] as const) if (!enabled(edge)) delete value[edge];
  }
  function edgeState(edge: BorderEdge): string { return value[edge] === undefined ? (mixedEdges.includes(edge) ? t('混合') : t('不修改')) : value[edge]?.style ? t('套用') : t('清除'); }
  function line(edge: BorderEdge) { return borderLineForPreview(value, edge, mixedEdges.includes(edge) ? undefined : originalEdges?.[edge] ?? (edge === 'insideH' || edge === 'insideV' ? undefined : original?.[edge])); }
  const points: Record<BorderEdge, [number, number, number, number]> = { top: [20, 20, 180, 20], bottom: [20, 120, 180, 120], left: [20, 20, 20, 120], right: [180, 20, 180, 120], insideH: [20, 70, 180, 70], insideV: [100, 20, 100, 120] };
</script>

<label>{t('線型')}<select value={styleMixed ? '__mixed__' : lineStyle} onchange={e=>{lineStyle=e.currentTarget.value;onstylechange?.(lineStyle);}}>{#if styleMixed}<option value="__mixed__" disabled>{t('混合')}</option>{/if}{#each styles as [code, label]}<option value={code}>{label}</option>{/each}</select></label>
<div class="presets" aria-label={t('框線預設')}>
  <button type="button" onclick={() => preset('none')}>{t('無框線')}</button>
  <button type="button" onclick={() => preset('outside')}>{t('外框線')}</button>
  <button type="button" onclick={() => preset('inside')} disabled={!enabled('insideH') && !enabled('insideV')}>{t('內框線')}</button>
  <button type="button" onclick={() => preset('all')}>{t('所有框線')}</button>
  <button type="button" onclick={leave}>{t('不修改')}</button>
</div>
<div class="border-layout">
  <svg class="border-preview" viewBox="0 0 200 140" role="group" aria-label={t('框線預覽')}>
    {#each BORDER_EDGES as edge}
      {@const b = line(edge)}
      {@const p = points[edge]}
      {@const colorValue = b.style ? (b.color ? `#${b.color}` : 'var(--chrome-text)') : 'var(--chrome-muted)'}
      {#if enabled(edge)}
        <g role="button" tabindex="0" aria-label={t('切換%s', labels[edge])} aria-pressed={value[edge] === undefined ? 'mixed' : !!value[edge]?.style}
          onclick={() => toggle(edge)}
          onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggle(edge); } }}>
        <line x1={p[0]} y1={p[1]} x2={p[2]} y2={p[3]} stroke="transparent" stroke-width="18" />
        {#if b.style === 'slantDashDot'}
          <path d={slantedBorderSegments(...p).map(([ax, ay, bx, by]) => `M${ax},${ay}L${bx},${by}`).join(' ')} stroke={colorValue} fill="none" stroke-width="1" pointer-events="none" />
        {:else}
        {#each b.style === 'double' ? [-1.5, 1.5] : [0] as offset}
          <line x1={p[0] + (p[0] === p[2] ? offset : 0)} y1={p[1] + (p[1] === p[3] ? offset : 0)}
            x2={p[2] + (p[0] === p[2] ? offset : 0)} y2={p[3] + (p[1] === p[3] ? offset : 0)}
            stroke={colorValue} stroke-width={b.style === 'double' ? 1 : BORDER_WIDTH[b.style ?? ''] ?? 1}
            stroke-dasharray={b.style ? (BORDER_DASH[b.style] ?? []).join(' ') : '2 4'} opacity={b.style ? 1 : 0.4} pointer-events="none" />
        {/each}
        {/if}
        </g>
      {/if}
    {/each}
  </svg>
  <div class="edges">
    {#each BORDER_EDGES as edge}
      <button type="button" class:edited={value[edge] !== undefined} data-edge={edge} disabled={!enabled(edge)}
        aria-label={labels[edge]} title={`${labels[edge]}：${edgeState(edge)}`} onclick={() => toggle(edge)}>
        {labels[edge]}<span>{edgeState(edge)}</span>
      </button>
    {/each}
  </div>
</div>
<p>{t('按一下套用線型，再按清除，第三次保留原設定。未修改的邊線保持原狀。')}</p>

<style>
  label { display:flex; flex-direction:column; gap:5px; }
  select { padding:5px 8px; font:inherit; color:var(--chrome-text); background:var(--input-bg); border:1px solid var(--chrome-border); border-radius:4px; }
  button { font:inherit; color:inherit; background:var(--app-bg); border:1px solid var(--chrome-border); border-radius:5px; padding:5px 10px; cursor:pointer; }
  button:disabled { opacity:.45; cursor:default; }
  button:focus-visible, select:focus-visible { outline:2px solid var(--accent); outline-offset:2px; }
  .presets { display:flex; flex-wrap:wrap; gap:6px; }
  .border-layout { display:grid; grid-template-columns:1fr 1fr; gap:12px; align-items:center; }
  .border-preview :global(g) { cursor:pointer; }
  .border-preview :global(g:focus-visible) { outline:2px solid var(--accent); }
  .border-preview { width:100%; min-width:120px; }
  .edges { display:grid; grid-template-columns:1fr 1fr; gap:6px; }
  .edges button { display:flex; flex-direction:column; gap:3px; }
  .edges span { font-size:11px; color:var(--chrome-muted); }
  .edited { border-color:var(--accent); }
  p { margin:0; font-size:12px; color:var(--chrome-muted); line-height:1.5; }
  @media (max-width:480px) { .border-layout { grid-template-columns:1fr; } .border-preview { max-height:150px; } }
</style>
