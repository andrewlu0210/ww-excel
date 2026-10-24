<script lang="ts">
  // 尋找與取代 (⌘F / ⇧⌘H): a non-modal panel; the sheet keeps working.
  import { onMount } from 'svelte';
  import type { FindOptionsDTO, HitDTO } from '../bridge/sheet';
  import { cellAddress } from '../grid/layout';

  interface Props {
    replaceMode: boolean;
    sheets: string[];
    // Each returns a message to show, or null.
    onfind: (opt: FindOptionsDTO, forward: boolean) => Promise<string | null>;
    onfindall: (opt: FindOptionsDTO) => Promise<HitDTO[]>;
    onreplace: (opt: FindOptionsDTO, withText: string, all: boolean) => Promise<string | null>;
    ongoto: (hit: HitDTO) => void;
    onclose: () => void;
  }

  let { replaceMode = $bindable(), sheets, onfind, onfindall, onreplace, ongoto, onclose }: Props = $props();

  let text = $state('');
  let withText = $state('');
  let matchCase = $state(false);
  let wholeCell = $state(false);
  let inFormulas = $state(false);
  let allSheets = $state(false);
  let message = $state<string | null>(null);
  let hits = $state<HitDTO[] | null>(null);
  let busy = $state(false);
  let input: HTMLInputElement;

  const opt = (): FindOptionsDTO => ({ text, matchCase, wholeCell, inFormulas, allSheets });

  async function run(fn: () => Promise<string | null>): Promise<void> {
    if (text === '' || busy) return;
    busy = true;
    try {
      message = await fn();
    } finally {
      busy = false;
    }
  }

  async function findAll(): Promise<void> {
    if (text === '') return;
    hits = await onfindall(opt());
    message = hits.length === 0 ? '找不到符合的資料。' : `找到 ${hits.length >= 1000 ? '1,000+' : hits.length} 個儲存格`;
  }

  onMount(() => input.focus());

  function onKey(e: KeyboardEvent): void {
    if (e.key === 'Escape') onclose();
    else if (e.key === 'Enter' && !e.isComposing) {
      e.preventDefault();
      void run(() => onfind(opt(), !e.shiftKey));
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="dlg panel" role="dialog" aria-label="尋找與取代" tabindex="-1" onkeydown={onKey}>
  <div class="head">
    <div class="tabs" role="tablist">
      <button role="tab" aria-selected={!replaceMode} class:on={!replaceMode} onclick={() => (replaceMode = false)}>尋找</button>
      <button role="tab" aria-selected={replaceMode} class:on={replaceMode} onclick={() => (replaceMode = true)}>取代</button>
    </div>
    <button class="close" aria-label="關閉" onclick={onclose}>✕</button>
  </div>
  <label>尋找目標 <input type="text" bind:this={input} bind:value={text} /></label>
  {#if replaceMode}
    <label>取代成 <input type="text" bind:value={withText} /></label>
  {/if}
  <div class="options">
    <label><input type="checkbox" bind:checked={matchCase} /> 大小寫須相符</label>
    <label><input type="checkbox" bind:checked={wholeCell} /> 儲存格內容須完全相符</label>
    <label><input type="checkbox" bind:checked={inFormulas} disabled={replaceMode} /> 搜尋公式</label>
    <label><input type="checkbox" bind:checked={allSheets} /> 整個活頁簿</label>
  </div>
  {#if message}<div class="message" role="status">{message}</div>{/if}
  <div class="dlg-actions">
    {#if replaceMode}
      <button disabled={busy || !text} onclick={() => void run(() => onreplace({ ...opt(), inFormulas: true }, withText, true))}>全部取代</button>
      <button disabled={busy || !text} onclick={() => void run(() => onreplace({ ...opt(), inFormulas: true }, withText, false))}>取代</button>
    {:else}
      <button disabled={busy || !text} onclick={() => void findAll()}>全部尋找</button>
    {/if}
    <button disabled={busy || !text} onclick={() => void run(() => onfind(opt(), false))}>上一個</button>
    <button class="primary" disabled={busy || !text} onclick={() => void run(() => onfind(opt(), true))}>找下一個</button>
  </div>
  {#if hits && hits.length > 0 && !replaceMode}
    <ul class="hits">
      {#each hits as h, i (i)}
        <li>
          <button onclick={() => ongoto(h)}>
            <span class="where">{sheets.length > 1 ? `${sheets[h.sheet]}!` : ''}{cellAddress(h.row, h.col)}</span>
            <span class="v">{h.v}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .panel {
    position: absolute;
    top: 84px;
    right: 16px;
    z-index: 45;
    width: 380px;
    gap: 10px;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .tabs {
    display: flex;
    gap: 4px;
  }
  .tabs button {
    border-color: transparent;
    background: none;
  }
  .tabs button.on {
    background: var(--popup-active);
    border-color: var(--chrome-border);
  }
  .close {
    border: 0 !important;
    background: none !important;
    padding: 2px 6px !important;
  }
  label {
    justify-content: space-between;
  }
  label input[type='text'] {
    flex: 1;
    margin-left: 8px;
  }
  .options {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4px;
    font-size: 12px;
  }
  .options label {
    justify-content: flex-start;
  }
  .message {
    color: var(--chrome-muted);
    font-size: 12px;
  }
  .hits {
    max-height: 200px;
    margin: 0;
    padding: 0;
    overflow: auto;
    list-style: none;
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
  }
  .hits button {
    display: flex;
    gap: 12px;
    width: 100%;
    padding: 3px 8px;
    border: 0;
    border-radius: 0;
    background: none;
    text-align: left;
  }
  .hits button:hover {
    background: var(--popup-active);
  }
  .where {
    min-width: 80px;
    color: var(--chrome-muted);
    font-variant-numeric: tabular-nums;
  }
  .v {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
