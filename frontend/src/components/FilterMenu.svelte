<script lang="ts">
  // AutoFilter dropdown: sort by the column, or pick the values it shows.
  import type { FilterValueDTO } from '../bridge/sheet';

  interface Props {
    x: number;
    y: number;
    title: string;
    values: FilterValueDTO[];
    onsort: (desc: boolean) => void;
    // null = show everything (clear the column's filter).
    onapply: (shown: string[] | null) => void;
    onclose: () => void;
  }

  let { x, y, title, values, onsort, onapply, onclose }: Props = $props();

  let checked = $state<boolean[]>([]);
  let search = $state('');
  $effect.pre(() => {
    checked = values.map((v) => v.shown);
  });

  let visible = $derived(
    values.map((v, i) => ({ ...v, i })).filter((v) => search === '' || v.text.toLowerCase().includes(search.toLowerCase())),
  );
  let all = $derived(visible.length > 0 && visible.every((v) => checked[v.i]));

  function toggleAll(): void {
    const on = !all;
    for (const v of visible) checked[v.i] = on;
  }

  function apply(): void {
    if (checked.every(Boolean)) return onapply(null);
    onapply(values.filter((_, i) => checked[i]).map((v) => v.text));
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="backdrop" onpointerdown={onclose}></div>
<div class="dlg menu" style:left="{x}px" style:top="{y}px" role="dialog" aria-label="篩選 {title}">
  <div class="sorts">
    <button onclick={() => onsort(false)}>A→Z 遞增排序</button>
    <button onclick={() => onsort(true)}>Z→A 遞減排序</button>
  </div>
  <input type="text" placeholder="搜尋" bind:value={search} />
  <ul>
    <li>
      <label><input type="checkbox" checked={all} onchange={toggleAll} /> （全選）</label>
    </li>
    {#each visible as v (v.i)}
      <li>
        <label><input type="checkbox" bind:checked={checked[v.i]} /> {v.text === '' ? '（空格）' : v.text}</label>
      </li>
    {/each}
  </ul>
  <div class="dlg-actions">
    <button onclick={onclose}>取消</button>
    <button class="primary" disabled={!checked.some(Boolean)} onclick={apply}>確定</button>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 30;
  }
  .menu {
    position: absolute;
    z-index: 31;
    width: 230px;
    gap: 8px;
    padding: 10px;
  }
  .sorts {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .sorts button {
    text-align: left;
  }
  ul {
    max-height: 220px;
    margin: 0;
    padding: 4px 6px;
    overflow: auto;
    list-style: none;
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
  }
  li label {
    padding: 1px 0;
    white-space: nowrap;
  }
</style>
