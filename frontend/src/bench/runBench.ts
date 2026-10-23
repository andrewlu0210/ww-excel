// v0.1 performance harness: IPC latency, block fetch cost, event push
// throughput, scroll frame rate and Go memory. Produces a markdown report.
import { benchApi, sheetApi, type BlockRef } from '../bridge/sheet';
import { mean, quantile, type ScrollBenchResult } from '../grid/perf';
import type { BlockCache } from '../stores/viewportCache';

export interface ScrollTarget {
  runScrollBench(frames: number, step: (i: number) => { x: number; y: number }): Promise<ScrollBenchResult>;
  scrollToOrigin(): void;
  contentSize(): { width: number; height: number };
}

type Row = Record<string, string | number>;

interface Section {
  title: string;
  rows: Row[];
  note?: string;
}

const fmt = (n: number, digits = 2) => n.toFixed(digits);

function latencyRow(label: string, ms: number[], extra: Row = {}): Row {
  return {
    case: label,
    n: ms.length,
    'mean ms': fmt(mean(ms)),
    'p50 ms': fmt(quantile(ms, 0.5)),
    'p95 ms': fmt(quantile(ms, 0.95)),
    'p99 ms': fmt(quantile(ms, 0.99)),
    ...extra,
  };
}

async function timeIt(fn: () => Promise<unknown>): Promise<number> {
  const t0 = performance.now();
  await fn();
  return performance.now() - t0;
}

async function benchPing(): Promise<Section> {
  for (let i = 0; i < 50; i++) await benchApi.ping();
  const seq: number[] = [];
  for (let i = 0; i < 500; i++) seq.push(await timeIt(() => benchApi.ping()));

  const parallel = 100;
  const total = await timeIt(() => Promise.all(Array.from({ length: parallel }, () => benchApi.ping())));

  return {
    title: 'Binding 呼叫延遲（空呼叫 Ping）',
    rows: [
      latencyRow('sequential', seq),
      { case: `${parallel} parallel`, n: parallel, 'total ms': fmt(total), 'per call ms': fmt(total / parallel) },
    ],
  };
}

async function benchBlocks(): Promise<Section> {
  const rows: Row[] = [];
  for (const count of [1, 9, 25, 64]) {
    // Dense blocks from the top of the sheet (all hold data).
    const refs: BlockRef[] = Array.from({ length: count }, (_, i) => ({ row: i, col: 0 }));
    const sample = (await sheetApi.getBlocks(0, refs)).blocks ?? [];
    const cells = sample.reduce((n, b) => n + (b.cells?.length ?? 0), 0);
    const kb = JSON.stringify(sample).length / 1024;

    const ms: number[] = [];
    for (let i = 0; i < 40; i++) ms.push(await timeIt(() => sheetApi.getBlocks(0, refs)));
    rows.push(latencyRow(`${count} blocks`, ms, { cells, 'payload KB': fmt(kb, 1) }));
  }
  return {
    title: 'GetBlocks 批次讀取',
    rows,
    note: '一個 viewport 約 3×3 blocks（含預取環則約 5×5）。',
  };
}

async function benchEvents(): Promise<Section> {
  const rows: Row[] = [];
  for (const [count, perEvent] of [
    [200, 0],
    [200, 1],
    [50, 9],
  ] as const) {
    const lag: number[] = [];
    let received = 0;
    let done!: () => void;
    const finished = new Promise<void>((r) => (done = r));
    const off = benchApi.onBlockEvent((e) => {
      lag.push(Date.now() - e.sentAt);
      if (++received === count) done();
    });

    const t0 = performance.now();
    await benchApi.emitBurst(count, perEvent);
    await Promise.race([finished, new Promise((r) => setTimeout(r, 10_000))]);
    const total = performance.now() - t0;
    off();

    rows.push({
      case: `${count} events × ${perEvent} blocks`,
      received,
      'total ms': fmt(total),
      'events/s': fmt((received / total) * 1000, 0),
      'lag p50 ms': fmt(quantile(lag, 0.5)),
      'lag p95 ms': fmt(quantile(lag, 0.95)),
    });
  }
  return {
    title: 'Event 推送（Go → 前端）',
    rows,
    note: 'lag 以 Go 端 UnixMilli 與前端 Date.now() 相減，精度為 1ms。',
  };
}

function scrollRow(label: string, r: ScrollBenchResult): Row {
  const dropped = r.frameMs.filter((ms) => ms > 1000 / 60 + 4).length;
  return {
    case: label,
    frames: r.frameMs.length,
    fps: fmt(1000 / mean(r.frameMs), 1),
    'frame p95 ms': fmt(quantile(r.frameMs, 0.95)),
    'frame max ms': fmt(Math.max(...r.frameMs)),
    'dropped (>20.7ms)': dropped,
    'draw p50 ms': fmt(quantile(r.drawMs, 0.5)),
    'draw p95 ms': fmt(quantile(r.drawMs, 0.95)),
  };
}

