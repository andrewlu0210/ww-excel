import type { BlockDTO, BlockRef, BlocksResult, CellsChanged } from '../bridge/sheet';

export interface CellView {
  text: string;
  numeric: boolean; // right-aligned by default
  center: boolean; // logicals and errors are centered by default
  style: number; // style ID (0 = default)
  color: string | null; // number-format color such as [Red], "#RRGGBB"
  // Conditional-format data bar: length 0..1 and "#RRGGBB", or null.
  bar: { len: number; color: string } | null;
}

interface CellFields {
  v: string;
  n?: boolean;
  x?: boolean;
  st?: number;
  fc?: string;
  bar?: number;
  bc?: string;
}

function view(c: CellFields): CellView {
  return {
    text: c.v,
    numeric: c.n === true,
    center: c.x === true,
    style: c.st ?? 0,
    color: c.fc ? `#${c.fc}` : null,
    bar: c.bc ? { len: c.bar ?? 0, color: `#${c.bc}` } : null,
  };
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

type Fetcher = (refs: BlockRef[]) => Promise<BlocksResult>;

export interface FetchStat {
  blocks: number;
  ms: number;
}

// Caches blocks around the viewport. ensure() batches every missing block
// (visible + prefetch ring) into a single backend call. Edits arrive as
// diffs; a fetch answered at an older document version than the latest
// diff is discarded and refetched on the next draw.
export class BlockCache {
  private blocks = new Map<number, CachedBlock>();
  private pending = new Set<number>();
  private lastFetches: FetchStat[] = [];
  private version = 0;
  // Bumped by reset() so fetches from a previous document are ignored.
  private generation = 0;

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
    const gen = this.generation;
    try {
      const result = await this.fetcher(refs);
      if (gen !== this.generation) return;
      if (result.version >= this.version) {
        for (const dto of result.blocks ?? []) this.store(dto);
      }
      this.recordFetch({ blocks: refs.length, ms: performance.now() - t0 });
    } catch (err) {
      console.error('GetBlocks failed', err);
    } finally {
      if (gen === this.generation) {
        for (const ref of refs) this.pending.delete(this.key(ref.row, ref.col));
      }
    }
    this.evict();
    this.onUpdate();
  }

  // Applies an edit diff. Cells in blocks that are not cached are
  // skipped; they will be fetched fresh when needed.
  applyDiff(diff: CellsChanged): void {
    this.version = Math.max(this.version, diff.version);
    if (diff.invalidate) {
      this.blocks.clear();
    } else {
      for (const c of diff.cells ?? []) {
        const block = this.blocks.get(this.key(Math.floor(c.row / this.blockRows), Math.floor(c.col / this.blockCols)));
        if (!block) continue;
        const i = (c.row % this.blockRows) * this.blockCols + (c.col % this.blockCols);
        // Keep empty-but-styled cells: their style still renders.
        block.cells[i] = c.v === '' && !c.st ? undefined : view(c);
      }
    }
    this.onUpdate();
  }

  private store(dto: BlockDTO): void {
    const cells: (CellView | undefined)[] = new Array(this.blockRows * this.blockCols);
    for (const c of dto.cells ?? []) {
      cells[c.r * this.blockCols + c.c] = view(c);
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

  // Drops everything, e.g. when another document is opened.
  reset(): void {
    this.generation++;
    this.blocks.clear();
    this.pending.clear();
    this.version = 0;
    this.onUpdate();
  }
}
