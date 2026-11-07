<script lang="ts">
  import type { FillSpec } from '../../bindings/ww-excel/internal/model/models';
  import { paintCellFill } from '../grid/cellFills';
  import { t } from '../i18n';
  let { color, mixed = false, spec = {} }: { color: string; mixed?: boolean; spec?: FillSpec } = $props();
  let canvas = $state<HTMLCanvasElement>();
  $effect(()=>{if(canvas && spec.kind && !mixed){canvas.width=320;canvas.height=80;paintCellFill(canvas.getContext('2d')!,spec,0,0,320,80);}});
</script>

<div class="fill-preview" role="img" aria-label={`${t('填滿預覽')}：${mixed ? t('混合') : color ? `#${color}` : t('無填滿')}`}>
  <div class="sample" style:background={!mixed && color ? `#${color}` : 'transparent'}>{#if !mixed && spec.kind}<canvas bind:this={canvas} style="width:100%;height:80px;display:block"></canvas>{/if}</div>
  <div class="caption">{mixed ? t('混合') : spec.kind ? t(spec.kind==='pattern'?'圖樣填滿':spec.kind==='linear'?'線性漸層':'矩形路徑漸層') : color ? `#${color}` : t('無填滿')}</div>
</div>

<style>
  .fill-preview { overflow:hidden; border:1px solid var(--chrome-border); border-radius:6px; background:repeating-conic-gradient(#ddd 0% 25%, #fff 0% 50%) 0 0 / 16px 16px; }
  .sample { min-height:80px; }
  .caption { padding:6px 8px; color:var(--chrome-text); background:var(--chrome-bg); border-top:1px solid var(--chrome-border); font-size:12px; }
</style>
