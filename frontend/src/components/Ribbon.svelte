<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { MenuAction } from '../input/keymap';
  import { t } from '../i18n';
  let { children, canUndo, canRedo, onaction }: {
    children: Snippet; canUndo: boolean; canRedo: boolean;
    onaction: (action: MenuAction) => void;
  } = $props();
  const tabs = ['常用', '插入', '頁面配置', '公式', '資料', '檢視'];
  let active = $state(0);
  const panels: { label: string; action: MenuAction; icon: string }[][] = [[],
    [{label:'表格',action:'insertTable',icon:'▦'}, {label:'圖片',action:'insertPicture',icon:'▧'}, {label:'圖表',action:'insertChart',icon:'▥'}, {label:'新增工作表',action:'insertSheet',icon:'＋'}],
    [{label:'版面設定…',action:'pageSetup',icon:'▤'}, {label:'列印預覽…',action:'printPreview',icon:'▣'}, {label:'列印…',action:'print',icon:'▤'}, {label:'匯出 PDF…',action:'exportPDF',icon:'↗'}],
    [{label:'定義名稱…',action:'defineName',icon:'ƒ'}, {label:'名稱管理員…',action:'nameManager',icon:'≡'}],
    [{label:'排序…',action:'sort',icon:'↕'}, {label:'遞增排序',action:'sortAsc',icon:'↑'}, {label:'遞減排序',action:'sortDesc',icon:'↓'}, {label:'篩選',action:'filter',icon:'▽'}, {label:'資料驗證…',action:'dataValidation',icon:'✓'}],
    [{label:'凍結窗格',action:'freezePanes',icon:'▦'}, {label:'凍結頂端列',action:'freezeTopRow',icon:'▤'}, {label:'凍結第一欄',action:'freezeFirstCol',icon:'▥'}, {label:'取消凍結窗格',action:'unfreeze',icon:'▦'}, {label:'偏好設定…',action:'preferences',icon:'⚙'}],
  ];
  function moveTab(e: KeyboardEvent) {
    if (!['ArrowLeft','ArrowRight','Home','End'].includes(e.key)) return;
    e.preventDefault(); e.stopPropagation();
    active = e.key === 'Home' ? 0 : e.key === 'End' ? tabs.length - 1 : (active + (e.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
    document.getElementById(`ribbon-tab-${active}`)?.focus();
  }
</script>

<div class="ribbon">
  <div class="tab-row">
  <div class="tabs" role="tablist" tabindex="-1" aria-label={t('功能區')} onkeydown={moveTab}>
    {#each tabs as tab, i}
      <button role="tab" id={`ribbon-tab-${i}`} aria-selected={active === i} aria-controls={`ribbon-panel-${i}`} tabindex={active === i ? 0 : -1}
        onpointerdown={(e) => e.preventDefault()} onclick={() => active = i}>{t(tab)}</button>
    {/each}
  </div>  <div class="quick" aria-label={t('快速存取工具列')}>
    {#each [{action:'save',label:'儲存',path:'M4 3h13l3 3v15H3V3h1Zm3 0v6h9V3M7 21v-8h10v8'}, {action:'undo',label:'復原',path:'M8 5 3 10l5 5M3 10h10a7 7 0 0 1 0 14'}, {action:'redo',label:'重做',path:'m16 5 5 5-5 5m5-5H11a7 7 0 0 0 0 14'}] as item}
      <button title={t(item.label)} aria-label={t(item.label)} disabled={item.action === 'undo' ? !canUndo : item.action === 'redo' ? !canRedo : false}
        onpointerdown={(e) => e.preventDefault()} onclick={() => onaction(item.action as MenuAction)}>
        <svg viewBox="0 0 24 26" width="18" height="18" aria-hidden="true"><path d={item.path} fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
      </button>
    {/each}
  </div>
  </div>

  {#each tabs as tab, i}
    <div role="tabpanel" id={`ribbon-panel-${i}`} aria-labelledby={`ribbon-tab-${i}`} hidden={active !== i}>
      {#if i === 0}{@render children()}
      {:else}
        <div class="commands" role="toolbar" aria-label={t(tab)}>
          {#each panels[i] as item}
            <button onpointerdown={(e) => e.preventDefault()} onclick={() => onaction(item.action)}><span aria-hidden="true">{item.icon}</span>{t(item.label)}</button>
          {/each}
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .ribbon { flex-shrink:0; background:var(--chrome-bg); color:var(--chrome-text); border-bottom:1px solid var(--chrome-border); position:relative; z-index:25; }
  .quick { display:flex; align-items:center; gap:4px; padding:0 10px; flex-shrink:0; }
  button { color:inherit; font:inherit; border:1px solid transparent; background:transparent; border-radius:4px; cursor:default; }
  button:hover:not(:disabled) { background:var(--popup-active); }
  button:focus-visible { outline:2px solid var(--accent); outline-offset:-2px; }
  button:disabled { opacity:.35; }
  .quick button { width:30px; height:28px; display:grid; place-items:center; }
  .tab-row { display:flex; align-items:center; }
  .tabs { flex:1; min-width:0; display:flex; gap:6px; padding:0 12px; overflow-x:auto; }
  .tabs button { position:relative; flex-shrink:0; border-radius:0; padding:7px 12px 9px; font-size:13px; }
  .tabs button[aria-selected='true'] { font-weight:650; }
  .tabs button[aria-selected='true']::after { content:''; position:absolute; bottom:2px; left:12px; right:12px; height:3px; background:var(--accent); border-radius:3px; }
  [hidden] { display:none; }
  .commands { min-height:88px; display:flex; align-items:center; flex-wrap:wrap; padding:4px 12px; gap:8px; }
  .commands button { display:flex; flex-direction:column; align-items:center; gap:10px; min-width:90px; padding:10px; font-size:12px; }
  .commands button span { font-size:26px; line-height:28px; color:var(--accent); }
</style>
