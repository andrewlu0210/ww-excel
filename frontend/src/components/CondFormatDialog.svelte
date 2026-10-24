<script lang="ts">
  // 格式 › 設定格式化的條件… (new rule) and 管理規則… (list, delete).
  import { onMount } from 'svelte';
  import type { CFRuleDTO } from '../bridge/sheet';
  import { colName } from '../grid/layout';

  interface Props {
    manage: boolean;
    rules: () => Promise<CFRuleDTO[]>;
    onadd: (rule: CFRuleDTO) => Promise<string | null>;
    ondelete: (index: number) => Promise<string | null>;
    onclose: () => void;
  }

  let { manage, rules, onadd, ondelete, onclose }: Props = $props();

  let list = $state<CFRuleDTO[]>([]);
  let adding = $state(false);
  let error = $state<string | null>(null);
  let rule = $state<CFRuleDTO>(fresh());

  function fresh(): CFRuleDTO {
    return {
      ranges: [],
      type: 'cell',
      op: 'greaterThan',
      value1: '',
      value2: '',
      percent: false,
      below: false,
      color: '9C0006',
      fill: 'FFC7CE',
      bold: false,
      italic: false,
      colors: [],
      stopIfTrue: false,
    };
  }

  const kinds = [
    { value: 'cell', label: '儲存格的值' },
    { value: 'text', label: '包含特定文字' },
    { value: 'top', label: '前 N 項' },
    { value: 'bottom', label: '後 N 項' },
    { value: 'average', label: '高於／低於平均' },
    { value: 'duplicate', label: '重複的值' },
    { value: 'unique', label: '唯一的值' },
    { value: 'blanks', label: '空格' },
    { value: 'formula', label: '使用公式' },
    { value: '3_color_scale', label: '三色色階' },
    { value: '2_color_scale', label: '雙色色階' },
    { value: 'data_bar', label: '資料橫條' },
  ];
  const ops = [
    { value: 'greaterThan', label: '大於' },
    { value: 'lessThan', label: '小於' },
    { value: 'between', label: '介於' },
    { value: 'notBetween', label: '不介於' },
    { value: 'equal', label: '等於' },
    { value: 'notEqual', label: '不等於' },
    { value: 'greaterThanOrEqual', label: '大於或等於' },
    { value: 'lessThanOrEqual', label: '小於或等於' },
  ];
  const textOps = [
    { value: 'containsText', label: '包含' },
    { value: 'notContains', label: '不包含' },
    { value: 'beginsWith', label: '開頭是' },
    { value: 'endsWith', label: '結尾是' },
  ];
  // Excel's preset highlight formats.
  const presets = [
    { label: '淺紅色填滿與深紅色文字', color: '9C0006', fill: 'FFC7CE' },
    { label: '黃色填滿與深黃色文字', color: '9C5700', fill: 'FFEB9C' },
    { label: '綠色填滿與深綠色文字', color: '006100', fill: 'C6EFCE' },
    { label: '淺紅色填滿', color: '', fill: 'FFC7CE' },
    { label: '紅色文字', color: 'C00000', fill: '' },
    { label: '粗體', color: '', fill: '' },
  ];
  let preset = $state(0);
  let usesFormat = $derived(!['3_color_scale', '2_color_scale', 'data_bar'].includes(rule.type));

  function describe(r: CFRuleDTO): string {
    const kind = kinds.find((k) => k.value === r.type)?.label ?? r.type;
    const op = [...ops, ...textOps].find((o) => o.value === r.op)?.label ?? '';
    const where = (r.ranges ?? []).map((g) => `${colName(g.c0)}${g.r0 + 1}:${colName(g.c1)}${g.r1 + 1}`).join(' ');
    let what = kind;
    if (r.type === 'cell') what += ` ${op} ${r.value1}${r.value2 ? ` 與 ${r.value2}` : ''}`;
    if (r.type === 'text') what += ` ${op}「${r.value1}」`;
    if (r.type === 'top' || r.type === 'bottom') what = `${kind.replace('N', r.value1 || '10')}${r.percent ? '%' : ''}`;
    if (r.type === 'average') what = r.below ? '低於平均' : '高於平均';
    if (r.type === 'formula') what += `：=${r.value1}`;
    return `${what}（${where}）`;
  }

  async function load(): Promise<void> {
    list = await rules();
  }

  async function add(): Promise<void> {
    const p = presets[preset];
    const r = { ...rule };
    if (usesFormat) {
      r.color = p.color;
      r.fill = p.fill;
      r.bold = p.label === '粗體';
    }
    if (r.type === 'top' || r.type === 'bottom') r.value1 = r.value1 || '10';
    error = await onadd(r);
    if (!error) {
      if (manage) {
        adding = false;
        await load();
      } else onclose();
    }
  }

  async function remove(i: number): Promise<void> {
    error = await ondelete(i);
    await load();
  }

  onMount(() => {
    if (manage) void load();
    else adding = true;
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label="設定格式化的條件" style="width: 500px">
    <h2>{adding ? '新增格式化規則' : '設定格式化的條件規則管理員'}</h2>
    {#if adding}
      <label>
        規則類型
        <select bind:value={rule.type}>
          {#each kinds as k (k.value)}<option value={k.value}>{k.label}</option>{/each}
        </select>
      </label>
      {#if rule.type === 'cell'}
        <label>
          條件
          <select bind:value={rule.op}>
            {#each ops as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
          </select>
        </label>
        <label>值 <input type="text" bind:value={rule.value1} placeholder="數字、文字或 =公式" /></label>
        {#if rule.op === 'between' || rule.op === 'notBetween'}
          <label>到 <input type="text" bind:value={rule.value2} /></label>
        {/if}
      {:else if rule.type === 'text'}
        <label>
          條件
          <select bind:value={rule.op}>
            {#each textOps as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
          </select>
        </label>
        <label>文字 <input type="text" bind:value={rule.value1} /></label>
      {:else if rule.type === 'top' || rule.type === 'bottom'}
        <label>項數 <input type="text" bind:value={rule.value1} placeholder="10" /></label>
        <label class="check"><input type="checkbox" bind:checked={rule.percent} /> 以百分比計算</label>
      {:else if rule.type === 'average'}
        <label>
          條件
          <select bind:value={rule.below}>
            <option value={false}>高於平均</option>
            <option value={true}>低於平均</option>
          </select>
        </label>
      {:else if rule.type === 'formula'}
        <label>公式 <input type="text" bind:value={rule.value1} placeholder="=$B1>100" /></label>
      {/if}
      {#if usesFormat}
        <label>
          格式
          <select bind:value={preset}>
            {#each presets as p, i (i)}<option value={i}>{p.label}</option>{/each}
          </select>
        </label>
      {/if}
      {#if error}<div class="error" role="alert">{error}</div>{/if}
      <div class="dlg-actions">
        <button onclick={() => (manage ? (adding = false) : onclose())}>取消</button>
        <button class="primary" onclick={() => void add()}>確定</button>
      </div>
    {:else}
      <ul class="rules">
        {#each list as r, i (i)}
          <li>
            <span class="swatch" style:background={r.fill ? `#${r.fill}` : 'transparent'} style:color={r.color ? `#${r.color}` : 'inherit'}>AaBb</span>
            <span class="what">{describe(r)}</span>
            <button onclick={() => void remove(i)}>刪除</button>
          </li>
        {:else}
          <li class="empty">這個工作表沒有格式化規則</li>
        {/each}
      </ul>
      {#if error}<div class="error" role="alert">{error}</div>{/if}
      <div class="dlg-actions">
        <button
          onclick={() => {
            rule = fresh();
            adding = true;
          }}>新增規則…</button
        >
        <button class="primary" onclick={onclose}>關閉</button>
      </div>
    {/if}
  </div>
</div>

<style>
  label {
    justify-content: space-between;
  }
  label.check {
    justify-content: flex-start;
  }
  label input[type='text'],
  label select {
    width: 300px;
  }
  .rules {
    max-height: 260px;
    margin: 0;
    padding: 0;
    overflow: auto;
    list-style: none;
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
  }
  .rules li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 8px;
    border-bottom: 1px solid var(--chrome-border);
  }
  .swatch {
    padding: 0 4px;
    border: 1px solid var(--chrome-border);
    font-size: 11px;
  }
  .what {
    flex: 1;
    font-size: 12px;
  }
  .empty {
    color: var(--chrome-muted);
    justify-content: center;
  }
  .error {
    color: var(--error-fg);
  }
</style>
