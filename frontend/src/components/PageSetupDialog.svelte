<script lang="ts">
  // 檔案 › 版面設定…: paper, orientation, scaling, margins, header and
  // footer, print area and options.
  import { t } from '../i18n';
  import type { PageSetupDTO, RangeDTO } from '../bridge/sheet';
  import { colName } from '../grid/layout';

  interface Props {
    initial: PageSetupDTO;
    // The current selection, offered as the print area.
    selection: RangeDTO;
    onapply: (p: PageSetupDTO) => Promise<string | null>;
    onclose: () => void;
  }

  let { initial, selection, onapply, onclose }: Props = $props();

  let p = $state<PageSetupDTO>({} as PageSetupDTO);
  let fit = $state(false);
  let error = $state<string | null>(null);
  $effect.pre(() => {
    p = { ...initial };
    fit = initial.fitWide > 0 || initial.fitTall > 0;
  });

  const ref = (r: RangeDTO) => `${colName(r.c0)}${r.r0 + 1}:${colName(r.c1)}${r.r1 + 1}`;
  // Margins are stored in inches and shown in centimeters.
  const cm = (inch: number) => Math.round(inch * 2.54 * 10) / 10;
  const inch = (v: number) => v / 2.54;

  async function apply(): Promise<void> {
    const out = { ...p };
    if (fit) {
      out.fitWide = Math.max(0, out.fitWide || 0);
      out.fitTall = Math.max(0, out.fitTall || 0);
      if (out.fitWide === 0 && out.fitTall === 0) out.fitWide = 1;
    } else {
      out.fitWide = 0;
      out.fitTall = 0;
    }
    error = await onapply(out);
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label={t('版面設定')} style="width: 480px">
    <h2>{t('版面設定')}</h2>
    <div class="grid2">
      <label>
        {t('紙張')}
        <select bind:value={p.paper}>
          {#each ['A4', 'A3', 'B5', 'Letter', 'Legal'] as s (s)}<option value={s}>{s}</option>{/each}
        </select>
      </label>
      <label>
        {t('方向')}
        <select bind:value={p.landscape}>
          <option value={false}>{t('直向')}</option>
          <option value={true}>{t('橫向')}</option>
        </select>
      </label>
    </div>
    <fieldset>
      <legend>{t('縮放比例')}</legend>
      <label class="check"><input type="radio" name="fit" value={false} bind:group={fit} /> {t('調整成')} <input class="num" type="number" min="10" max="400" bind:value={p.scale} disabled={fit} /> {t('% 正常大小')}</label>
      <label class="check">
        <input type="radio" name="fit" value={true} bind:group={fit} /> {t('調整成')}
        <input class="num" type="number" min="0" bind:value={p.fitWide} disabled={!fit} /> {t('頁寬')}
        <input class="num" type="number" min="0" bind:value={p.fitTall} disabled={!fit} /> {t('頁高（0 = 不限）')}
      </label>
    </fieldset>
    <fieldset>
      <legend>{t('邊界（公分）')}</legend>
      <div class="grid3">
        <label>{t('上')} <input class="num" type="number" step="0.1" value={cm(p.top)} oninput={(e) => (p.top = inch(+e.currentTarget.value))} /></label>
        <label>{t('下')} <input class="num" type="number" step="0.1" value={cm(p.bottom)} oninput={(e) => (p.bottom = inch(+e.currentTarget.value))} /></label>
        <label>{t('頁首')} <input class="num" type="number" step="0.1" value={cm(p.header)} oninput={(e) => (p.header = inch(+e.currentTarget.value))} /></label>
        <label>{t('左')} <input class="num" type="number" step="0.1" value={cm(p.left)} oninput={(e) => (p.left = inch(+e.currentTarget.value))} /></label>
        <label>{t('右')} <input class="num" type="number" step="0.1" value={cm(p.right)} oninput={(e) => (p.right = inch(+e.currentTarget.value))} /></label>
        <label>{t('頁尾')} <input class="num" type="number" step="0.1" value={cm(p.footer)} oninput={(e) => (p.footer = inch(+e.currentTarget.value))} /></label>
      </div>
      <div class="grid2">
        <label class="check"><input type="checkbox" bind:checked={p.centerH} /> {t('水平置中')}</label>
        <label class="check"><input type="checkbox" bind:checked={p.centerV} /> {t('垂直置中')}</label>
      </div>
    </fieldset>
    <label>{t('頁首')} <input type="text" bind:value={p.headerText} placeholder={t('&A（工作表名稱）')} /></label>
    <label>{t('頁尾')} <input type="text" bind:value={p.footerText} placeholder={t('第 &P 頁，共 &N 頁')} /></label>
    <div class="hint">{t('代碼：&amp;P 頁碼、&amp;N 總頁數、&amp;D 日期、&amp;T 時間、&amp;F 檔名、&amp;A 工作表名稱')}</div>
    <fieldset>
      <legend>{t('工作表')}</legend>
      <div class="area">
        {t('列印範圍：')}<strong>{p.printArea ? ref(p.printArea) : t('整個工作表')}</strong>
        <button onclick={() => (p.printArea = { ...selection })}>{t('使用選取範圍 %s', ref(selection))}</button>
        {#if p.printArea}<button onclick={() => (p.printArea = null)}>{t('清除')}</button>{/if}
      </div>
      <div class="grid2">
        <label class="check"><input type="checkbox" bind:checked={p.gridlines} /> {t('列印格線')}</label>
        <label class="check"><input type="checkbox" bind:checked={p.headings} /> {t('列印列與欄標題')}</label>
      </div>
    </fieldset>
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="dlg-actions">
      <button onclick={onclose}>{t('取消')}</button>
      <button class="primary" onclick={() => void apply()}>{t('確定')}</button>
    </div>
  </div>
</div>

<style>
  .grid2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px 16px;
  }
  .grid3 {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 6px 12px;
  }
  label {
    justify-content: space-between;
  }
  label.check {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
  label input[type='text'] {
    width: 320px;
  }
  .num {
    width: 56px;
    padding: 2px 4px;
    font: inherit;
    color: var(--chrome-text);
    background: var(--input-bg);
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
  }
  .hint {
    color: var(--chrome-muted);
    font-size: 11px;
  }
  .area {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .error {
    color: var(--error-fg);
  }
</style>
