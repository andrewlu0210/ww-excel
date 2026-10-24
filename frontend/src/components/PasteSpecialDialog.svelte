<script lang="ts">
  // 編輯 › 選擇性貼上… (⌃⌘V)
  import { t } from '../i18n';
  import type { PasteMode } from '../bridge/sheet';

  interface Props {
    onapply: (mode: PasteMode, transpose: boolean, skipBlanks: boolean) => void;
    onclose: () => void;
  }

  let { onapply, onclose }: Props = $props();
  let mode = $state<PasteMode>('all');
  let transpose = $state(false);
  let skipBlanks = $state(false);
  const modes: { value: PasteMode; label: string }[] = [
    { value: 'all', label: t('全部') },
    { value: 'formulas', label: t('公式') },
    { value: 'values', label: t('值') },
    { value: 'formats', label: t('格式') },
  ];
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape') onclose();
    if (e.key === 'Enter') onapply(mode, transpose, skipBlanks);
  }}
/>

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label={t('選擇性貼上')} style="width: 300px">
    <h2>{t('選擇性貼上')}</h2>
    <fieldset>
      <legend>{t('貼上')}</legend>
      {#each modes as m (m.value)}
        <label><input type="radio" name="paste-mode" value={m.value} bind:group={mode} /> {m.label}</label>
      {/each}
    </fieldset>
    <label><input type="checkbox" bind:checked={skipBlanks} /> {t('略過空格')}</label>
    <label><input type="checkbox" bind:checked={transpose} /> {t('轉置')}</label>
    <div class="dlg-actions">
      <button onclick={onclose}>{t('取消')}</button>
      <button class="primary" onclick={() => onapply(mode, transpose, skipBlanks)}>{t('確定')}</button>
    </div>
  </div>
</div>
