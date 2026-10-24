<script lang="ts">
  // 列印預覽: the PDF that will be printed, shown by WebKit's PDF viewer.
  import { t } from '../i18n';
  interface Props {
    url: string;
    pages: number;
    onprint: () => void;
    onexport: () => void;
    onsetup: () => void;
    onclose: () => void;
  }

  let { url, pages, onprint, onexport, onsetup, onclose }: Props = $props();
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="dlg-backdrop" role="presentation">
  <div class="dlg preview" role="dialog" aria-modal="true" aria-label={t('列印預覽')}>
    <div class="bar">
      <h2>{t('列印預覽')}</h2>
      <span class="pages">{t('共 %d 頁', pages)}</span>
      <span class="spacer"></span>
      <button onclick={onsetup}>{t('版面設定…')}</button>
      <button onclick={onexport}>{t('匯出 PDF…')}</button>
      <button class="primary" onclick={onprint}>{t('列印…')}</button>
      <button onclick={onclose}>{t('關閉')}</button>
    </div>
    <iframe title={t('列印預覽')} src={url}></iframe>
  </div>
</div>

<style>
  .preview {
    width: min(960px, calc(100vw - 48px));
    height: calc(100vh - 64px);
    padding: 10px;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .pages {
    color: var(--chrome-muted);
  }
  .spacer {
    flex: 1;
  }
  iframe {
    flex: 1;
    width: 100%;
    border: 1px solid var(--chrome-border);
    border-radius: 6px;
    background: #808080;
  }
</style>
