<script lang="ts">
  import { t } from '../i18n';
  import { MIN_ZOOM, MAX_ZOOM, normalizeZoom } from '../grid/zoom';

  let { zoom, onchange, onopen }: {
    zoom: number;
    onchange: (value: number) => void;
    onopen: () => void;
  } = $props();
</script>

<div class="zoom" role="group" aria-label={t('工作表縮放')}>
  <button type="button" aria-label={t('縮小')} title={t('縮小')} disabled={zoom <= MIN_ZOOM}
    onpointerdown={(e) => e.preventDefault()} onclick={() => onchange(normalizeZoom(zoom - 10))}>−</button>
  <input class="slider" type="range" min={MIN_ZOOM} max={MAX_ZOOM} step="1" value={zoom}
    aria-label={t('縮放比例')} aria-valuetext={`${zoom}%`}
    oninput={(e) => onchange(Number(e.currentTarget.value))} />
  <button type="button" aria-label={t('放大')} title={t('放大')} disabled={zoom >= MAX_ZOOM}
    onpointerdown={(e) => e.preventDefault()} onclick={() => onchange(normalizeZoom(zoom + 10))}>+</button>
  <button class="percentage" type="button" aria-label={t('縮放百分比')} aria-haspopup="dialog"
    title={t('縮放')} onpointerdown={(e) => e.preventDefault()} onclick={onopen}>{zoom}%</button>
</div>

<style>
  .zoom { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }
  button { width: 24px; height: 22px; padding: 0; border: 0; border-radius: 4px; background: transparent; color: var(--chrome-text); font: inherit; font-size: 18px; cursor: pointer; }
  button:hover:enabled { background: var(--app-bg); }
  button:disabled { opacity: 0.35; cursor: default; }
  .slider { width: 120px; min-width: 60px; height: 18px; margin: 0; accent-color: var(--accent); }
  .percentage { height: 22px; font-size: inherit; width: 48px; box-sizing: border-box; padding: 2px; border: 1px solid transparent; border-radius: 3px; background: transparent; color: var(--chrome-text); font: inherit; text-align: right; font-variant-numeric: tabular-nums; }
  .percentage:focus { border-color: var(--accent); outline: none; background: var(--app-bg); }
  @media (max-width: 650px) { .slider { width: 75px; } }
</style>
