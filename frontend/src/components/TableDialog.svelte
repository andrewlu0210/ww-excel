<script lang="ts">
  // 插入 › 表格… (⌘T): confirm the range, headers and a style; or rename
  // a table.
  import type { RangeDTO } from '../bridge/sheet';
  import { colName } from '../grid/layout';

  interface Props {
    // Create mode: the suggested range and header guess.
    range?: RangeDTO;
    header?: boolean;
    styles?: string[];
    // Rename mode: the current name.
    renaming?: string;
    oncreate?: (header: boolean, style: string) => Promise<string | null>;
    onrename?: (name: string) => Promise<string | null>;
    onclose: () => void;
  }

  let { range, header: guess = true, styles = [], renaming, oncreate, onrename, onclose }: Props = $props();

  let header = $state(true);
  let name = $state('');
  let style = $state('TableStyleMedium2');
  let error = $state<string | null>(null);
  $effect.pre(() => {
    header = guess;
    name = renaming ?? '';
  });

  // Swatch colors for the style list (header, band).
  const swatches: Record<string, [string, string]> = {
    TableStyleMedium2: ['#4472C4', '#D9E1F2'],
    TableStyleMedium7: ['#70AD47', '#E2EFDA'],
    TableStyleMedium3: ['#ED7D31', '#FCE4D6'],
    TableStyleMedium5: ['#FFC000', '#FFF2CC'],
    TableStyleMedium6: ['#5B9BD5', '#DDEBF7'],
    TableStyleMedium4: ['#A5A5A5', '#EDEDED'],
    TableStyleMedium1: ['#000000', '#D9D9D9'],
    TableStyleLight9: ['#4472C4', '#FFFFFF'],
  };
  let rangeText = $derived(range ? `${colName(range.c0)}${range.r0 + 1}:${colName(range.c1)}${range.r1 + 1}` : '');

  async function ok(): Promise<void> {
    error = renaming !== undefined ? ((await onrename?.(name)) ?? null) : ((await oncreate?.(header, style)) ?? null);
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape') onclose();
    if (e.key === 'Enter' && !e.isComposing) void ok();
  }}
/>

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label="表格" style="width: 360px">
    {#if renaming !== undefined}
      <h2>重新命名表格</h2>
      <label>表格名稱 <input type="text" bind:value={name} /></label>
    {:else}
      <h2>建立表格</h2>
      <div>表格的資料範圍：<strong>{rangeText}</strong></div>
      <label><input type="checkbox" bind:checked={header} /> 我的表格有標題</label>
      <div class="styles" role="radiogroup" aria-label="表格樣式">
        {#each styles as s (s)}
          {@const sw = swatches[s] ?? ['#4472C4', '#D9E1F2']}
          <button role="radio" aria-checked={style === s} class:on={style === s} title={s} onclick={() => (style = s)}>
            <span style:background={sw[0]}></span><span style:background={sw[1]}></span><span style:background={sw[0] + '22'}></span>
          </button>
        {/each}
      </div>
    {/if}
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="dlg-actions">
      <button onclick={onclose}>取消</button>
      <button class="primary" onclick={() => void ok()}>確定</button>
    </div>
  </div>
</div>

<style>
  .styles {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 6px;
  }
  .styles button {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: 4px;
  }
  .styles button.on {
    outline: 2px solid var(--accent);
  }
  .styles span {
    height: 6px;
  }
  label input[type='text'] {
    flex: 1;
  }
  .error {
    color: var(--error-fg);
  }
</style>
