import { describe, expect, it, vi } from 'vitest';
import type { BlockDTO, BlockRef } from '../bridge/sheet';
import { BlockCache } from './viewportCache';

vi.mock('../bridge/sheet', () => ({}));

function fakeFetcher() {
  const calls: BlockRef[][] = [];
  const fetcher = async (refs: BlockRef[]): Promise<BlockDTO[]> => {
    calls.push(refs);
    return refs.map((r) => ({ row: r.row, col: r.col, cells: [{ r: 1, c: 2, v: `${r.row}:${r.col}`, n: true }] }));
  };
  return { calls, fetcher };
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
    expect(cache.cell(5 * 64 + 1, 2)).toEqual({ text: '5:0', numeric: true });
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
});
