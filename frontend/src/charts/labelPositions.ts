// Excel imposes chart-specific restrictions beyond the Open XML enumeration.
export function chartLabelPositions(kind:string,stacked=false):string[] {
 switch(kind){
  case 'column':case 'bar':return ['', 'ctr','inEnd','inBase',...(stacked?[]:['outEnd'])];
  case 'line':case 'scatter':return ['', 'ctr','t','b','l','r'];
  case 'pie':return ['', 'ctr','inEnd','outEnd','bestFit'];
  case 'area':case 'doughnut':return [''];
  default:return [''];
 }
}
