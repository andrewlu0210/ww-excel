import type { CellPos } from '../grid/selection';

export type PointerArea = 'cell' | 'col' | 'row' | 'corner';

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
