import type { CellPos } from '../grid/selection';

// 'fill': the fill handle at the selection's bottom-right corner.
export type PointerArea = 'cell' | 'col' | 'row' | 'corner' | 'fill';

export interface GridPointer {
  kind: 'down' | 'drag' | 'up' | 'dbl';
  area: PointerArea;
  pos: CellPos;
  shift: boolean;
  meta: boolean;
}

// A header border dragged to resize a row or column.
export interface ResizeRequest {
  rows: boolean;
  index: number;
  px: number;
}

// Right-click: what was under the pointer, and where to show the menu.
export interface ContextRequest {
  area: PointerArea;
  pos: CellPos;
  x: number;
  y: number;
}
