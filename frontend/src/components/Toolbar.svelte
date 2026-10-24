<script lang="ts">
  // The 常用 (Home) toolbar. Buttons keep keyboard focus on the sheet:
  // pointerdown is prevented and selects hand focus back after a change.
  import { t } from '../i18n';
  import type { FormatPatch, StyleDTO } from '../bridge/sheet';
  import { FONT_CHOICES, fontFamilies, fontLabel } from '../stores/fonts';
  import ColorPicker from './ColorPicker.svelte';

  interface Props {
    style: StyleDTO | null;
    merged: boolean;
    onformat: (patch: FormatPatch) => void;
    onmerge: (center: boolean) => void;
    onclear: (mode: 'contents' | 'formats' | 'all') => void;
    onnumberdialog: () => void;
  }

  let { style, merged, onformat, onmerge, onclear, onnumberdialog }: Props = $props();

  const sizes = [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48, 72];
  const numberFormats: { label: string; code: string }[] = [
    { label: t('通用格式'), code: '' },
    { label: t('數值'), code: '0.00' },
    { label: t('貨幣'), code: '"$"#,##0.00_);[Red]\\("$"#,##0.00\\)' },
    { label: t('簡短日期'), code: 'yyyy/m/d' },
    { label: t('完整日期'), code: 'yyyy"年"m"月"d"日"' },
    { label: t('民國日期'), code: '[$-404]e/m/d' },
    { label: t('時間'), code: 'h:mm:ss' },
    { label: t('百分比'), code: '0.00%' },
    { label: t('科學記號'), code: '0.00E+00' },
    { label: t('文字'), code: '@' },
  ];
  const borders: { label: string; value: string }[] = [
    { label: t('下框線'), value: 'bottom' },
    { label: t('上框線'), value: 'top' },
    { label: t('左框線'), value: 'left' },
    { label: t('右框線'), value: 'right' },
    { label: t('無框線'), value: 'none' },
    { label: t('所有框線'), value: 'all' },
    { label: t('外框線'), value: 'outside' },
    { label: t('粗外框線'), value: 'thickOutside' },
    { label: t('粗下框線'), value: 'thickBottom' },
    { label: t('雙下框線'), value: 'doubleBottom' },
  ];

  let open = $state<'font' | 'fill' | 'border' | 'clear' | null>(null);
  let lastFont = $state('C00000');
  let lastFill = $state('FFFF00');
  let lastBorder = $state('bottom');

  let font = $derived(style?.font.name || 'Calibri');
  let size = $derived(style?.font.size || 11);
  let alignH = $derived(style?.alignH ?? '');
  let alignV = $derived(style?.alignV ?? '');
  let numberLabel = $derived(numberFormats.find((f) => f.code === (style?.numFmt ?? ''))?.label ?? t('自訂'));

  const keep = (e: PointerEvent) => e.preventDefault();

  function toggleMenu(name: NonNullable<typeof open>): void {
    open = open === name ? null : name;
  }

  function align(h: string): void {
    onformat({ alignH: alignH === h ? '' : h, label: t('對齊方式') });
  }

  function valign(v: string): void {
    onformat({ alignV: alignV === v ? '' : v, label: t('對齊方式') });
  }
</script>

<svelte:window onpointerdown={(e) => open && !(e.target as Element).closest?.('.menu-host') && (open = null)} />

