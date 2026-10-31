// The only module that touches generated Wails bindings. Wails3 is still
// in beta, so keep binding/event usage behind these wrappers.
import { Events, Window } from '@wailsio/runtime';
import { BenchService, DocumentService, SettingsService, SheetService } from '../../bindings/ww-excel/internal/service';
import type { Settings } from '../../bindings/ww-excel/internal/settings/models';
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
  CFRuleDTO,
  ChartDataDTO,
  ChartDTO,
  CopyResult,
  DocState,
  ObjectDTO,
  PageSetupDTO,
  PreviewResult,
  NameDTO,
  NoteDTO,
  TableDTO,
  TargetDTO,
  ValidationDTO,
  FilterValueDTO,
  FindOptionsDTO,
  HitDTO,
  PasteResult,
  SortInfo,
  SortKeyDTO,
  FormatPatch,
  LayoutChanged,
  LayoutDTO,
  MemStats,
  RangeDTO,
  SheetInfo,
  StyleDTO,
  TextSample,
  UISettings,
} from '../../bindings/ww-excel/internal/service/models';
import type { MenuAction } from '../input/keymap';
import { tr } from '../i18n';

export type {
  Settings,
  UISettings,
  ChartDataDTO,
  ChartDTO,
  ObjectDTO,
  PageSetupDTO,
  PreviewResult,
  CFRuleDTO,
  NameDTO,
  NoteDTO,
  TableDTO,
  TargetDTO,
  ValidationDTO,
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
  CopyResult,
  DocState,
  FilterValueDTO,
  FindOptionsDTO,
  HitDTO,
  PasteResult,
  SortInfo,
  SortKeyDTO,
  MemStats,
  RangeDTO,
  SheetInfo,
};

// Every document window runs this frontend. Go sends a window's events
// from that window (its name is the sender); drop the other windows'.
let windowName: string | null = null;
void Window.Name()
  .then((n) => (windowName = n))
  .catch(() => {});

function on<T>(name: string, cb: (data: T) => void): () => void {
  return Events.On(name, (ev) => {
    if (windowName !== null && ev.sender && ev.sender !== windowName) return;
    cb(ev.data as T);
  });
}

export type PasteMode = 'all' | 'values' | 'formats' | 'formulas';

