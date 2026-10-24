<script lang="ts">
  // 公式 › 定義名稱 / 名稱管理員: list, add, edit and delete names.
  import { t } from '../i18n';
  import { onMount } from 'svelte';
  import { documentApi, type NameDTO } from '../bridge/sheet';

  interface Props {
    sheets: string[];
    // Suggested reference for a new name (the selection).
    selectionRef: string;
    // Start with the editor open for a new name (定義名稱).
    startNew: boolean;
    onclose: () => void;
  }

  let { sheets, selectionRef, startNew, onclose }: Props = $props();

  let list = $state<NameDTO[]>([]);
  // The name being edited: original name/scope ("" = new) and fields.
  let edit = $state<{ oldName: string; oldScope: number; n: NameDTO } | null>(null);
  let error = $state<string | null>(null);

  async function load(): Promise<void> {
    list = await documentApi.names();
  }

  function startEdit(n: NameDTO | null): void {
    error = null;
    edit = n
      ? { oldName: n.name, oldScope: n.scope, n: { ...n } }
      : { oldName: '', oldScope: -1, n: { name: '', refersTo: '=' + selectionRef, scope: -1, comment: '' } };
  }

  async function save(): Promise<void> {
    if (!edit) return;
    error = await documentApi.setName(edit.oldName, edit.oldScope, edit.n);
    if (!error) {
      edit = null;
      await load();
    }
  }

  async function remove(n: NameDTO): Promise<void> {
    error = await documentApi.deleteName(n.name, n.scope);
    await load();
  }

  onMount(() => {
    void load();
    if (startNew) startEdit(null);
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && (edit ? (edit = null) : onclose())} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label={t('名稱管理員')} style="width: 560px">
    <h2>{t('名稱管理員')}</h2>
    {#if edit}
      <label>{t('名稱')} <input type="text" bind:value={edit.n.name} /></label>
      <label>
        {t('範圍')}
        <select bind:value={edit.n.scope}>
          <option value={-1}>{t('活頁簿')}</option>
          {#each sheets as s, i (i)}<option value={i}>{s}</option>{/each}
        </select>
      </label>
      <label>{t('參照到')} <input type="text" bind:value={edit.n.refersTo} /></label>
      <label>{t('註解')} <input type="text" bind:value={edit.n.comment} /></label>
      {#if error}<div class="error" role="alert">{error}</div>{/if}
      <div class="dlg-actions">
        <button onclick={() => (edit = null)}>{t('取消')}</button>
        <button class="primary" onclick={() => void save()}>{t('確定')}</button>
      </div>
    {:else}
      <table>
        <thead><tr><th>{t('名稱')}</th><th>{t('參照到')}</th><th>{t('範圍')}</th><th></th></tr></thead>
        <tbody>
          {#each list as n (n.name + n.scope)}
            <tr>
              <td>{n.name}</td>
              <td class="ref">{n.refersTo}</td>
              <td>{n.scope < 0 ? t('活頁簿') : sheets[n.scope]}</td>
              <td class="row-actions">
                <button onclick={() => startEdit(n)}>{t('編輯')}</button>
                <button onclick={() => void remove(n)}>{t('刪除')}</button>
              </td>
            </tr>
          {:else}
            <tr><td colspan="4" class="empty">{t('還沒有定義任何名稱')}</td></tr>
          {/each}
        </tbody>
      </table>
      {#if error}<div class="error" role="alert">{error}</div>{/if}
      <div class="dlg-actions">
        <button onclick={() => startEdit(null)}>{t('新增…')}</button>
        <button class="primary" onclick={onclose}>{t('關閉')}</button>
      </div>
    {/if}
  </div>
</div>

<style>
  label {
    justify-content: space-between;
  }
  label input,
  label select {
    width: 380px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }
  th,
  td {
    padding: 4px 6px;
    text-align: left;
    border-bottom: 1px solid var(--chrome-border);
  }
  .ref {
    font-family: ui-monospace, Menlo, monospace;
  }
  .row-actions {
    white-space: nowrap;
    text-align: right;
  }
  .row-actions button {
    padding: 1px 8px;
  }
  .empty {
    color: var(--chrome-muted);
    text-align: center;
  }
  .error {
    color: var(--error-fg);
  }
</style>
