import type { CellPos } from '../grid/selection';

export type PointerArea = 'cell' | 'col' | 'row' | 'corner';

export interface GridPointer {
  kind: 'down' | 'drag' | 'up' | 'dbl';
  area: PointerArea;
  pos: CellPos;
  shift: boolean;
  meta: boolean;
}
