// The only module that touches generated Wails bindings. Wails3 is still
// in beta, so keep binding/event usage behind these wrappers.
import { Events } from '@wailsio/runtime';
import { BenchService, DocumentService, SheetService } from '../../bindings/ww-excel/internal/service';
import type { FuncInfo } from '../../bindings/ww-excel/internal/formula/models';
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
  FormatPatch,
  LayoutChanged,
  LayoutDTO,
  MemStats,
  RangeDTO,
  SheetInfo,
  StyleDTO,
  TextSample,
} from '../../bindings/ww-excel/internal/service/models';
import type { MenuAction } from '../input/keymap';

export type {
  FuncInfo,
  FormatPatch,
  LayoutChanged,
  LayoutDTO,
  StyleDTO,
  TextSample,
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

  async getBlocks(sheet: number, refs: BlockRef[], knownStyles: number): Promise<BlocksResult> {
    const res = await SheetService.GetBlocks(sheet, refs, knownStyles);
    return { version: res.version, blocks: res.blocks ?? [], styles: res.styles ?? [] };
  },
  async layout(sheet: number, knownStyles: number): Promise<LayoutDTO> {
    return SheetService.Layout(sheet, knownStyles);
  },

  editText: (sheet: number, row: number, col: number): Promise<string> => SheetService.EditText(sheet, row, col),

  // Edits resolve to null, or to a message for the user when Go rejects
  // the input (e.g. a formula syntax error); nothing changes then.
  setCell: (sheet: number, row: number, col: number, text: string): Promise<string | null> =>
    rejected(SheetService.SetCell(sheet, row, col, text)),
  fill: (sheet: number, ranges: RangeDTO[], text: string, activeRow: number, activeCol: number): Promise<string | null> =>
    rejected(SheetService.Fill(sheet, ranges, text, activeRow, activeCol)),
  async functions(): Promise<FuncInfo[]> {
    return (await SheetService.Functions()) ?? [];
  },
  clear: (sheet: number, ranges: RangeDTO[], mode: 'contents' | 'formats' | 'all' = 'contents'): Promise<void> =>
    SheetService.Clear(sheet, ranges, mode),
  format: (sheet: number, ranges: RangeDTO[], patch: FormatPatch): Promise<string | null> =>
    rejected(SheetService.Format(sheet, ranges, patch)),
  setSizes: (sheet: number, rows: boolean, indices: number[], px: number): Promise<void> =>
    SheetService.SetSizes(sheet, rows, indices, px),
  merge: (sheet: number, rg: RangeDTO, center: boolean): Promise<string | null> =>
    rejected(SheetService.Merge(sheet, rg, center)),
  unmerge: (sheet: number, ranges: RangeDTO[]): Promise<void> => SheetService.Unmerge(sheet, ranges),
  async autoFitColumn(sheet: number, col: number): Promise<TextSample[]> {
    return (await SheetService.AutoFitColumn(sheet, col)) ?? [];
  },
  previewFormat: (sheet: number, row: number, col: number, code: string): Promise<string> =>
    SheetService.PreviewFormat(sheet, row, col, code),
  async rowTexts(sheet: number, row: number): Promise<TextSample[]> {
    return (await SheetService.RowTexts(sheet, row)) ?? [];
  },
  undo: (): Promise<boolean> => SheetService.Undo(),
  redo: (): Promise<boolean> => SheetService.Redo(),
  dataEdge: (sheet: number, row: number, col: number, dr: number, dc: number): Promise<CellPos> =>
    SheetService.DataEdge(sheet, row, col, dr, dc),

  onCellsChanged(cb: (e: CellsChanged) => void): () => void {
    return Events.On('cells:changed', (ev) => cb(ev.data));
  },
  onLayoutChanged(cb: (e: LayoutChanged) => void): () => void {
    return Events.On('layout:changed', (ev) => cb(ev.data));
  },
};

async function rejected(p: Promise<void>): Promise<string | null> {
  try {
    await p;
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : String(err);
  }
}

export const documentApi = {
  state: (): Promise<DocState> => DocumentService.State(),
  newDocument: (): Promise<boolean> => DocumentService.New(),
  open: (): Promise<boolean> => DocumentService.Open(),
  save: (): Promise<boolean> => DocumentService.Save(),
  saveAs: (): Promise<boolean> => DocumentService.SaveAs(),
  requestClose: (): Promise<void> => DocumentService.RequestClose(),
  openPath: (path: string): Promise<void> => DocumentService.OpenPath(path),
  addSheet: (index: number): Promise<number> => DocumentService.AddSheet(index),
  deleteSheet: (index: number): Promise<boolean> => DocumentService.DeleteSheet(index),
  renameSheet: (index: number, name: string): Promise<string | null> =>
    rejected(DocumentService.RenameSheet(index, name)),
  moveSheet: (from: number, to: number): Promise<void> => DocumentService.MoveSheet(from, to),

  onState(cb: (s: DocState) => void): () => void {
    return Events.On('doc:state', (ev) => cb(ev.data));
  },
  onReset(cb: (s: DocState) => void): () => void {
    return Events.On('doc:reset', (ev) => cb(ev.data));
  },
  onSheetsChanged(cb: (s: DocState) => void): () => void {
    return Events.On('sheets:changed', (ev) => cb(ev.data));
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
  recalcChain: (n: number): Promise<number> => BenchService.RecalcChain(n),
  saveCopy: (name: string): Promise<string> => BenchService.SaveCopy(name),

  onBlockEvent(cb: (e: BenchEvent) => void): () => void {
    return Events.On('bench:block', (ev) => cb(ev.data));
  },
};
