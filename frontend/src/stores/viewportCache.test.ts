import { describe, expect, it, vi } from 'vitest';
import type { BlockDTO, BlockRef, BlocksResult } from '../bridge/sheet';
import { BlockCache } from './viewportCache';

vi.mock('../bridge/sheet', () => ({}));

function fakeFetcher() {
  const calls: BlockRef[][] = [];
  const state = { version: 0 };
  const fetcher = async (refs: BlockRef[]): Promise<BlocksResult> => {
    calls.push(refs);
    const blocks: BlockDTO[] = refs.map((r) => ({
      row: r.row,
      col: r.col,
      cells: [{ r: 1, c: 2, v: `${r.row}:${r.col}`, n: true }],
    }));
    return { version: state.version, blocks };
  };
  return { calls, fetcher, state };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('BlockCache', () => {
  it('batches visible blocks and the prefetch ring into one call', async () => {
    const { calls, fetcher } = fakeFetcher();
    const cache = new BlockCache(fetcher, 64, 32, 100, 100);
    cache.ensure({ r0: 5, r1: 6, c0: 0, c1: 0 });
    // rows 4..7 x cols 0..1 (col -1 clamped)
    expect(calls).toHaveLength(1);
    expect(calls[0]).toHaveLength(8);

    // Pending blocks are not requested twice.
    cache.ensure({ r0: 5, r1: 6, c0: 0, c1: 0 });
    expect(calls).toHaveLength(1);

    await flush();
    expect(cache.cell(5 * 64 + 1, 2)).toEqual({ text: '5:0', numeric: true, center: false });
    expect(cache.cell(5 * 64, 0)).toBeUndefined();
  });

  it('notifies and evicts least recently used blocks', async () => {
    const { fetcher } = fakeFetcher();
    const cache = new BlockCache(fetcher, 64, 32, 1000, 0, 0, 3);
    const onUpdate = vi.fn();
    cache.onUpdate = onUpdate;

    for (const r of [0, 1, 2]) cache.ensure({ r0: r, r1: r, c0: 0, c1: 0 });
    await flush();
    cache.ensure({ r0: 0, r1: 0, c0: 0, c1: 0 }); // touch block 0
    cache.ensure({ r0: 3, r1: 3, c0: 0, c1: 0 });
    await flush();

    expect(onUpdate).toHaveBeenCalled();
    expect(cache.size).toBe(3);
    expect(cache.cell(1, 2)).toBeDefined(); // block 0 kept
    expect(cache.cell(64 + 1, 2)).toBeUndefined(); // block 1 evicted
  });

  it('applies diffs to cached blocks', async () => {
    const { fetcher } = fakeFetcher();
    const cache = new BlockCache(fetcher, 64, 32, 100, 100, 0);
    cache.ensure({ r0: 0, r1: 0, c0: 0, c1: 0 });
    await flush();
    cache.applyDiff({
      sheet: 0,
      version: 1,
      usedRows: 0,
      usedCols: 0,
      cells: [
        { row: 0, col: 0, v: '台北' },
        { row: 1, col: 2, v: '' },
        { row: 5000, col: 0, v: 'not cached' },
      ],
    });
    expect(cache.cell(0, 0)).toEqual({ text: '台北', numeric: false, center: false });
    expect(cache.cell(1, 2)).toBeUndefined();
  });

  it('drops fetches answered before the latest edit', async () => {
    const { calls, fetcher, state } = fakeFetcher();
    const cache = new BlockCache(fetcher, 64, 32, 100, 100, 0);
    cache.applyDiff({ sheet: 0, version: 5, usedRows: 0, usedCols: 0, cells: [] });

    state.version = 4; // stale answer
    cache.ensure({ r0: 0, r1: 0, c0: 0, c1: 0 });
    await flush();
    expect(cache.size).toBe(0);

    state.version = 5;
    cache.ensure({ r0: 0, r1: 0, c0: 0, c1: 0 });
    await flush();
    expect(calls).toHaveLength(2);
    expect(cache.size).toBe(1);
  });

  it('invalidates everything for large diffs', async () => {
    const { fetcher } = fakeFetcher();
    const cache = new BlockCache(fetcher, 64, 32, 100, 100, 0);
    cache.ensure({ r0: 0, r1: 0, c0: 0, c1: 0 });
    await flush();
    cache.applyDiff({ sheet: 0, version: 1, usedRows: 0, usedCols: 0, cells: [], invalidate: true });
    expect(cache.size).toBe(0);
  });

  it('ignores fetches from before a reset', async () => {
    const { fetcher } = fakeFetcher();
    const cache = new BlockCache(fetcher, 64, 32, 100, 100, 0);
    cache.ensure({ r0: 0, r1: 0, c0: 0, c1: 0 });
    cache.reset(); // fetch still in flight
    await flush();
    expect(cache.size).toBe(0);
  });
});
