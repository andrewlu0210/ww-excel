// The user's preferences (偏好設定), applied to this window.
import { settingsApi, type UISettings } from '../bridge/sheet';
import { setLanguage } from '../i18n';
import { setEnterMove, type EnterMove } from '../input/keymap';

let current: UISettings | null = null;

export function settings(): UISettings | null {
  return current;
}

function applyTheme(appearance: string): void {
  const root = document.documentElement;
  if (appearance === 'light' || appearance === 'dark') root.dataset.theme = appearance;
  else delete root.dataset.theme;
  // The canvases read colors from CSS variables; tell them to refresh.
  window.dispatchEvent(new Event('ww-theme'));
}

function apply(s: UISettings): void {
  current = s;
  applyTheme(s.appearance);
  setEnterMove(s.enterMove as EnterMove);
}

// initSettings loads the preferences and the UI language before the app
// mounts. A language change later reloads the window: strings are
// translated once, when components are created.
export async function initSettings(): Promise<void> {
  try {
    const s = await settingsApi.get();
    setLanguage(s.lang, s.lang === 'en' ? await settingsApi.dictionary() : null);
    apply(s);
  } catch {
    // Outside Wails (unit tests, plain browser): defaults.
  }
  settingsApi.onChanged((s) => {
    const reload = current !== null && current.lang !== s.lang;
    apply(s);
    if (reload) location.reload();
  });
}
