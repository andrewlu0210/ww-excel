export interface NumberFormatType {
  label: string;
  code: string;
  example?: string;
}
export interface NumberFormatCategory {
  id: string;
  label: string;
  types: NumberFormatType[];
}

// Categories describe the purpose; date/calendar and time variants belong to
// the type list within their category, not to the top-level category list.
export const NUMBER_FORMAT_CATEGORIES: NumberFormatCategory[] = [
  { id: 'general', label: '通用格式', types: [{ label: '通用格式', code: '' }] },
  { id: 'number', label: '數值', types: [{ label: '數值', code: '#,##0.00' }] },
  { id: 'currency', label: '貨幣', types: [{ label: '貨幣', code: '"$"#,##0.00_);[Red]\\("$"#,##0.00\\)' }] },
  { id: 'accounting', label: '會計專用', types: [{ label: '會計專用', code: '_("$"* #,##0.00_);_("$"* \\(#,##0.00\\);_("$"* "-"??_);_(@_)' }] },
  { id: 'date', label: '日期', types: [
    { label: '日期', code: 'yyyy/m/d', example: '2012/3/14' },
    { label: '日期', code: 'yyyy-mm-dd', example: '2012-03-14' },
    { label: '日期（中文）', code: 'yyyy"年"m"月"d"日"', example: '2012年3月14日' },
    { label: '日期（中文）', code: 'm"月"d"日"', example: '3月14日' },
    { label: '日期（中文）', code: '[DBNum1][$-404]yyyy"年"m"月"d"日"', example: '二○一二年三月十四日' },
    { label: '日期（中文）', code: '[DBNum1][$-404]m"月"d"日"', example: '三月十四日' },
    { label: '日期', code: '[$-404]dddd', example: '星期三' },
    { label: '日期', code: '[$-404]ddd', example: '週三' },
    { label: '民國年', code: '[$-404]e/m/d', example: '101/3/14' },
    { label: '民國年（中文）', code: '[$-404]gge"年"m"月"d"日"', example: '民國101年3月14日' },
  ] },
  { id: 'time', label: '時間', types: [
    { label: '時間', code: 'h:mm:ss', example: '13:30:00' },
    { label: '上午／下午', code: '上午/下午 h"時"mm"分"', example: '下午 1時30分' },
  ] },
  { id: 'percent', label: '百分比', types: [{ label: '百分比', code: '0.00%' }] },
  { id: 'fraction', label: '分數', types: [{ label: '分數', code: '# ?/?' }] },
  { id: 'scientific', label: '科學記號', types: [{ label: '科學記號', code: '0.00E+00' }] },
  { id: 'text', label: '文字', types: [{ label: '文字', code: '@' }] },
  { id: 'custom', label: '自訂', types: [] },
];

// Unknown imported codes remain editable as custom formats and are never
// replaced merely because the dialog opens or the category is inspected.
export function numberFormatCategory(code: string): string {
  return NUMBER_FORMAT_CATEGORIES.find(category => category.types.some(type => type.code === code))?.id ?? 'custom';
}
