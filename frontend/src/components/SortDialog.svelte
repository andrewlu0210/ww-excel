<script lang="ts">
  // 資料 › 排序…: up to three keys, with or without a header row.
  import type { RangeDTO, SortKeyDTO } from '../bridge/sheet';

  interface Props {
    range: RangeDTO;
    header: boolean;
    // Column choices for the range, labelled by header text or letter.
    columns: (header: boolean) => { col: number; label: string }[];
    onapply: (range: RangeDTO, keys: SortKeyDTO[], header: boolean) => void;
    onclose: () => void;
  }

  let { range, header: initialHeader, columns, onapply, onclose }: Props = $props();

  let header = $state(false);
  let keys = $state<{ col: number; desc: boolean }[]>([]);
  $effect.pre(() => {
    header = initialHeader;
    keys = [{ col: range.c0, desc: false }];
  });
  let choices = $derived(columns(header));

  function add(): void {
    const used = new Set(keys.map((k) => k.col));
    const next = choices.find((c) => !used.has(c.col));
    if (next) keys.push({ col: next.col, desc: false });
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label="排序" style="width: 420px">
    <h2>排序</h2>
    <label><input type="checkbox" bind:checked={header} /> 我的資料有標題</label>
    {#each keys as key, i (i)}
      <div class="key">
        <span>{i === 0 ? '排序方式' : '次要依據'}</span>
        <select bind:value={key.col}>
          {#each choices as c (c.col)}
            <option value={c.col}>{c.label}</option>
          {/each}
        </select>
        <select bind:value={key.desc}>
          <option value={false}>A 到 Z（最小到最大）</option>
          <option value={true}>Z 到 A（最大到最小）</option>
        </select>
        {#if i > 0}<button aria-label="移除" onclick={() => keys.splice(i, 1)}>－</button>{/if}
      </div>
    {/each}
    <div>
      <button disabled={keys.length >= 3 || keys.length >= choices.length} onclick={add}>＋ 新增層級</button>
    </div>
    <div class="dlg-actions">
      <button onclick={onclose}>取消</button>
      <button class="primary" onclick={() => onapply(range, keys, header)}>確定</button>
    </div>
  </div>
</div>

<style>
  .key {
    display: grid;
    grid-template-columns: 64px 1fr 1fr auto;
    gap: 6px;
    align-items: center;
  }
  .key span {
    color: var(--chrome-muted);
    font-size: 12px;
  }
</style>
