// The only module that touches generated Wails bindings. Wails3 is still
// in beta, so keep binding/event usage behind these wrappers.
import { Events } from '@wailsio/runtime';
import { BenchService, SheetService } from '../../bindings/ww-excel/internal/service';
import type {
  BenchEvent,
  BlockDTO,
  BlockRef,
  CellDTO,
  MemStats,
  SheetInfo,
} from '../../bindings/ww-excel/internal/service/models';

export type { BenchEvent, BlockDTO, BlockRef, CellDTO, MemStats, SheetInfo };

export const sheetApi = {
  info(sheet: number): Promise<SheetInfo> {
    return SheetService.Info(sheet);
  },

  async getBlocks(sheet: number, refs: BlockRef[]): Promise<BlockDTO[]> {
    return (await SheetService.GetBlocks(sheet, refs)) ?? [];
  },
};

export const benchApi = {
  benchMode: (): Promise<boolean> => BenchService.BenchMode(),
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