<div class="toolbar" role="toolbar" aria-label={t('常用')}>
  <select
    class="font"
    tabindex="-1"
    value={font}
    title={t('字型')}
    onchange={(e) => onformat({ fontName: e.currentTarget.value, label: t('字型') })}
  >
    {#if !FONT_CHOICES.some((f) => f.value === font)}<option value={font}>{fontLabel(font)}</option>{/if}
    {#each FONT_CHOICES as f (f.value)}<option value={f.value} style:font-family={fontFamilies(f.value).map((n) => `"${n}"`).join(', ')}>{fontLabel(f.value)}</option>{/each}
  </select>
  <select
    class="size"
    tabindex="-1"
    value={size}
    title={t('字型大小')}
    onchange={(e) => onformat({ fontSize: Number(e.currentTarget.value), label: t('字型大小') })}
  >
    {#if !sizes.includes(size)}<option value={size}>{size}</option>{/if}
    {#each sizes as s (s)}<option value={s}>{s}</option>{/each}
  </select>

  <div class="group">
    <button class="b" class:on={style?.font.bold} tabindex="-1" title={t('粗體 (⌘B)')} onpointerdown={keep}
      onclick={() => onformat({ bold: !style?.font.bold, label: t('粗體') })}>B</button>
    <button class="i" class:on={style?.font.italic} tabindex="-1" title={t('斜體 (⌘I)')} onpointerdown={keep}
      onclick={() => onformat({ italic: !style?.font.italic, label: t('斜體') })}>I</button>
    <button class="u" class:on={style?.font.underline} tabindex="-1" title={t('底線 (⌘U)')} onpointerdown={keep}
      onclick={() => onformat({ underline: !style?.font.underline, label: t('底線') })}>U</button>
    <button class="s" class:on={style?.font.strike} tabindex="-1" title={t('刪除線')} onpointerdown={keep}
      onclick={() => onformat({ strike: !style?.font.strike, label: t('刪除線') })}>S</button>
  </div>

  <div class="group">
    <div class="menu-host">
      <button class="split" tabindex="-1" title={t('字型色彩')} onpointerdown={keep} onclick={() => onformat({ fontColor: lastFont, label: t('字型色彩') })}>
        <span class="letter">A</span><span class="bar" style:background="#{lastFont}"></span>
      </button><button class="caret" tabindex="-1" title={t('選擇字型色彩')} onpointerdown={keep} onclick={() => toggleMenu('font')}>▾</button>
      {#if open === 'font'}
        <ColorPicker resetLabel={t('自動')} onclose={() => (open = null)}
          onpick={(c) => { if (c) lastFont = c; onformat({ fontColor: c, label: t('字型色彩') }); }} />
      {/if}
    </div>
    <div class="menu-host">
      <button class="split" tabindex="-1" title={t('填滿色彩')} onpointerdown={keep} onclick={() => onformat({ fill: lastFill, label: t('填滿色彩') })}>
        <span class="bucket">▧</span><span class="bar" style:background="#{lastFill}"></span>
      </button><button class="caret" tabindex="-1" title={t('選擇填滿色彩')} onpointerdown={keep} onclick={() => toggleMenu('fill')}>▾</button>
      {#if open === 'fill'}
        <ColorPicker resetLabel={t('無填滿')} onclose={() => (open = null)}
          onpick={(c) => { if (c) lastFill = c; onformat({ fill: c, label: t('填滿色彩') }); }} />
      {/if}
    </div>
    <div class="menu-host">
      <button class="split" tabindex="-1" title={t('框線')} onpointerdown={keep} onclick={() => onformat({ border: lastBorder, label: t('框線') })}>
        <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
          <rect x="2" y="2" width="12" height="12" fill="none" stroke="currentColor" stroke-dasharray="1.5 1.5" />
          <line x1="2" y1="14" x2="14" y2="14" stroke="currentColor" stroke-width="2" />
        </svg>
      </button><button class="caret" tabindex="-1" title={t('選擇框線')} onpointerdown={keep} onclick={() => toggleMenu('border')}>▾</button>
      {#if open === 'border'}
        <ul class="menu" role="menu">
          {#each borders as b (b.value)}
            <li><button role="menuitem" tabindex="-1" onpointerdown={keep}
                onclick={() => { lastBorder = b.value; onformat({ border: b.value, label: t('框線') }); open = null; }}>{b.label}</button></li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>

  <div class="group">
    {#each [['left', t('靠左對齊')], ['center', t('置中')], ['right', t('靠右對齊')]] as [h, label] (h)}
      <button class:on={alignH === h} tabindex="-1" title={label} onpointerdown={keep} onclick={() => align(h)} aria-label={label}>
        <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
          {#each [3, 6.5, 10, 13.5] as y, i (y)}
            {@const w = i % 2 ? 8 : 12}
            {@const x = h === 'left' ? 2 : h === 'right' ? 14 - w : 8 - w / 2}
            <line x1={x} y1={y} x2={x + w} y2={y} stroke="currentColor" stroke-width="1.5" />
          {/each}
        </svg>
      </button>
    {/each}
    {#each [['top', t('靠上對齊')], ['center', t('置中對齊')], ['bottom', t('靠下對齊')]] as [v, label] (v)}
      <button class:on={alignV === v || (v === 'bottom' && alignV === '')} tabindex="-1" title={label} onpointerdown={keep}
        onclick={() => valign(v === 'bottom' ? '' : v)} aria-label={label}>
        <svg viewBox="0 0 16 16" width="15" height="15" aria-hidden="true">
          <line x1="2" y1={v === 'top' ? 2 : v === 'center' ? 8 : 14} x2="14" y2={v === 'top' ? 2 : v === 'center' ? 8 : 14} stroke="currentColor" stroke-width="1" />
          <rect x="5" y={v === 'top' ? 4 : v === 'center' ? 5 : 6} width="6" height="6" fill="currentColor" />
        </svg>
      </button>
    {/each}
    <button class="text" class:on={style?.wrap} tabindex="-1" title={t('自動換行')} onpointerdown={keep}
      onclick={() => onformat({ wrap: !style?.wrap, label: t('自動換行') })}>{t('自動換行')}</button>
    <button class="text" class:on={merged} tabindex="-1" title={merged ? t('取消合併儲存格') : t('跨欄置中')} onpointerdown={keep}
      onclick={() => onmerge(true)}>{t('跨欄置中')}</button>
  </div>

  <div class="group">
    <select class="numfmt" tabindex="-1" title={t('數值格式')} value={numberLabel}
      onchange={(e) => {
        const label = e.currentTarget.value;
        if (label === t('其他…')) onnumberdialog();
        else onformat({ numFmt: numberFormats.find((f) => f.label === label)?.code ?? '', label: t('數值格式') });
      }}>
      {#if numberLabel === t('自訂')}<option>{t('自訂')}</option>{/if}
      {#each numberFormats as f (f.label)}<option>{f.label}</option>{/each}
      <option>{t('其他…')}</option>
    </select>
    <button class="text" tabindex="-1" title={t('百分比樣式')} onpointerdown={keep} onclick={() => onformat({ numFmt: '0%', label: t('百分比樣式') })}>%</button>
    <button class="text" tabindex="-1" title={t('千分位樣式')} onpointerdown={keep} onclick={() => onformat({ numFmt: '#,##0.00', label: t('千分位樣式') })}>,</button>
    <button class="text" tabindex="-1" title={t('增加小數位數')} onpointerdown={keep} onclick={() => onformat({ decimals: 1, label: t('增加小數位數') })}>.0→.00</button>
    <button class="text" tabindex="-1" title={t('減少小數位數')} onpointerdown={keep} onclick={() => onformat({ decimals: -1, label: t('減少小數位數') })}>.00→.0</button>
  </div>

  <div class="group">
    <div class="menu-host">
      <button class="text" tabindex="-1" title={t('清除')} onpointerdown={keep} onclick={() => toggleMenu('clear')}>{t('清除 ▾')}</button>
      {#if open === 'clear'}
        <ul class="menu" role="menu">
          <li><button role="menuitem" tabindex="-1" onpointerdown={keep} onclick={() => { onclear('all'); open = null; }}>{t('全部清除')}</button></li>
          <li><button role="menuitem" tabindex="-1" onpointerdown={keep} onclick={() => { onclear('formats'); open = null; }}>{t('清除格式')}</button></li>
          <li><button role="menuitem" tabindex="-1" onpointerdown={keep} onclick={() => { onclear('contents'); open = null; }}>{t('清除內容')}</button></li>
        </ul>
      {/if}
    </div>
  </div>
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 36px;
    padding: 0 8px;
    background: var(--chrome-bg);
    border-bottom: 1px solid var(--chrome-border);
    font-size: 12px;
    /* Wrap instead of scrolling: overflow other than visible would clip
       the color palettes and menus that drop down from the toolbar. */
    flex-wrap: wrap;
    flex-shrink: 0;
    position: relative;
    z-index: 25;
  }
  .group {
    display: flex;
    align-items: center;
    gap: 2px;
    padding-left: 10px;
    border-left: 1px solid var(--chrome-border);
  }
  select {
    height: 24px;
    font: inherit;
    color: var(--chrome-text);
    background: var(--grid-bg);
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
  }
  .font {
    width: 120px;
  }
  .size {
    width: 52px;
  }
  .numfmt {
    width: 96px;
  }
  button {
    min-width: 26px;
    height: 26px;
    padding: 0 5px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 1px;
    font: inherit;
    color: var(--chrome-text);
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: default;
    white-space: nowrap;
  }
  button:hover {
    background: var(--app-bg);
    border-color: var(--chrome-border);
  }
  button.on {
    background: var(--header-active-bg);
    border-color: var(--accent);
    color: var(--header-active-text);
  }
  .b {
    font-weight: 700;
  }
  .i {
    font-style: italic;
    font-family: Georgia, serif;
  }
  .u {
    text-decoration: underline;
  }
  .s {
    text-decoration: line-through;
  }
  .split {
    flex-direction: column;
    min-width: 22px;
    padding: 0 3px;
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
  }
  .caret {
    min-width: 14px;
    padding: 0 2px;
    font-size: 9px;
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
  }
  .letter {
    font-weight: 700;
    line-height: 13px;
  }
  .bucket {
    line-height: 13px;
  }
  .bar {
    width: 16px;
    height: 4px;
    border-radius: 1px;
  }
  .menu-host {
    position: relative;
    display: inline-flex;
  }
  .menu {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 30;
    margin: 4px 0 0;
    padding: 4px 0;
    list-style: none;
    min-width: 120px;
    background: var(--popup-bg);
    border: 1px solid var(--popup-border);
    border-radius: 8px;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.2);
  }
  .menu button {
    width: 100%;
    justify-content: flex-start;
    border-radius: 0;
    padding: 0 12px;
  }
  .menu button:hover {
    background: var(--popup-active);
    border-color: transparent;
  }
</style>
