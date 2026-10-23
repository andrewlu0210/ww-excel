import type { BlockDTO, BlockRef } from '../bridge/sheet';

export interface CellView {
  text: string;
  numeric: boolean;
}

export interface BlockRange {
  r0: number; // inclusive block indices
  r1: number;
  c0: number;
  c1: number;
}

interface CachedBlock {
  cells: (CellView | undefined)[];
}

type Fetcher = (refs: BlockRef[]) => Promise<BlockDTO[]>;

export interface FetchStat {
  blocks: number;
  ms: number;
}

// Caches blocks around the viewport. ensure() batches every missing block
// (visible + prefetch ring) into a single backend call.
export class BlockCache {
  private blocks = new Map<number, CachedBlock>();
  private pending = new Set<number>();
  private lastFetches: FetchStat[] = [];

  onUpdate: () => void = () => {};

  constructor(
    private fetcher: Fetcher,
    readonly blockRows: number,
    readonly blockCols: number,
    private maxBlockRow: number,
    private maxBlockCol: number,
    private prefetch = 1,
    private capacity = 1500,
  ) {}

  private key(br: number, bc: number): number {
    return br * (this.maxBlockCol + 1) + bc;
  }

  get size(): number {
    return this.blocks.size;
  }

  get inflight(): number {
    return this.pending.size;
  }

  // Recent fetch timings, newest last.
  get fetches(): readonly FetchStat[] {
    return this.lastFetches;
  }

  cell(row: number, col: number): CellView | undefined {
    const br = Math.floor(row / this.blockRows);
    const bc = Math.floor(col / this.blockCols);
    const block = this.blocks.get(this.key(br, bc));
    if (!block) return undefined;
    return block.cells[(row % this.blockRows) * this.blockCols + (col % this.blockCols)];
  }

  // Ensure blocks covering the visible block range are loaded.
  ensure(visible: BlockRange): void {
    const p = this.prefetch;
    const r0 = Math.max(0, visible.r0 - p);
    const r1 = Math.min(this.maxBlockRow, visible.r1 + p);
    const c0 = Math.max(0, visible.c0 - p);
    const c1 = Math.min(this.maxBlockCol, visible.c1 + p);

    const missing: BlockRef[] = [];
    for (let br = r0; br <= r1; br++) {
      for (let bc = c0; bc <= c1; bc++) {
        const k = this.key(br, bc);
        const hit = this.blocks.get(k);
        if (hit) {
          // Refresh LRU position for visible blocks only.
          if (br >= visible.r0 && br <= visible.r1 && bc >= visible.c0 && bc <= visible.c1) {
            this.blocks.delete(k);
            this.blocks.set(k, hit);
          }
        } else if (!this.pending.has(k)) {
          this.pending.add(k);
          missing.push({ row: br, col: bc });
        }
      }
    }
    if (missing.length > 0) void this.load(missing);
  }

  private async load(refs: BlockRef[]): Promise<void> {
    const t0 = performance.now();
    try {
      const result = await this.fetcher(refs);
      for (const dto of result) this.store(dto);
      this.recordFetch({ blocks: refs.length, ms: performance.now() - t0 });
    } catch (err) {
      console.error('GetBlocks failed', err);
    } finally {
      for (const ref of refs) this.pending.delete(this.key(ref.row, ref.col));
    }
    this.evict();
    this.onUpdate();
  }

  private store(dto: BlockDTO): void {
    const cells: (CellView | undefined)[] = new Array(this.blockRows * this.blockCols);
    for (const c of dto.cells ?? []) {
      cells[c.r * this.blockCols + c.c] = { text: c.v, numeric: c.n === true };
    }
    this.blocks.set(this.key(dto.row, dto.col), { cells });
  }

  private evict(): void {
    const excess = this.blocks.size - this.capacity;
    if (excess <= 0) return;
    // Map iteration order is insertion order, so the first keys are the
    // least recently used.
    let n = 0;
    for (const k of this.blocks.keys()) {
      if (n++ >= excess) break;
      this.blocks.delete(k);
    }
  }

  private recordFetch(stat: FetchStat): void {
    this.lastFetches.push(stat);
    if (this.lastFetches.length > 200) this.lastFetches.shift();
  }

  clear(): void {
    this.blocks.clear();
  }
}
