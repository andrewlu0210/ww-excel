<script lang="ts">
  // Sheet tabs: click to switch, double-click to rename, right-click for
  // a menu, drag to reorder, + to add.
  import ColorPicker from './ColorPicker.svelte';
  import { sheetTabStyle } from './sheetTabColor';
  import { t } from '../i18n';
  import { tick } from 'svelte';

  interface Props {
    sheets: string[];
    colors?: string[];
    oncolor?:(i:number,color:string)=>void;
    hidden?: number[];
    active: number;
    onselect: (i: number) => void;
    onadd: () => void;
    onrename: (i: number, name: string) => Promise<string | null>;
    ondelete: (i: number) => void;
    onmove: (from: number, to: number) => void;
    // Hand keyboard focus back to the sheet.
    ondone: () => void;
  }

  let { sheets, colors = [], oncolor, hidden = [], active, onselect, onadd, onrename, ondelete, onmove, ondone }: Props = $props();

  const visibleCount = $derived(sheets.length - hidden.length);

  let renaming = $state<number | null>(null);
  let draft = $state('');
  let error = $state<string | null>(null);
  let menu = $state<{ i: number; x: number; y: number } | null>(null);
  let colorTarget=$state<{i:number;x:number;y:number}|null>(null);
  function setColor(i:number,color:string):void{colorTarget=null;oncolor?.(i,color);ondone();}
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

  // Capture the derived menu index as an argument before clearing menu.
  function deleteSheet(i: number): void {
    menu = null;
    ondelete(i);
  }

  function moveSheet(from: number, to: number): void {
    menu = null;
    onmove(from, to);
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
  onpointerdown={(e)=>{if(!(e.target as Element).closest?.('.tab-menu,.tab-color-popup')){menu=null;colorTarget=null;}}}
/>

<div class="tabs" data-sheet-tabs role="tablist" aria-label={t('工作表')}>
  {#each sheets as name, i (i + ':' + name)}
    {@const colorStyle=sheetTabStyle(colors[i])}
    {#if !hidden.includes(i)}
    {#if renaming === i}
      <span class="tab active editing" class:colored={!!colorStyle} style={colorStyle}>
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
        class:colored={!!colorStyle}
        style={colorStyle}
        data-tab-color={colors[i] ?? ''}
        class:active={i === active}
        class:drop={dropAt === i}
        data-sheet-index={i}
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
    {/if}
  {/each}
  <button class="add" tabindex="-1" title={t('新增工作表 (⇧F11)')} onpointerdown={(e) => e.preventDefault()} onclick={onadd}>{t('＋')}</button>
</div>

{#if menu}
  {@const i = menu.i}
  <ul class="tab-menu" role="menu" style:left="{menu.x}px" style:top="{menu.y}px">
    <li><button role="menuitem" onclick={() => { menu = null; onadd(); }}>{t('插入工作表')}</button></li>
    <li><button role="menuitem" onclick={() => void startRename(i)}>{t('重新命名')}</button></li>
    <li><button role="menuitem" disabled={!oncolor} onclick={()=>{colorTarget={i,x:Math.max(4,Math.min(menu!.x,window.innerWidth-248)),y:Math.max(4,menu!.y-270)};menu=null;}}>{t('分頁色彩…')}</button></li>
    <li><button role="menuitem" disabled={visibleCount <= 1} onclick={() => deleteSheet(i)}>{t('刪除')}</button></li>
    <li><button role="menuitem" disabled={i === 0} onclick={() => moveSheet(i, i - 1)}>{t('左移')}</button></li>
    <li><button role="menuitem" disabled={i === sheets.length - 1} onclick={() => moveSheet(i, i + 1)}>{t('右移')}</button></li>
  </ul>
{/if}

{#if colorTarget}
 {@const target=colorTarget}
 <div class="tab-color-popup" style:left="{target.x}px" style:top="{target.y}px">
  <ColorPicker resetLabel={t('無色彩')} onpick={color=>setColor(target.i,color)} onclose={()=>colorTarget=null}/>
 </div>
{/if}

<style>
  .tab-color-popup {position:fixed;z-index:50;}
  .tab-color-popup :global(.picker){position:static;margin:0;}
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
  .tab.colored { background:var(--sheet-tab-color);color:var(--sheet-tab-text); }
  .tab.colored:hover { filter:brightness(1.08); }
  .tab.colored.active { background:var(--sheet-tab-active-bg);color:#111111;border-bottom-color:var(--sheet-tab-color); }
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
