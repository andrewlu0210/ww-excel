<script lang="ts">
  // 資料 › 資料驗證…: settings, input message and error alert.
  import type { ValidationDTO } from '../bridge/sheet';

  interface Props {
    current: ValidationDTO | null;
    onapply: (v: ValidationDTO | null) => Promise<string | null>;
    onclose: () => void;
  }

  let { current, onapply, onclose }: Props = $props();

  const blank: ValidationDTO = {
    type: 'any',
    operator: 'between',
    formula1: '',
    formula2: '',
    allowBlank: true,
    dropdown: true,
    errorStyle: 'stop',
    errorTitle: '',
    error: '',
    promptTitle: '',
    prompt: '',
  };
  let v = $state<ValidationDTO>({ ...blank });
  $effect.pre(() => {
    v = current ? { ...current } : { ...blank };
  });
  let tab = $state<'settings' | 'input' | 'error'>('settings');
  let error = $state<string | null>(null);

  const types = [
    { value: 'any', label: '任何值' },
    { value: 'whole', label: '整數' },
    { value: 'decimal', label: '實數' },
    { value: 'list', label: '清單' },
    { value: 'date', label: '日期' },
    { value: 'time', label: '時間' },
    { value: 'textLength', label: '文字長度' },
    { value: 'custom', label: '自訂' },
  ];
  const operators = [
    { value: 'between', label: '介於' },
    { value: 'notBetween', label: '不介於' },
    { value: 'equal', label: '等於' },
    { value: 'notEqual', label: '不等於' },
    { value: 'greaterThan', label: '大於' },
    { value: 'lessThan', label: '小於' },
    { value: 'greaterThanOrEqual', label: '大於或等於' },
    { value: 'lessThanOrEqual', label: '小於或等於' },
  ];
  let numeric = $derived(['whole', 'decimal', 'date', 'time', 'textLength'].includes(v.type));
  let twoValues = $derived(numeric && (v.operator === 'between' || v.operator === 'notBetween'));

  async function apply(): Promise<void> {
    error = await onapply(v.type === 'any' ? null : { ...v, formula2: twoValues ? v.formula2 : '' });
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg" role="dialog" aria-modal="true" aria-label="資料驗證" style="width: 440px">
    <h2>資料驗證</h2>
    <div class="tabs" role="tablist">
      <button role="tab" aria-selected={tab === 'settings'} class:on={tab === 'settings'} onclick={() => (tab = 'settings')}>設定</button>
      <button role="tab" aria-selected={tab === 'input'} class:on={tab === 'input'} onclick={() => (tab = 'input')}>提示訊息</button>
      <button role="tab" aria-selected={tab === 'error'} class:on={tab === 'error'} onclick={() => (tab = 'error')}>錯誤提醒</button>
    </div>
    {#if tab === 'settings'}
      <label>
        儲存格內允許
        <select bind:value={v.type}>
          {#each types as t (t.value)}<option value={t.value}>{t.label}</option>{/each}
        </select>
      </label>
      {#if numeric}
        <label>
          資料
          <select bind:value={v.operator}>
            {#each operators as o (o.value)}<option value={o.value}>{o.label}</option>{/each}
          </select>
        </label>
        <label>{twoValues ? '最小值' : '值'} <input type="text" bind:value={v.formula1} placeholder={v.type === 'date' ? '2026/1/1 或 =A1' : '數字或 =公式'} /></label>
        {#if twoValues}
          <label>最大值 <input type="text" bind:value={v.formula2} /></label>
        {/if}
      {:else if v.type === 'list'}
        <label>來源 <input type="text" bind:value={v.formula1} placeholder="是,否 或 =$H$1:$H$5" /></label>
        <label class="check"><input type="checkbox" bind:checked={v.dropdown} /> 儲存格內的下拉式清單</label>
      {:else if v.type === 'custom'}
        <label>公式 <input type="text" bind:value={v.formula1} placeholder="=A1>0" /></label>
      {/if}
      {#if v.type !== 'any'}
        <label class="check"><input type="checkbox" bind:checked={v.allowBlank} /> 忽略空白</label>
      {/if}
    {:else if tab === 'input'}
      <label>標題 <input type="text" bind:value={v.promptTitle} /></label>
      <label>提示訊息 <input type="text" bind:value={v.prompt} /></label>
    {:else}
      <label>
        樣式
        <select bind:value={v.errorStyle}>
          <option value="stop">停止（不允許輸入）</option>
          <option value="warning">警告（允許輸入並提醒）</option>
          <option value="information">資訊（允許輸入並提醒）</option>
        </select>
      </label>
      <label>標題 <input type="text" bind:value={v.errorTitle} /></label>
      <label>錯誤訊息 <input type="text" bind:value={v.error} /></label>
    {/if}
    {#if error}<div class="error" role="alert">{error}</div>{/if}
    <div class="dlg-actions">
      <button onclick={() => void onapply(null)}>全部清除</button>
      <span class="spacer"></span>
      <button onclick={onclose}>取消</button>
      <button class="primary" onclick={() => void apply()}>確定</button>
    </div>
  </div>
</div>

<style>
  .tabs {
    display: flex;
    gap: 4px;
  }
  .tabs button.on {
    background: var(--popup-active);
  }
  label {
    justify-content: space-between;
  }
  label.check {
    justify-content: flex-start;
  }
  label input[type='text'],
  label select {
    width: 260px;
  }
  .spacer {
    flex: 1;
  }
  .error {
    color: var(--error-fg);
  }
</style>
