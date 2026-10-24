<script lang="ts">
  // 插入… / 刪除… for a cell selection: whole rows or whole columns.
  import { t } from '../i18n';
  interface Props {
    insert: boolean;
    onapply: (rows: boolean) => void;
    onclose: () => void;
  }

  let { insert, onapply, onclose }: Props = $props();
  let rows = $state(true);
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape') onclose();
    if (e.key === 'Enter') onapply(rows);
  }}
/>

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label={insert ? t('插入') : t('刪除')} style="width: 260px">
    <h2>{insert ? t('插入') : t('刪除')}</h2>
    <fieldset>
      <label><input type="radio" name="shift" value={true} bind:group={rows} /> {t('整列')}</label>
      <label><input type="radio" name="shift" value={false} bind:group={rows} /> {t('整欄')}</label>
    </fieldset>
    <div class="dlg-actions">
      <button onclick={onclose}>{t('取消')}</button>
      <button class="primary" onclick={() => onapply(rows)}>{t('確定')}</button>
    </div>
  </div>
</div>
