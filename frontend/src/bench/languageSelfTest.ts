import { documentApi, settingsApi } from '../bridge/sheet';
import { language, t, tr } from '../i18n';

export async function languageSelfTest(openPreferences: () => void): Promise<{markdown:string;failed:number}> {
  const checks: {name:string;ok:boolean}[] = [];
  const check = (name:string,ok:boolean) => checks.push({name,ok});
  const lang = language();
  const s = await settingsApi.get();
  check('Settings resolve the requested locale', s.lang === lang);
  check('HTML declares the requested locale', document.documentElement.lang === lang);
  const dictionary = await settingsApi.dictionary();
  check('Backend returns the selected catalog', dictionary['儲存'] === (lang === 'zh-CN' ? '保存' : 'Save'));
  check('Ribbon renders translated text', document.querySelector('.ribbon')?.textContent?.includes(t('頁面配置')) === true);
  check('Clipboard renders translated text', document.querySelector('.toolbar')?.textContent?.includes(lang === 'zh-CN' ? '粘贴' : 'Paste') === true);
  check('Formatted messages retain document names', tr('無法開啟「客戶資料.xlsx」').includes('客戶資料.xlsx'));
  openPreferences();
  await new Promise(r=>setTimeout(r,200));
  const select = document.querySelector<HTMLSelectElement>('#pref-language');
  check('Preferences offer all three languages', ['zh-TW','zh-CN','en'].every(value=>[...(select?.options ?? [])].some(o=>o.value===value)));
  check('Preferences display Simplified Chinese by its native name', [...(select?.options ?? [])].some(o=>o.value==='zh-CN' && o.text==='简体中文'));
  check('Preferences title is translated', document.querySelector('.prefs h2')?.textContent === t('偏好設定'));
  const state = await documentApi.state();
  check('Opening preferences does not edit the document', !state.dirty);
  window.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape'}));
  const failed = checks.filter(c=>!c.ok).length;
  return {failed,markdown:`# Language self-test (${lang})\n\n${checks.length-failed} / ${checks.length} passed\n\n`+checks.map(c=>`${c.ok?'✅':'❌'} ${c.name}`).join('\n')+'\n'};
}
