<script lang="ts">
  // WW Excel › 偏好設定… (⌘,). Like macOS preference windows, every
  // change applies at once; a language change reloads the window.
  import { settingsApi, type Settings, type UISettings } from '../bridge/sheet';
  import { untrack } from 'svelte';
  import { t } from '../i18n';

  interface Props {
    initial: UISettings;
    onclose: () => void;
  }

  let { initial, onclose }: Props = $props();
  // The dialog edits its own copy, starting from the current settings.
  const start = untrack(() => initial);
  let s = $state<Settings>({
    appearance: start.appearance,
    language: start.language,
    autoRecoverMinutes: start.autoRecoverMinutes,
    enterMove: start.enterMove,
    checkUpdates: start.checkUpdates,
  });
  let error = $state<string | null>(null);

  async function save(): Promise<void> {
    try {
      await settingsApi.set($state.snapshot(s));
      error = null;
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    }
  }
</script>

<svelte:window onkeydown={(e) => (e.key === 'Escape' || (e.key === 'Enter' && !(e.target instanceof HTMLSelectElement))) && onclose()} />

<div class="dlg-backdrop" role="presentation" onpointerdown={(e) => e.target === e.currentTarget && onclose()}>
  <div class="dlg prefs" role="dialog" aria-modal="true" aria-label={t('偏好設定')} style="width: 400px">
    <h2>{t('偏好設定')}</h2>
    <div class="grid">
      <label for="pref-appearance">{t('外觀')}</label>
      <select id="pref-appearance" bind:value={s.appearance} onchange={save}>
        <option value="system">{t('跟隨系統')}</option>
        <option value="light">{t('淺色')}</option>
        <option value="dark">{t('深色')}</option>
      </select>

      <label for="pref-language">{t('語言')}</label>
      <select id="pref-language" bind:value={s.language} onchange={save}>
        <option value="system">{t('跟隨系統')}</option>
        <option value="zh-TW">{t('繁體中文')}</option>
        <option value="en">English</option>
      </select>

      <label for="pref-enter">{t('按 Enter 後移動')}</label>
      <select id="pref-enter" bind:value={s.enterMove} onchange={save}>
        <option value="down">{t('向下')}</option>
        <option value="right">{t('向右')}</option>
        <option value="up">{t('向上')}</option>
        <option value="left">{t('向左')}</option>
        <option value="none">{t('不移動')}</option>
      </select>

      <label for="pref-recover">{t('自動儲存復原資訊')}</label>
      <span>
        <select id="pref-recover" bind:value={s.autoRecoverMinutes} onchange={save}>
          <option value={0}>{t('關閉')}</option>
          <option value={1}>{t('每 %d 分鐘', 1)}</option>
          <option value={2}>{t('每 %d 分鐘', 2)}</option>
          <option value={5}>{t('每 %d 分鐘', 5)}</option>
          <option value={10}>{t('每 %d 分鐘', 10)}</option>
        </select>
      </span>

      <span></span>
      <label class="check"><input type="checkbox" bind:checked={s.checkUpdates} onchange={save} /> {t('啟動時檢查更新')}</label>
    </div>
    <p class="hint">{t('程式沒有正常結束時，下次啟動會詢問是否復原尚未儲存的文件。')}</p>
    {#if error}<p class="error">{error}</p>{/if}
    <div class="dlg-actions">
      <button class="primary" onclick={onclose}>{t('完成')}</button>
    </div>
  </div>
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 10px 12px;
    align-items: center;
  }
  .grid label:not(.check) {
    text-align: right;
  }
  .hint {
    color: var(--chrome-muted);
    font-size: 11px;
    margin: 12px 0 0;
  }
  .error {
    color: var(--error-fg);
  }
</style>
