// The only module that touches generated Wails bindings. Wails3 is still
// in beta, so keep binding/event usage behind these wrappers.
import { Events } from '@wailsio/runtime';
import { BenchService, DocumentService, SheetService } from '../../bindings/ww-excel/internal/service';
import type {
  BenchEvent,
  BlockDTO,
  BlockRef,
  BlocksResult,
  CellDTO,
  CellPos,
  CellsChanged,
  ChangedCell,
  DocState,
  MemStats,
  RangeDTO,
  SheetInfo,
} from '../../bindings/ww-excel/internal/service/models';
import type { MenuAction } from '../input/keymap';

export type {
  BenchEvent,
  BlockDTO,
  BlockRef,
  BlocksResult,
  CellDTO,
  CellPos,
  CellsChanged,
  ChangedCell,
  DocState,
  MemStats,
  RangeDTO,
  SheetInfo,
};

export const sheetApi = {
  info(sheet: number): Promise<SheetInfo> {
    return SheetService.Info(sheet);
  },

  async getBlocks(sheet: number, refs: BlockRef[]): Promise<BlocksResult> {
    const res = await SheetService.GetBlocks(sheet, refs);
    return { version: res.version, blocks: res.blocks ?? [] };
  },

  editText: (sheet: number, row: number, col: number): Promise<string> => SheetService.EditText(sheet, row, col),
  setCell: (sheet: number, row: number, col: number, text: string): Promise<void> =>
    SheetService.SetCell(sheet, row, col, text),
  fill: (sheet: number, ranges: RangeDTO[], text: string): Promise<void> => SheetService.Fill(sheet, ranges, text),
  clear: (sheet: number, ranges: RangeDTO[]): Promise<void> => SheetService.Clear(sheet, ranges),
  undo: (): Promise<boolean> => SheetService.Undo(),
  redo: (): Promise<boolean> => SheetService.Redo(),
  dataEdge: (sheet: number, row: number, col: number, dr: number, dc: number): Promise<CellPos> =>
    SheetService.DataEdge(sheet, row, col, dr, dc),

  onCellsChanged(cb: (e: CellsChanged) => void): () => void {
    return Events.On('cells:changed', (ev) => cb(ev.data));
  },
};

export const documentApi = {
  state: (): Promise<DocState> => DocumentService.State(),
  newDocument: (): Promise<boolean> => DocumentService.New(),
  open: (): Promise<boolean> => DocumentService.Open(),
  save: (): Promise<boolean> => DocumentService.Save(),
  saveAs: (): Promise<boolean> => DocumentService.SaveAs(),
  requestClose: (): Promise<void> => DocumentService.RequestClose(),

  onState(cb: (s: DocState) => void): () => void {
    return Events.On('doc:state', (ev) => cb(ev.data));
  },
  onReset(cb: (s: DocState) => void): () => void {
    return Events.On('doc:reset', (ev) => cb(ev.data));
  },
  onMenu(cb: (action: MenuAction) => void): () => void {
    return Events.On('menu:action', (ev) => cb(ev.data as MenuAction));
  },
};

export const benchApi = {
  benchMode: (): Promise<boolean> => BenchService.BenchMode(),
  selfTestMode: (): Promise<boolean> => BenchService.SelfTestMode(),
  ping: (): Promise<number> => BenchService.Ping(),
  mem: (): Promise<MemStats> => BenchService.Mem(),
  emitBurst: (count: number, blocksPerEvent: number): Promise<void> =>
    BenchService.EmitBurst(count, blocksPerEvent),
  report: (markdown: string): Promise<void> => BenchService.ReportBench(markdown),
  snapshot: (name: string, dataURL: string): Promise<void> => BenchService.SaveSnapshot(name, dataURL),

  onBlockEvent(cb: (e: BenchEvent) => void): () => void {
    return Events.On('bench:block', (ev) => cb(ev.data));
  },
};