// Result of an edit that may be refused: the message for the user.
async function failed<T>(p: Promise<T>): Promise<{ value: T | null; error: string | null }> {
  try {
    return { value: await p, error: null };
  } catch (err) {
    return { value: null, error: tr(err instanceof Error ? err.message : String(err)) };
  }
}

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
  // error: the input was refused (formula error, data validation);
  // notice: it was entered but the user should see a message.
  async setCell(sheet: number, row: number, col: number, text: string): Promise<{ error: string | null; notice: string | null }> {
    const res = await failed(SheetService.SetCell(sheet, row, col, text));
    return { error: res.error, notice: res.value?.notice ? tr(res.value.notice) : null };
  },
  fill: (sheet: number, ranges: RangeDTO[], text: string, activeRow: number, activeCol: number): Promise<string | null> =>
    rejected(SheetService.Fill(sheet, ranges, text, activeRow, activeCol)),
  async functions(): Promise<FuncInfo[]> {
    const list = (await SheetService.Functions()) ?? [];
    return list.map((f) => ({ ...f, desc: tr(f.desc), category: tr(f.category) }));
  },
  clear: (sheet: number, ranges: RangeDTO[], mode: 'contents' | 'formats' | 'all' = 'contents'): Promise<void> =>
    SheetService.Clear(sheet, ranges, mode),
  captureFormats: (sheet: number, area: RangeDTO): Promise<string | null> => rejected(SheetService.CaptureFormats(sheet, area)),
  paintFormats: (sheet: number, ranges: RangeDTO[]): Promise<string | null> => rejected(SheetService.PaintFormats(sheet, ranges)),
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

  // ---- v0.5 ----
  copy: (sheet: number, rg: RangeDTO, cut: boolean) => failed(SheetService.Copy(sheet, rg, cut)),
  clearClip: (): Promise<void> => SheetService.ClearClip(),
  hasClip: (): Promise<boolean> => SheetService.HasClip(),
  // text: from a paste event, or null to read the system clipboard.
  paste: (sheet: number, sel: RangeDTO, text: string | null) => failed(SheetService.Paste(sheet, sel, text)),
  pasteSpecial: (sheet: number, sel: RangeDTO, mode: PasteMode, transpose: boolean, skipBlanks: boolean) =>
    failed(SheetService.PasteSpecial(sheet, sel, mode, transpose, skipBlanks)),
  shift: (sheet: number, rows: boolean, at: number, count: number): Promise<string | null> =>
    rejected(SheetService.Shift(sheet, rows, at, count)),
  fillSeries: (sheet: number, src: RangeDTO, dst: RangeDTO, copyOnly: boolean): Promise<string | null> =>
    rejected(SheetService.FillSeries(sheet, src, dst, copyOnly)),
  hide: (sheet: number, rows: boolean, lo: number, hi: number, hidden: boolean): Promise<string | null> =>
    rejected(SheetService.Hide(sheet, rows, lo, hi, hidden)),
  freeze: (sheet: number, rows: number, cols: number): Promise<string | null> =>
    rejected(SheetService.Freeze(sheet, rows, cols)),
  currentRegion: (sheet: number, row: number, col: number): Promise<RangeDTO> =>
    SheetService.CurrentRegion(sheet, row, col),
  findNext: (sheet: number, row: number, col: number, opt: FindOptionsDTO, forward: boolean): Promise<HitDTO | null> =>
    SheetService.FindNext(sheet, row, col, opt, forward),
  async findAll(sheet: number, opt: FindOptionsDTO): Promise<HitDTO[]> {
    return (await SheetService.FindAll(sheet, opt)) ?? [];
  },
  replace: (sheet: number, opt: FindOptionsDTO, withText: string, all: boolean, row: number, col: number) =>
    failed(SheetService.Replace(sheet, opt, withText, all, row, col)),
  sortInfo: (sheet: number, sel: RangeDTO): Promise<SortInfo> => SheetService.SortInfo(sheet, sel),
  sort: (sheet: number, rg: RangeDTO, keys: SortKeyDTO[], header: boolean): Promise<string | null> =>
    rejected(SheetService.Sort(sheet, rg, keys, header)),
  toggleFilter: (sheet: number, sel: RangeDTO): Promise<string | null> => rejected(SheetService.ToggleFilter(sheet, sel)),
  async filterValues(sheet: number, col: number, headerRow=-1): Promise<FilterValueDTO[]> {
    return (await SheetService.FilterValues(sheet, col,headerRow)) ?? [];
  },
  setFilter: (sheet: number, col: number, shown: string[], all: boolean,headerRow=-1): Promise<string | null> =>
    rejected(SheetService.SetFilter(sheet, col, shown, all,headerRow)),
  sortFilter: (sheet: number, col: number, desc: boolean,headerRow=-1): Promise<string | null> =>
    rejected(SheetService.SortFilter(sheet, col, desc,headerRow)),

  // ---- v0.6 ----
  setNote: (sheet: number, row: number, col: number, text: string): Promise<string | null> =>
    rejected(SheetService.SetNote(sheet, row, col, text)),
  validation: (sheet: number, row: number, col: number): Promise<ValidationDTO | null> =>
    SheetService.Validation(sheet, row, col),
  async listValues(sheet: number, row: number, col: number): Promise<string[]> {
    return (await SheetService.ListValues(sheet, row, col)) ?? [];
  },
  setValidation: (sheet: number, ranges: RangeDTO[], v: ValidationDTO | null): Promise<string | null> =>
    rejected(SheetService.SetValidation(sheet, ranges, v)),
  async condFormats(sheet: number): Promise<CFRuleDTO[]> {
    return (await SheetService.CondFormats(sheet)) ?? [];
  },
  addCondFormat: (sheet: number, ranges: RangeDTO[], rule: CFRuleDTO): Promise<string | null> =>
    rejected(SheetService.AddCondFormat(sheet, ranges, rule)),
  deleteCondFormat: (sheet: number, index: number): Promise<string | null> =>
    rejected(SheetService.DeleteCondFormat(sheet, index)),
  clearCondFormats: (sheet: number, ranges: RangeDTO[]): Promise<string | null> =>
    rejected(SheetService.ClearCondFormats(sheet, ranges)),
  async tableStyles(): Promise<string[]> {
    return (await SheetService.TableStyleList()) ?? [];
  },
  tableRange: (sheet: number, sel: RangeDTO): Promise<SortInfo> => SheetService.TableRange(sheet, sel),
  createTable: (sheet: number, rg: RangeDTO, hasHeaders: boolean, style: string) =>
    failed(SheetService.CreateTable(sheet, rg, hasHeaders, style)),
  tableAt: (sheet: number, row: number, col: number): Promise<TableDTO | null> => SheetService.TableAt(sheet, row, col),
  setTableStyle: (sheet: number, name: string, style: string, bands: boolean): Promise<string | null> =>
    rejected(SheetService.SetTableStyle(sheet, name, style, bands)),
  renameTable: (sheet: number, old: string, name: string): Promise<string | null> =>
    rejected(SheetService.RenameTable(sheet, old, name)),
  toggleTableTotals: (sheet: number, name: string): Promise<string | null> =>
    rejected(SheetService.ToggleTableTotals(sheet, name)),
  convertTable: (sheet: number, name: string): Promise<string | null> =>
    rejected(SheetService.ConvertTable(sheet, name)),

  // ---- v0.7 ----
  chartFromSelection: (sheet: number, sel: RangeDTO, type: string) => failed(SheetService.ChartFromSelection(sheet, sel, type)),
  addChart: (sheet: number, row: number, col: number, w: number, h: number, chart: ChartDTO) =>
    failed(SheetService.AddChart(sheet, row, col, w, h, chart)),
  updateObject: (sheet: number, o: ObjectDTO, label: string): Promise<string | null> =>
    rejected(SheetService.UpdateObject(sheet, o, label)),
  deleteObjects: (sheet: number, ids: number[]): Promise<string | null> => rejected(SheetService.DeleteObjects(sheet, ids)),
  objectImage: (sheet: number, id: number): Promise<string> => SheetService.ObjectImage(sheet, id),
  chartData: (sheet: number, id: number): Promise<ChartDataDTO> => SheetService.ChartData(sheet, id),
  chartPreview: (sheet: number, chart: ChartDTO): Promise<ChartDataDTO> => SheetService.ChartPreview(sheet, chart),
  pageSetup: (sheet: number): Promise<PageSetupDTO> => SheetService.PageSetup(sheet),
  setPageSetup: (sheet: number, p: PageSetupDTO): Promise<string | null> => rejected(SheetService.SetPageSetup(sheet, p)),
  // charts: chart object ID -> PNG data URL, drawn by the frontend.
  printPreview: (sheet: number, charts: Record<number, string>) => failed(SheetService.PrintPreview(sheet, charts)),

  onCellsChanged(cb: (e: CellsChanged) => void): () => void {
    return on('cells:changed', cb);
  },
  onLayoutChanged(cb: (e: LayoutChanged) => void): () => void {
    return on('layout:changed', cb);
  },
};

