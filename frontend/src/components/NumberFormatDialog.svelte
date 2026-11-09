<script lang="ts">
  import { t } from '../i18n';
  import { onMount, tick, untrack } from 'svelte';
  import type { FormatPatch, StyleDTO } from '../bridge/sheet';
  import { FONT_CHOICES, fontLabel, fontFamilies } from '../stores/fonts';
  import { createFormatDraft, formatDraftPatch, type EditableFormatField, type CellFormatDraft } from './cellFormatDraft';
  import ColorPicker from './ColorPicker.svelte';
  import BorderEditor from './BorderEditor.svelte';
  import { initialBorderBrush } from './borderDraft';
  import FillPreview from './FillPreview.svelte';
  import { validCellFill } from '../grid/cellFills';
  import DiagonalPreview from './DiagonalPreview.svelte';
  import AdvancedFillEditor from './AdvancedFillEditor.svelte';
  import { NUMBER_FORMAT_CATEGORIES, numberFormatCategory } from './numberFormatCategories';
  import type { Range } from '../grid/selection';

  interface Props {
    style: StyleDTO | null;
    mixed?: string[];
    borderOriginal?: FormatPatch['borders'];
    selection: Range[];
    merges: Range[];
    preview: (code: string) => Promise<string>;
    onapply: (patch: FormatPatch) => void;
    onclose: () => void;
  }
  let { style, mixed = [], borderOriginal, selection, merges, preview: render, onapply, onclose }: Props = $props();
  // Capture a private snapshot once per opening, independent of later selection updates.
  const initialBrush = untrack(() => initialBorderBrush(style, borderOriginal, mixed));
  const initial = untrack(() => ({ ...createFormatDraft(style), borderColor: initialBrush.color, borderStyle: initialBrush.style }));
  let borderColorEdited = $state(false);
  let borderStyleEdited = $state(false);
  let borderReset = $state(0);
  let draft = $state({ ...initial });
  const mixedFields = untrack(() => [...mixed]);
  const mixedValue = '__mixed__';
  let edited = $state<Partial<Record<EditableFormatField, boolean>>>({});
  const colorOnly = $derived((borderColorEdited || borderStyleEdited) && Object.keys(draft.borders).length === 0 && !edited.diagonal && !edited.diagonalUp && !edited.diagonalDown);
  const previewEdges = $derived(colorOnly && borderOriginal ? Object.fromEntries(Object.entries(borderOriginal).map(([key,line])=>[key,line?.style ? {...line,...(borderColorEdited?{color:draft.borderColor}:{}),...(borderStyleEdited?{style:draft.borderStyle}:{})} : line])) : borderOriginal);
  const previewStyle = $derived(colorOnly && style ? {...style,...Object.fromEntries(['top','bottom','left','right'].map(key=>{const line=style[key as 'top'];return [key,line.style?{...line,...(borderColorEdited?{color:draft.borderColor}:{}),...(borderStyleEdited?{style:draft.borderStyle}:{})}:line]}))} : style);
  function isMixed(key: EditableFormatField): boolean { return mixedFields.includes(key) && !edited[key]; }
  function change<K extends EditableFormatField>(key: K, value: CellFormatDraft[K]): void {
    draft[key] = value;
    if (key === 'fillSpec') edited.fill=true;
    if (key === 'fill') {draft.fillSpec={};edited.fillSpec=true;}
    edited[key] = true;
  }
  function selectValue(key: 'fontName' | 'alignH' | 'alignV', value: string): void {
    if (value === mixedValue) { draft[key] = initial[key]; delete edited[key]; }
    else change(key, value);
  }
  function mixedCheck(node: HTMLInputElement, value: boolean) {
    node.indeterminate = value;
    return { update(value: boolean) { node.indeterminate = value; } };
  }
  const checks = [['bold', t('粗體')], ['italic', t('斜體')], ['underline', t('底線')], ['strike', t('刪除線')]] as const;
  const fontFields: EditableFormatField[] = ['fontName', 'fontSize', 'bold', 'italic', 'underline', 'strike', 'fontColor'];
  const tabs = ['數值', '字型', '對齊', '框線', '填滿'];
  let tab = $state(0);
  let sizeValid = $derived(!edited.fontSize || (Number.isFinite(draft.fontSize) && draft.fontSize >= 1 && draft.fontSize <= 409));
  let fillValid = $derived(!edited.fillSpec || validCellFill(draft.fillSpec));
  let palette = $state<'fontColor' | 'fill' | 'borderColor' | null>(null);
  const dropdownPalette = $derived(palette === 'borderColor' || palette === 'fill');
  let paletteTrigger: HTMLButtonElement | null = null;
  let paletteElement = $state<HTMLDivElement>();
  let palettePosition = $state({ left: 0, top: 0 });
  let preview = $state('');
  let previewError = $state(false);
  let dialog: HTMLFormElement;
  let input = $state<HTMLInputElement>();
  const sizes = [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48, 72];
  let numberCategory = $state<string | null>(untrack(() => mixed.includes('numFmt') ? null : numberFormatCategory(initial.numFmt)));
  const currentCategory = $derived(NUMBER_FORMAT_CATEGORIES.find(category => category.id === numberCategory));
  function chooseNumberCategory(id: string): void {
    numberCategory=id;
    const types=NUMBER_FORMAT_CATEGORIES.find(category => category.id === id)?.types ?? [];
    if (types.length && (isMixed('numFmt') || !types.some(type => type.code === draft.numFmt))) change('numFmt',types[0].code);
  }
  function editNumberCode(code: string): void {
    change('numFmt',code);
    numberCategory=numberFormatCategory(code);
  }


  onMount(() => {
    const previous = document.activeElement;
    input?.focus();
    input?.select();
    return () => { if (previous instanceof HTMLElement && previous.isConnected) previous.focus(); };
  });

  $effect(() => {
    const code = draft.numFmt;
    if (isMixed('numFmt')) { preview = t('混合格式'); return; }
    let current = true;
    previewError = false;
    render(code).then((value) => {
      if (current) preview = value;
    }).catch(() => { if (current) { preview = ''; previewError = true; } });
    return () => { current = false; };
  });

  function apply(): void {
    if (!sizeValid || !fillValid) return;
    const patch = formatDraftPatch(initial, draft, { mixed: mixedFields, borderColorEdited, borderColorMixed: initialBrush.mixed, borderStyleEdited, borderStyleMixed: initialBrush.styleMixed, edited: Object.keys(edited).filter(key => edited[key as EditableFormatField]) });
    if (Object.keys(patch).length) onapply({ ...patch, label: t('儲存格格式') });
    else onclose();
  }

  async function switchTab(index: number, focus = true): Promise<void> {
    tab = index;
    palette = null;
    if (focus) {
      await tick();
      dialog.querySelector<HTMLButtonElement>(`#cell-format-tab-${index}`)?.focus();
    }
  }

  async function togglePalette(name: NonNullable<typeof palette>, trigger: HTMLButtonElement): Promise<void> {
    paletteTrigger = trigger;
    palette = palette === name ? null : name;
    if (dropdownPalette) {
      const rect = trigger.getBoundingClientRect();
      palettePosition = { left: rect.left, top: rect.bottom + 4 };
      await tick();
      if (palette !== name || !paletteElement) return;
      const popup = paletteElement.getBoundingClientRect();
      palettePosition = {
        left: Math.max(8, Math.min(rect.left, window.innerWidth - popup.width - 8)),
        top: Math.max(8, rect.bottom + popup.height + 12 <= window.innerHeight ? rect.bottom + 4 : rect.top - popup.height - 4),
      };
    }
  }

  async function closePalette(): Promise<void> {
    palette = null;
    await tick();
    if (paletteTrigger?.isConnected) paletteTrigger.focus();
  }

  function keydown(e: KeyboardEvent): void {
    // Contain shortcuts and focus inside the modal, including the color palette.
    e.stopPropagation();
    if (e.key === 'Escape') {
      e.preventDefault();
      if (palette) void closePalette();
      else onclose();
    }
    if (e.key === 'Tab') {
      const controls = Array.from(dialog.querySelectorAll<HTMLElement>('button:not([disabled]), input, select, [tabindex="0"]'))
        .filter(el => el.tabIndex >= 0 && el.getClientRects().length > 0);
      const first = controls[0], last = controls.at(-1);
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last?.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus(); }
    }
  }
