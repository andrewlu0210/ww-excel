// Accept only resolved RGB values; workbook metadata must not become arbitrary CSS.
export function sheetTabStyle(color:string|null|undefined):string {
  if(!color||!/^([0-9a-f]{6})$/i.test(color))return '';
  color=color.toUpperCase();
  const rgb=[0,2,4].map(i=>parseInt(color.slice(i,i+2),16));
  const linear=rgb.map(v=>{const n=v/255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4;});
  const luminance=.2126*linear[0]+.7152*linear[1]+.0722*linear[2];
  const text=luminance>.179?'#000000':'#FFFFFF';
  const active=rgb.map(v=>Math.round(v*.15+255*.85).toString(16).padStart(2,'0')).join('');
  return `--sheet-tab-color:#${color};--sheet-tab-text:${text};--sheet-tab-active-bg:#${active}`;
}
