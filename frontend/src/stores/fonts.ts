// Font names: the choices in the toolbar, and macOS stand-ins for the
// Windows fonts Excel files use (新細明體, 微軟正黑體, 標楷體...), which
// are not installed on a Mac. The cell keeps the original name, so the
// xlsx file still says 新細明體; only drawing uses the stand-in.

// Windows (and older Mac) family names -> installed macOS families, in
// order of preference. Keys are lower case.
const ALIASES: Record<string, string[]> = {
  新細明體: ['LiSong Pro', 'Apple LiSung', 'Songti TC'],
  pmingliu: ['LiSong Pro', 'Apple LiSung', 'Songti TC'],
  細明體: ['LiSong Pro', 'Apple LiSung', 'Songti TC'],
  mingliu: ['LiSong Pro', 'Apple LiSung', 'Songti TC'],
  微軟正黑體: ['PingFang TC', 'Heiti TC'],
  'microsoft jhenghei': ['PingFang TC', 'Heiti TC'],
  'microsoft jhenghei ui': ['PingFang TC', 'Heiti TC'],
  標楷體: ['BiauKaiTC', 'Kaiti TC'],
  'dfkai-sb': ['BiauKaiTC', 'Kaiti TC'],
  微软雅黑: ['PingFang SC', 'Heiti SC'],
  'microsoft yahei': ['PingFang SC', 'Heiti SC'],
  宋体: ['Songti SC'],
  simsun: ['Songti SC'],
  新宋体: ['Songti SC'],
  nsimsun: ['Songti SC'],
  黑体: ['Heiti SC'],
  simhei: ['Heiti SC'],
  楷体: ['Kaiti SC'],
  kaiti: ['Kaiti SC'],
  仿宋: ['STFangsong'],
  fangsong: ['STFangsong'],
  'ms gothic': ['Hiragino Sans'],
  'ms mincho': ['Hiragino Mincho ProN'],
  'yu gothic': ['YuGothic'],
  calibri: ['Calibri', 'Helvetica Neue'],
  cambria: ['Cambria', 'Georgia'],
};

// The families to try for a font name, the name itself first.
export function fontFamilies(name: string): string[] {
  const alias = ALIASES[name.trim().toLowerCase()] ?? [];
  return [name, ...alias.filter((a) => a !== name)];
}

// Fonts offered in the toolbar: value is the family stored in the cell,
// label what the menu shows.
export const FONT_CHOICES: { value: string; label: string }[] = [
  { value: 'Calibri', label: 'Calibri' },
  { value: 'Aptos', label: 'Aptos' },
  { value: 'Arial', label: 'Arial' },
  { value: 'Helvetica', label: 'Helvetica' },
  { value: 'Times New Roman', label: 'Times New Roman' },
  { value: 'Georgia', label: 'Georgia' },
  { value: 'Courier New', label: 'Courier New' },
  { value: 'Menlo', label: 'Menlo' },
  { value: 'PingFang TC', label: '蘋方-繁' },
  { value: 'Heiti TC', label: '黑體-繁' },
  { value: 'Songti TC', label: '宋體-繁' },
  { value: 'Kaiti TC', label: '楷體-繁' },
  { value: 'BiauKaiTC', label: '標楷體-繁' },
  { value: 'LiSong Pro', label: '儷宋 Pro' },
  { value: 'LiHei Pro', label: '儷黑 Pro' },
  { value: 'Yuanti TC', label: '圓體-繁' },
  { value: 'Lantinghei TC', label: '蘭亭黑-繁' },
  { value: 'Xingkai TC', label: '行楷-繁' },
  { value: 'Libian TC', label: '隸變-繁' },
  { value: 'Hannotate TC', label: '手札體-繁' },
  { value: 'HanziPen TC', label: '翩翩體-繁' },
  { value: '新細明體', label: '新細明體（Windows）' },
  { value: '微軟正黑體', label: '微軟正黑體（Windows）' },
  { value: '標楷體', label: '標楷體（Windows）' },
];

// Label for a font name (the name itself when it is not in the list).
export function fontLabel(name: string): string {
  return FONT_CHOICES.find((f) => f.value === name)?.label ?? name;
}
