<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { t } from '../i18n';
  import { parseZoom } from '../grid/zoom';

  let { zoom, onapply, onclose }: {
    zoom: number;
    onapply: (percent: number | null) => void;
    onclose: () => void;
  } = $props();

  const presets = [200, 150, 125, 100, 75, 50, 25];
  const initial = untrack(() => zoom);
  let choice = $state(presets.includes(initial) ? String(initial) : 'custom');
  let custom = $state(String(initial));
  let dialog: HTMLDialogElement;
  let input: HTMLInputElement;

  onMount(() => {
    dialog.showModal();
    if (choice === 'custom') {
      input.focus();
      input.select();
    } else {
      dialog.querySelector<HTMLInputElement>('input:checked')?.focus();
    }
  });

  function close(): void {
    dialog.close();
    onclose();
  }

  function apply(): void {
    dialog.close();
    onapply(choice === 'fit' ? null : choice === 'custom' ? parseZoom(custom, initial) : Number(choice));
  }
</script>

<dialog bind:this={dialog} class="zoom-dialog" aria-labelledby="zoom-title"
  oncancel={(e) => { e.preventDefault(); close(); }}>
  <form class="dlg" onsubmit={(e) => { e.preventDefault(); apply(); }}>
    <h2 id="zoom-title">{t('縮放')}</h2>
    <fieldset>
      <legend>{t('縮放為：')}</legend>
      {#each presets as percent (percent)}
        <label><input type="radio" name="zoom-choice" value={String(percent)} bind:group={choice} /> {percent}%</label>
      {/each}
      <label><input type="radio" name="zoom-choice" value="fit" bind:group={choice} /> {t('選取範圍最適化')}</label>
      <label class="custom">
        <input type="radio" name="zoom-choice" value="custom" bind:group={choice} /> {t('自訂：')}
        <input bind:this={input} class="custom-percent" type="text" inputmode="decimal" bind:value={custom}
          aria-label={t('縮放百分比')} title={t('縮放範圍：10%–400%')}
          onfocus={() => (choice = 'custom')} /> %
      </label>
    </fieldset>
    <div class="dlg-actions">
      <button type="button" onclick={close}>{t('取消')}</button>
      <button type="submit" class="primary">{t('確定')}</button>
    </div>
  </form>
</dialog>

<style>
  .zoom-dialog { padding: 0; border: 0; border-radius: 12px; background: transparent; color: var(--chrome-text); max-width: calc(100vw - 32px); max-height: calc(100vh - 32px); }
  .zoom-dialog::backdrop { background: rgb(0 0 0 / 0.25); }
  .dlg { width: 280px; max-width: 100%; gap: 18px; padding: 20px; }
  fieldset { border: 0; padding: 8px 0 0; gap: 10px; }
  legend { padding: 0; color: var(--chrome-text); font-size: inherit; }
  label { min-height: 22px; }
  input[type='radio'] { margin: 0 4px 0 0; accent-color: var(--accent); }
  .custom { margin-top: 4px; }
  .custom-percent { width: 66px; text-align: right; }
  .dlg-actions { gap: 12px; }
  .dlg-actions button { flex: 1; }
</style>
