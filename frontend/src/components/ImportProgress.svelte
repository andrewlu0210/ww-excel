<script lang="ts">
  import { onMount } from 'svelte';
  import { t } from '../i18n';
  let { name, oncancel }: { name: string; oncancel: () => Promise<boolean> } = $props();
  let dialog: HTMLDialogElement;
  let cancelling = $state(false);
  onMount(() => dialog.showModal());
  async function cancel(): Promise<void> {
    if (cancelling) return;
    cancelling = true;
    try { await oncancel(); } catch { cancelling = false; }
  }
</script>

<dialog class="import-progress" bind:this={dialog} aria-labelledby="import-title" oncancel={(e) => { e.preventDefault(); void cancel(); }}>
  <div class="dlg">
    <h2 id="import-title">{t('正在開啟檔案')}</h2>
    <p>{name}</p>
    <p role="status">{t(cancelling ? '正在取消…' : '正在讀取及計算，請稍候…')}</p>
    <div class="dlg-actions"><button type="button" disabled={cancelling} onclick={() => void cancel()}>{t('取消')}</button></div>
  </div>
</dialog>

<style>
  dialog { padding: 0; border: 0; border-radius: 12px; color: var(--chrome-text); background: var(--chrome-bg, #292929); max-width: calc(100vw - 32px); }
  dialog::backdrop { background: rgb(0 0 0 / 0.25); }
  .dlg { width: 340px; max-width: 100%; padding: 20px; gap: 12px; }
  p { overflow-wrap: anywhere; margin: 0; }
</style>