async function rejected(p: Promise<void>): Promise<string | null> {
  try {
    await p;
    return null;
  } catch (err) {
    return tr(err instanceof Error ? err.message : String(err));
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
  async names(): Promise<NameDTO[]> {
    return (await DocumentService.Names()) ?? [];
  },
  // oldName "" adds a name.
  setName: (oldName: string, oldScope: number, n: NameDTO): Promise<string | null> =>
    rejected(DocumentService.SetName(oldName, oldScope, n)),
  deleteName: (name: string, scope: number): Promise<string | null> => rejected(DocumentService.DeleteName(name, scope)),
  resolve: (sheet: number, text: string): Promise<TargetDTO> => DocumentService.Resolve(sheet, text),
  insertPicture: (sheet: number, row: number, col: number): Promise<string | null> =>
    rejected(DocumentService.InsertPicture(sheet, row, col)),
  exportPDF: (sheet: number, charts: Record<number, string>) => failed(DocumentService.ExportPDF(sheet, charts)),
  printSheet: (sheet: number, charts: Record<number, string>): Promise<string | null> =>
    rejected(DocumentService.PrintSheet(sheet, charts)),

  onState(cb: (s: DocState) => void): () => void {
    return on('doc:state', cb);
  },
  onReset(cb: (s: DocState) => void): () => void {
    return on('doc:reset', cb);
  },
  onSheetsChanged(cb: (s: DocState) => void): () => void {
    return on('sheets:changed', cb);
  },
  onMenu(cb: (action: MenuAction) => void): () => void {
    return on('menu:action', cb);
  },
};

export const settingsApi = {
  get: (): Promise<UISettings> => SettingsService.Get(),
  set: (s: Settings): Promise<UISettings> => SettingsService.Set(s),
  dictionary: async (): Promise<Record<string, string>> => ((await SettingsService.Dictionary()) ?? {}) as Record<string, string>,
  onChanged: (cb: (s: UISettings) => void) => on<UISettings>('settings:changed', cb),
};

export const benchApi = {
  benchMode: (): Promise<boolean> => BenchService.BenchMode(),
  selfTestMode: (): Promise<boolean> => BenchService.SelfTestMode(),
  fixturePath: (): Promise<string> => BenchService.FixturePath(),
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