</script>

<svelte:window onresize={() => { if (dropdownPalette) palette = null; }} onpointerdown={(e) => {
  if (dropdownPalette && e.target instanceof Node && !paletteElement?.contains(e.target) && !paletteTrigger?.contains(e.target)) palette = null;
}} />

<div class="backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dialog" role="dialog" aria-modal="true" aria-label={t('儲存格格式')} tabindex="-1" onkeydown={keydown}>
  <form bind:this={dialog} onsubmit={(e) => { e.preventDefault(); apply(); }}>
    <h2>{t('儲存格格式')}</h2>
    {#if mixedFields.length}<p class="mixed-note">{t('混合格式僅套用您修改的項目。')}</p>{/if}
    <div class="tabs" role="tablist" aria-label={t('儲存格格式') }>
      {#each tabs as label, i}
        <button type="button" role="tab" id={`cell-format-tab-${i}`} aria-controls={`cell-format-panel-${i}`}
          aria-selected={tab === i} tabindex={tab === i ? 0 : -1} onclick={() => switchTab(i)}
          onkeydown={(e) => {
            const index = e.key === 'ArrowRight' ? (i + 1) % tabs.length : e.key === 'ArrowLeft' ? (i + tabs.length - 1) % tabs.length : e.key === 'Home' ? 0 : e.key === 'End' ? tabs.length - 1 : -1;
            if (index >= 0) { e.preventDefault(); void switchTab(index, true); }
          }}>{t(label)}</button>
      {/each}
    </div>
    <div class="scroll-body" onscroll={() => { if (dropdownPalette) palette = null; }}>
      <div class="panel" role="tabpanel" id={`cell-format-panel-${tab}`} aria-labelledby={`cell-format-tab-${tab}`} tabindex="0">
        {#if tab === 0}
          <div class="body">
            <ul class="list" aria-label={t('類別')}>
              {#each NUMBER_FORMAT_CATEGORIES as category (category.id)}
                <li><button type="button" data-number-category={category.id} class:selected={category.id === numberCategory} aria-pressed={category.id === numberCategory} onclick={() => chooseNumberCategory(category.id)}>{t(category.label)}</button></li>
              {/each}
            </ul>
            <div class="right">
              {#if currentCategory && currentCategory.types.length > 1}
                <label>{t('類型')}<select class="number-types" aria-label={t('類型')} size={Math.min(8,currentCategory.types.length)} value={draft.numFmt} onchange={e=>change('numFmt',e.currentTarget.value)}>
                  {#each currentCategory.types as type (type.code)}<option value={type.code}>{type.example ? `${type.example} — ${t(type.label)}` : t(type.label)}</option>{/each}
                </select></label>
              {/if}
              <label>{t('格式碼')}<input bind:this={input} value={isMixed('numFmt') ? '' : draft.numFmt} oninput={(e) => editNumberCode(e.currentTarget.value)} spellcheck="false" placeholder={isMixed('numFmt') ? t('混合') : t('通用格式')} /></label>
              <div class="preview" aria-live="polite"><span class="muted">{t('範例')}</span><span class="value">{previewError ? t('無法取得預覽') : preview}</span></div>
            </div>
          </div>
        {:else if tab === 1}
          <div class="fields">
            <label>{t('字型')}<select value={isMixed('fontName') ? mixedValue : draft.fontName} onchange={(e) => selectValue('fontName', e.currentTarget.value)}>
              {#if mixedFields.includes('fontName')}<option value={mixedValue}>{t('混合')}</option>{/if}
              {#if !FONT_CHOICES.some(f => f.value === draft.fontName)}<option value={draft.fontName}>{fontLabel(draft.fontName)}</option>{/if}
              {#each FONT_CHOICES as f}<option value={f.value}>{fontLabel(f.value)}</option>{/each}
            </select></label>
            <label>{t('字型大小')}<input type="number" min="1" max="409" step="any" required={!isMixed('fontSize')} placeholder={isMixed('fontSize') ? t('混合') : ''} list="cell-format-sizes" value={isMixed('fontSize') || !Number.isFinite(draft.fontSize) ? undefined : draft.fontSize} oninput={(e) => change('fontSize', e.currentTarget.valueAsNumber)} /></label>
            <datalist id="cell-format-sizes">{#each sizes as size}<option value={size}></option>{/each}</datalist>
          </div>
          <div class="checks">
            {#each checks as [key, label]}
              <label><input type="checkbox" use:mixedCheck={isMixed(key)} checked={!isMixed(key) && draft[key]} onchange={(e) => change(key, e.currentTarget.checked)} />{label}{#if isMixed(key)}<span class="muted">{t('混合')}</span>{/if}</label>
            {/each}
          </div>
          <button type="button" onclick={(e) => togglePalette('fontColor', e.currentTarget)}>{t('字型色彩')} <span class="swatch" class:mixed-swatch={isMixed('fontColor')} style:background={isMixed('fontColor') ? undefined : draft.fontColor ? `#${draft.fontColor}` : 'currentColor'}></span> {isMixed('fontColor') ? t('混合') : draft.fontColor ? `#${draft.fontColor}` : t('自動')}</button>
          {#if fontFields.some(isMixed)}<div class="preview">{t('混合格式')}</div>{:else}
          <div class="preview font-preview" style:font-family={fontFamilies(draft.fontName).map(n => `"${n}"`).join(', ')} style:font-size={`${Math.min(draft.fontSize || 11, 48)}pt`}
            style:font-weight={draft.bold ? 'bold' : 'normal'} style:font-style={draft.italic ? 'italic' : 'normal'}
            style:text-decoration={`${draft.underline ? 'underline' : ''} ${draft.strike ? 'line-through' : ''}`}
            style:color={draft.fontColor ? `#${draft.fontColor}` : 'var(--chrome-text)'}>{t('範例')} AaBb 123</div>
          {/if}
        {:else if tab === 2}
          <div class="fields">
            <label>{t('水平對齊')}<select value={isMixed('alignH') ? mixedValue : draft.alignH} onchange={(e) => selectValue('alignH', e.currentTarget.value)}>
              {#if mixedFields.includes('alignH')}<option value={mixedValue}>{t('混合')}</option>{/if}
              <option value="">{t('通用格式')}</option><option value="left">{t('靠左對齊')}</option><option value="center">{t('置中')}</option><option value="right">{t('靠右對齊')}</option>
              {#if !['', 'left', 'center', 'right'].includes(draft.alignH)}<option value={draft.alignH}>{draft.alignH}</option>{/if}
            </select></label>
            <label>{t('垂直對齊')}<select value={isMixed('alignV') ? mixedValue : draft.alignV} onchange={(e) => selectValue('alignV', e.currentTarget.value)}>
              {#if mixedFields.includes('alignV')}<option value={mixedValue}>{t('混合')}</option>{/if}
              <option value="">{t('靠下對齊')}</option><option value="top">{t('靠上對齊')}</option><option value="center">{t('置中對齊')}</option><option value="bottom">{t('靠下對齊')}</option>
              {#if !['', 'top', 'center', 'bottom'].includes(draft.alignV)}<option value={draft.alignV}>{draft.alignV}</option>{/if}
            </select></label>
          </div>
          <label class="check"><input type="checkbox" use:mixedCheck={isMixed('wrap')} checked={!isMixed('wrap') && draft.wrap} onchange={(e) => change('wrap', e.currentTarget.checked)} />{t('自動換行')}{#if isMixed('wrap')}<span class="muted">{t('混合')}</span>{/if}</label>
        {:else if tab === 3}
          <BorderEditor bind:value={draft.borders} bind:lineStyle={draft.borderStyle} color={draft.borderColor} original={previewStyle} originalEdges={previewEdges} mixedEdges={mixedFields} {selection} {merges} resetKey={borderReset} styleMixed={!borderStyleEdited && initialBrush.styleMixed} onstylechange={()=>borderStyleEdited=true} onleave={()=>{borderColorEdited=false;borderStyleEdited=false;draft.borderColor=initial.borderColor;draft.borderStyle=initial.borderStyle;}} onclear={()=>{change('diagonalUp',false);change('diagonalDown',false);change('diagonal',{});}} />
          <div class="diagonal-layout">
          <div class="diagonal-controls">
            {#each ['diagonalUp','diagonalDown'] as key}
              <label class="check"><input type="checkbox" aria-label={key==='diagonalUp'?t('斜線 ╱'):t('斜線 ╲')} use:mixedCheck={isMixed(key as 'diagonalUp')} checked={!isMixed(key as 'diagonalUp') && draft[key as 'diagonalUp']} onchange={e=>{change(key as 'diagonalUp',e.currentTarget.checked);if(e.currentTarget.checked)change('diagonal',{style:draft.borderStyle,color:draft.borderColor});}}/>{key==='diagonalUp'?t('斜線 ╱'):t('斜線 ╲')}</label>
            {/each}
            <button type="button" onclick={()=>{change('diagonal',{style:draft.borderStyle,color:draft.borderColor});}}>{t('套用斜線線型／色彩')}</button>
          </div>
          <DiagonalPreview line={colorOnly ? {...draft.diagonal,...(borderColorEdited?{color:draft.borderColor}:{}),...(borderStyleEdited?{style:draft.borderStyle}:{})} : draft.diagonal} up={draft.diagonalUp} down={draft.diagonalDown} mixedUp={isMixed('diagonalUp')} mixedDown={isMixed('diagonalDown')} />
          </div>
          <div class="color-field">
            <span id="border-color-label">{t('線條顏色')}</span>
            <button class="color-select" type="button" aria-labelledby="border-color-label border-color-value" aria-haspopup="dialog" aria-expanded={palette === 'borderColor'} onclick={(e) => togglePalette('borderColor', e.currentTarget)} onkeydown={(e) => { if (e.key === 'ArrowDown') { e.preventDefault(); if (palette !== 'borderColor') void togglePalette('borderColor', e.currentTarget).then(() => paletteElement?.querySelector('button')?.focus()); } }}>
              <span class="swatch" class:mixed-swatch={!borderColorEdited && initialBrush.mixed} style:background={!borderColorEdited && initialBrush.mixed ? undefined : draft.borderColor ? `#${draft.borderColor}` : 'currentColor'}></span>
              <span id="border-color-value">{!borderColorEdited && initialBrush.mixed ? t('混合') : draft.borderColor ? `#${draft.borderColor}` : t('自動')}</span>
              <svg class="dropdown-arrow" width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="m3 5 4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.5" /></svg>
            </button>
          </div>
        {:else}
          <div class="color-field">
            <span id="fill-color-label">{t('填滿色彩')}</span>
            <button class="color-select" type="button" aria-labelledby="fill-color-label fill-color-value" aria-haspopup="dialog" aria-expanded={palette === 'fill'} onclick={(e) => togglePalette('fill', e.currentTarget)} onkeydown={(e) => { if (e.key === 'ArrowDown') { e.preventDefault(); if (palette !== 'fill') void togglePalette('fill', e.currentTarget).then(() => paletteElement?.querySelector('button')?.focus()); } }}>
              <span class="swatch" class:mixed-swatch={isMixed('fill')} style:background={isMixed('fill') ? undefined : draft.fill ? `#${draft.fill}` : 'transparent'}></span>
              <span id="fill-color-value">{isMixed('fill') ? t('混合') : draft.fill ? `#${draft.fill}` : t('無填滿')}</span>
              <svg class="dropdown-arrow" width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="m3 5 4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.5" /></svg>
            </button>
          </div>
          <AdvancedFillEditor value={draft.fillSpec} mixed={isMixed('fillSpec')} onchange={spec=>{if(spec.kind)change('fillSpec',spec);else change('fill',draft.fill);}} />
          <FillPreview color={draft.fill} spec={draft.fillSpec} mixed={isMixed('fill')} />
          {#if !fillValid}<p role="alert">{t('請檢查漸層角度、範圍與色彩節點。')}</p>{/if}
        {/if}
        {#if palette}
          <div bind:this={paletteElement} class="palette" class:dropdown-palette={dropdownPalette} style:left={dropdownPalette ? `${palettePosition.left}px` : undefined} style:top={dropdownPalette ? `${palettePosition.top}px` : undefined}>
            <ColorPicker tabindex={0} resetLabel={palette === 'fill' ? t('無填滿') : t('自動')}
              onpick={(color) => { if (palette === 'fill' || palette === 'fontColor') change(palette, color); else if (palette) { draft[palette] = color; borderColorEdited = true; } }} onclose={() => void closePalette()} />
          </div>
        {/if}
      </div>
    </div>
    <div class="actions"><button type="button" onclick={(e) => { draft = { ...initial, borders: {} }; edited = {}; borderColorEdited = false; borderStyleEdited = false; borderReset++; numberCategory = mixedFields.includes('numFmt') ? null : numberFormatCategory(initial.numFmt); palette = null; e.currentTarget.focus(); }}>{t('重設變更')}</button><button type="button" onclick={onclose}>{t('取消')}</button><button class="primary" type="submit" disabled={!sizeValid || !fillValid}>{t('確定')}</button></div>
  </form>
  </div>
</div>

<style>
  .backdrop { position:fixed; inset:0; z-index:50; display:flex; align-items:center; justify-content:center; background:rgb(0 0 0 / .25); }
  .dialog { width:min(600px, calc(100% - 32px)); max-height:calc(100dvh - 32px); box-sizing:border-box; display:flex; flex-direction:column; padding:16px; background:var(--chrome-bg); color:var(--chrome-text); border:1px solid var(--chrome-border); border-radius:10px; box-shadow:0 12px 40px rgb(0 0 0 / .3); font-size:13px; }
  h2 { margin:0 0 12px; font-size:14px; }
  form { display:flex; flex-direction:column; min-height:0; }
  .tabs { display:flex; flex-shrink:0; overflow-x:auto; border-bottom:1px solid var(--chrome-border); }
  .tabs button { white-space:nowrap; border:0; border-radius:4px 4px 0 0; padding:8px 16px; }
  .tabs [aria-selected="true"] { color:var(--accent); background:var(--popup-active); box-shadow:inset 0 -2px var(--accent); }
  .scroll-body { overflow:auto; min-height:0; }
  .panel { min-height:280px; padding:16px 2px; display:flex; flex-direction:column; align-items:stretch; gap:14px; }
  .body { display:flex; gap:16px; }
  .list { width:170px; max-height:280px; margin:0; padding:4px 0; overflow:auto; list-style:none; border:1px solid var(--chrome-border); border-radius:6px; flex-shrink:0; }
  .list button { width:100%; padding:4px 10px; text-align:left; border:0; background:none; }
  .list button.selected { background:var(--popup-active); }
  .number-types { min-height:100px; max-width:100%; }
  .number-types option { padding:5px 8px; }
  .right { flex:1; min-width:0; display:flex; flex-direction:column; gap:12px; }
  label { display:flex; flex-direction:column; gap:5px; }
  .fields { display:grid; grid-template-columns:2fr 1fr; gap:16px; }
  .checks { display:flex; flex-wrap:wrap; gap:16px; }
  .checks label, .check { flex-direction:row; align-items:center; }
  .diagonal-layout { display:flex; flex-wrap:wrap; align-items:flex-start; gap:12px 24px; }
  .diagonal-controls { display:flex; flex-direction:column; align-items:flex-start; gap:4px; }
  .color-field { display:flex; flex-direction:column; align-items:flex-start; gap:5px; }
  .color-select { display:flex; align-items:center; gap:8px; width:200px; max-width:100%; text-align:left; background:var(--input-bg); }
  .dropdown-arrow { margin-left:auto; }
  input, select { min-width:0; box-sizing:border-box; width:100%; padding:5px 8px; color:var(--chrome-text); background:var(--input-bg); border:1px solid var(--chrome-border); border-radius:4px; font:inherit; user-select:text; -webkit-user-select:text; }
  input[type="checkbox"] { width:auto; }
  .preview { display:flex; flex-direction:column; gap:4px; padding:8px; border:1px solid var(--chrome-border); border-radius:6px; overflow-wrap:anywhere; }
  .mixed-note { margin:0 0 8px; color:var(--chrome-muted); font-size:12px; }
  .mixed-swatch { background:repeating-conic-gradient(#ddd 0% 25%, #fff 0% 50%) 0 0 / 8px 8px; }
  .muted { color:var(--chrome-muted); font-size:11px; }
  .value { font-size:15px; min-height:20px; white-space:pre-wrap; }
  .font-preview { overflow:auto; max-height:130px; }
  .swatch { display:inline-block; width:16px; height:16px; vertical-align:middle; border:1px solid var(--chrome-border); }
  .palette { position:relative; min-height:230px; }
  .palette :global(.picker) { position:static; width:max-content; }
  .dropdown-palette { position:fixed; z-index:60; min-height:0; max-height:calc(100dvh - 16px); overflow:auto; }
  .dropdown-palette :global(.picker) { margin-top:0; }
  button { padding:5px 12px; font:inherit; color:inherit; background:var(--app-bg); border:1px solid var(--chrome-border); border-radius:5px; cursor:pointer; }
  button:focus-visible, input:focus-visible, select:focus-visible { outline:2px solid var(--accent); outline-offset:2px; }
  .actions { display:flex; justify-content:flex-end; flex-shrink:0; gap:8px; padding-top:12px; }
  .actions button { padding:4px 16px; }
  .actions .primary { color:white; background:var(--accent); border-color:var(--accent); }
  @media (max-width:480px) { .body { flex-direction:column; } .list { width:100%; max-height:130px; } .fields { grid-template-columns:1fr; } .tabs button { padding:8px 10px; } }
</style>
