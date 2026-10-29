<script lang="ts">
  // Sheet tabs: click to switch, double-click to rename, right-click for
  // a menu, drag to reorder, + to add.
  import { t } from '../i18n';
  import { tick } from 'svelte';

  interface Props {
    sheets: string[];
    active: number;
    onselect: (i: number) => void;
    onadd: () => void;
    onrename: (i: number, name: string) => Promise<string | null>;
    ondelete: (i: number) => void;
    onmove: (from: number, to: number) => void;
    // Hand keyboard focus back to the sheet.
    ondone: () => void;
  }

  let { sheets, active, onselect, onadd, onrename, ondelete, onmove, ondone }: Props = $props();

  let renaming = $state<number | null>(null);
  let draft = $state('');
  let error = $state<string | null>(null);
  let menu = $state<{ i: number; x: number; y: number } | null>(null);
  let dragFrom: number | null = null;
  let dropAt = $state<number | null>(null);
  let input = $state<HTMLInputElement>();

  async function startRename(i: number): Promise<void> {
    menu = null;
    renaming = i;
    draft = sheets[i];
    error = null;
    await tick();
    input?.focus();
    input?.select();
  }

  async function finishRename(commit: boolean): Promise<void> {
    if (renaming === null) return;
    const i = renaming;
    if (commit && draft !== sheets[i]) {
      const err = await onrename(i, draft);
      if (err) {
        error = err;
        input?.focus();
        return;
      }
    }
    renaming = null;
    error = null;
    ondone();
  }

  function onPointerDown(e: PointerEvent, i: number): void {
    if (e.button !== 0 || renaming !== null) return;
    e.preventDefault();
    dragFrom = i;
    onselect(i);
  }

  function onPointerEnter(i: number): void {
    if (dragFrom !== null && dragFrom !== i) dropAt = i;
  }

  function onPointerUp(): void {
    if (dragFrom !== null && dropAt !== null && dropAt !== dragFrom) onmove(dragFrom, dropAt);
    dragFrom = null;
    dropAt = null;
    ondone();
  }
</script>

<svelte:window
  onpointerup={onPointerUp}
  onpointerdown={(e) => menu && !(e.target as Element).closest?.('.tab-menu') && (menu = null)}
/>

<div class="tabs" data-sheet-tabs role="tablist" aria-label={t('工作表')}>
  {#each sheets as name, i (i + ':' + name)}
    {#if renaming === i}
      <span class="tab active editing">
        <input
          bind:this={input}
          bind:value={draft}
          spellcheck="false"
          aria-label={t('工作表名稱')}
          onkeydown={(e) => {
            if (e.key === 'Enter') void finishRename(true);
            if (e.key === 'Escape') void finishRename(false);
            e.stopPropagation();
          }}
          onblur={() => void finishRename(true)}
        />
        {#if error}<span class="error" role="alert">{error}</span>{/if}
      </span>
    {:else}
      <button
        class="tab"
        class:active={i === active}
        class:drop={dropAt === i}
        role="tab"
        aria-selected={i === active}
        tabindex="-1"
        onpointerdown={(e) => onPointerDown(e, i)}
        onpointerenter={() => onPointerEnter(i)}
        ondblclick={() => void startRename(i)}
        oncontextmenu={(e) => {
          e.preventDefault();
          onselect(i);
          menu = { i, x: e.clientX, y: e.clientY };
        }}>{name}</button
      >
    {/if}
  {/each}
  <button class="add" tabindex="-1" title={t('新增工作表 (⇧F11)')} onpointerdown={(e) => e.preventDefault()} onclick={onadd}>{t('＋')}</button>
</div>

{#if menu}
  {@const i = menu.i}
  <ul class="tab-menu" role="menu" style:left="{menu.x}px" style:top="{menu.y}px">
    <li><button role="menuitem" onclick={() => { menu = null; onadd(); }}>{t('插入工作表')}</button></li>
    <li><button role="menuitem" onclick={() => void startRename(i)}>{t('重新命名')}</button></li>
    <li><button role="menuitem" disabled={sheets.length <= 1} onclick={() => { menu = null; ondelete(i); }}>{t('刪除')}</button></li>
    <li><button role="menuitem" disabled={i === 0} onclick={() => { menu = null; onmove(i, i - 1); }}>{t('左移')}</button></li>
    <li><button role="menuitem" disabled={i === sheets.length - 1} onclick={() => { menu = null; onmove(i, i + 1); }}>{t('右移')}</button></li>
  </ul>
{/if}

<style>
  .tabs {
    display: flex;
    align-items: stretch;
    height: 100%;
    gap: 1px;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tab {
    display: flex;
    align-items: center;
    position: relative;
    padding: 0 14px;
    font: inherit;
    color: var(--chrome-text);
    background: none;
    border: 0;
    border-bottom: 2px solid transparent;
    white-space: nowrap;
    cursor: default;
  }
  .tab:hover {
    background: var(--app-bg);
  }
  .tab.active {
    color: var(--accent);
    font-weight: 600;
    border-bottom-color: var(--accent);
    background: var(--input-bg);
  }
  .tab.drop {
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .tab.editing {
    padding: 0 4px;
  }
  .tab input {
    width: 110px;
    font: inherit;
    padding: 1px 4px;
    color: var(--chrome-text);
    background: var(--input-bg);
    border: 1px solid var(--accent);
    border-radius: 3px;
    -webkit-user-select: text;
    user-select: text;
  }
  .error {
    position: absolute;
    bottom: 100%;
    left: 0;
    margin-bottom: 4px;
    padding: 4px 8px;
    white-space: nowrap;
    color: var(--error-fg);
    background: var(--popup-bg);
    border: 1px solid var(--error-fg);
    border-radius: 6px;
    font-weight: 400;
  }
  .add {
    padding: 0 10px;
    font: inherit;
    font-size: 14px;
    color: var(--chrome-muted);
    background: none;
    border: 0;
  }
  .add:hover {
    color: var(--accent);
  }
  .tab-menu {
    position: fixed;
    z-index: 60;
    margin: 0;
    padding: 4px 0;
    list-style: none;
    transform: translateY(-100%);
    background: var(--popup-bg);
    border: 1px solid var(--popup-border);
    border-radius: 8px;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.2);
  }
  .tab-menu button {
    width: 100%;
    padding: 4px 16px;
    text-align: left;
    font: 12px var(--ui-font);
    color: var(--chrome-text);
    background: none;
    border: 0;
  }
  .tab-menu button:hover:not(:disabled) {
    background: var(--popup-active);
  }
  .tab-menu button:disabled {
    color: var(--chrome-muted);
  }
</style>