async function benchScroll(grid: ScrollTarget, cache: BlockCache): Promise<Section[]> {
  const { height } = grid.contentSize();
  const frames = 300;
  const fetchStart = cache.fetches.length;

  // ~10 rows per frame through the dense region: typical trackpad flick.
  const steady = await grid.runScrollBench(frames, (i) => ({ x: 0, y: i * 200 }));
  // Large jumps across the full 1M rows: every frame misses the cache.
  const jumps = await grid.runScrollBench(frames, (i) => ({ x: 0, y: ((i * 7919) % frames) * (height / frames) }));
  // Diagonal scroll, adding columns into the mix.
  const diag = await grid.runScrollBench(frames, (i) => ({ x: i * 40, y: i * 120 }));
  // Wait for in-flight fetches so their timings are recorded.
  await new Promise((r) => setTimeout(r, 300));
  grid.scrollToOrigin();

  // fetches is a bounded ring; fall back to all retained samples.
  const fetched = cache.fetches.length > fetchStart ? cache.fetches.slice(fetchStart) : cache.fetches;
  const ms = fetched.map((f) => f.ms);
  const blocks = fetched.map((f) => f.blocks);

  return [
    {
      title: 'Canvas 捲動',
      rows: [scrollRow('steady 200px/frame', steady), scrollRow('random jumps over 1M rows', jumps), scrollRow('diagonal', diag)],
      note: '每個 frame 以程式設定捲動位置並同步重繪；draw 為 grid layer 單次繪製耗時。',
    },
    {
      title: '捲動期間的 block 載入（資料送達畫面的延遲）',
      rows: [latencyRow('GetBlocks during scroll', ms, { 'blocks/call mean': fmt(mean(blocks), 1), 'blocks/call max': Math.max(0, ...blocks) })],
      note: '從 ensure() 發出請求到資料寫入快取的時間；載入後下一個 frame 即重繪。',
    },
  ];
}

async function benchRecalc(): Promise<Section> {
  const rows: Row[] = [];
  for (const n of [1_000, 10_000, 100_000]) {
    const ms: number[] = [];
    for (let i = 0; i < 3; i++) ms.push(await benchApi.recalcChain(n));
    rows.push({ case: `chain of ${n.toLocaleString()} formulas`, 'best ms': fmt(Math.min(...ms)), 'mean ms': fmt(mean(ms)) });
  }
  return {
    title: '公式重算（Go 端，連鎖 A2=A1+1, A3=A2+1 …）',
    rows,
    note: '每次改 A1 後整串重算一次的時間，由 Go 端計時（微秒精度）。v0.3 驗收：1 萬個公式 < 100ms。',
  };
}

async function benchMemory(cachedBlocks: () => number): Promise<Section> {
  const m = await benchApi.mem();
  return {
    title: '記憶體',
    rows: [
      { metric: 'Go heap alloc', value: `${fmt(m.heapAllocMB, 1)} MB` },
      { metric: 'Go sys', value: `${fmt(m.sysMB, 1)} MB` },
      { metric: 'Frontend cached blocks', value: cachedBlocks() },
    ],
    note: 'WKWebView 不提供 performance.memory，前端記憶體請用「活動監視器」觀察 WebContent 程序。',
  };
}

function table(rows: Row[]): string {
  if (rows.length === 0) return '';
  const cols = Object.keys(Object.assign({}, ...rows));
  const head = `| ${cols.join(' | ')} |\n| ${cols.map(() => '---').join(' | ')} |`;
  const body = rows.map((r) => `| ${cols.map((c) => r[c] ?? '').join(' | ')} |`).join('\n');
  return `${head}\n${body}`;
}

export async function runBench(grid: ScrollTarget, cache: BlockCache, info: string): Promise<string> {
  const sections: Section[] = [];
  sections.push(await benchPing());
  sections.push(await benchBlocks());
  sections.push(await benchEvents());
  sections.push(...(await benchScroll(grid, cache)));
  sections.push(await benchRecalc());
  sections.push(await benchMemory(() => cache.size));

  const header = [
    '# Perf Report',
    '',
    `- 時間：${new Date().toISOString()}`,
    `- 資料：${info}`,
    `- 視窗：${window.innerWidth}×${window.innerHeight} @${window.devicePixelRatio}x`,
    `- UA：${navigator.userAgent}`,
    '- 注意：WKWebView 的 `performance.now()` 精度為 1ms，低於 1ms 的數值會顯示為 0 或 1，請以 mean 判讀。',
  ].join('\n');

  const body = sections
    .map((s) => `## ${s.title}\n\n${table(s.rows)}${s.note ? `\n\n> ${s.note}` : ''}`)
    .join('\n\n');
  return `${header}\n\n${body}\n`;
}
