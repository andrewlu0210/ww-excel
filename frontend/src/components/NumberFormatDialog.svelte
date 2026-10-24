<script lang="ts">
  // 儲存格格式 › 數值 (⌘1): pick a category or type a format code. The
  // preview is computed by Go's formatter so it matches the grid.
  import { onMount } from 'svelte';

  interface Props {
    code: string;
    // Renders the active cell with a format code.
    preview: (code: string) => Promise<string>;
    onapply: (code: string) => void;
    onclose: () => void;
  }

  let { code: initial, preview: render, onapply, onclose }: Props = $props();

  const categories: { label: string; code: string }[] = [
    { label: '通用格式', code: '' },
    { label: '數值', code: '#,##0.00' },
    { label: '貨幣', code: '"$"#,##0.00_);[Red]\\("$"#,##0.00\\)' },
    { label: '會計專用', code: '_("$"* #,##0.00_);_("$"* \\(#,##0.00\\);_("$"* "-"??_);_(@_)' },
    { label: '日期', code: 'yyyy/m/d' },
    { label: '日期（中文）', code: 'yyyy"年"m"月"d"日"' },
    { label: '民國年', code: '[$-404]e/m/d' },
    { label: '民國年（中文）', code: '[$-404]gge"年"m"月"d"日"' },
    { label: '時間', code: 'h:mm:ss' },
    { label: '上午／下午', code: '上午/下午 h"時"mm"分"' },
    { label: '百分比', code: '0.00%' },
    { label: '分數', code: '# ?/?' },
    { label: '科學記號', code: '0.00E+00' },
    { label: '文字', code: '@' },
  ];

  let code = $state('');
  let preview = $state('');
  let input: HTMLInputElement;

  onMount(() => {
    code = initial;
    input.focus();
    input.select();
  });

  $effect(() => {
    const c = code;
    render(c).then((p) => {
      if (c === code) preview = p;
    });
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dialog" role="dialog" aria-modal="true" aria-label="儲存格格式">
    <h2>儲存格格式 — 數值</h2>
    <div class="body">
      <ul class="list" role="listbox" aria-label="類別">
        {#each categories as c (c.label)}
          <li role="option" aria-selected={c.code === code}>
            <button class:selected={c.code === code} onclick={() => (code = c.code)}>{c.label}</button>
          </li>
        {/each}
      </ul>
      <div class="right">
        <label>
          格式碼
          <input bind:this={input} bind:value={code} spellcheck="false" placeholder="通用格式"
            onkeydown={(e) => e.key === 'Enter' && onapply(code)} />
        </label>
        <div class="preview">
          <span class="muted">範例</span>
          <span class="value">{preview}</span>
        </div>
      </div>
    </div>
    <div class="actions">
      <button onclick={onclose}>取消</button>
      <button class="primary" onclick={() => onapply(code)}>確定</button>
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgb(0 0 0 / 0.25);
  }
  .dialog {
    width: min(560px, calc(100% - 32px));
    padding: 16px;
    background: var(--chrome-bg);
    border: 1px solid var(--chrome-border);
    border-radius: 10px;
    box-shadow: 0 12px 40px rgb(0 0 0 / 0.3);
    color: var(--chrome-text);
  }
  h2 {
    margin: 0 0 12px;
    font-size: 14px;
  }
  .body {
    display: flex;
    gap: 16px;
  }
  .list {
    width: 170px;
    max-height: 280px;
    margin: 0;
    padding: 4px 0;
    overflow: auto;
    list-style: none;
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
  }
  .list button {
    width: 100%;
    padding: 4px 10px;
    text-align: left;
    font: inherit;
    color: inherit;
    background: none;
    border: 0;
  }
  .list button.selected {
    background: var(--popup-active);
  }
  .right {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  input {
    font: 13px ui-monospace, Menlo, monospace;
    padding: 5px 8px;
    color: var(--chrome-text);
    background: var(--grid-bg);
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
    -webkit-user-select: text;
    user-select: text;
  }
  .preview {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px;
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
  }
  .muted {
    color: var(--chrome-muted);
    font-size: 11px;
  }
  .value {
    font-size: 15px;
    min-height: 20px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
  .actions button {
    padding: 4px 16px;
    font: inherit;
    color: var(--chrome-text);
    background: var(--app-bg);
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
  }
  .actions .primary {
    color: white;
    background: var(--accent);
    border-color: var(--accent);
  }
</style>
