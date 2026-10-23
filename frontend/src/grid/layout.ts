// Axis layout: maps row/col indices to pixel offsets and back.
// Sizes are stored as deltas from the default size in a Fenwick tree,
// so offset() and setSize() are O(log n) even for 1M rows.

class Fenwick {
  private tree: Float64Array;

  constructor(readonly n: number) {
    this.tree = new Float64Array(n + 1);
  }

  add(i: number, delta: number): void {
    for (i++; i <= this.n; i += i & -i) this.tree[i] += delta;
  }

  // Sum of entries [0, i).
  prefix(i: number): number {
    let s = 0;
    for (; i > 0; i -= i & -i) s += this.tree[i];
    return s;
  }
}

export class AxisLayout {
  private deltas: Fenwick;
  private overrides = new Map<number, number>();

  constructor(
    readonly count: number,
    readonly defaultSize: number,
  ) {
    this.deltas = new Fenwick(count);
  }

  size(i: number): number {
    return this.overrides.get(i) ?? this.defaultSize;
  }

  // Hidden rows/cols are size 0.
  setSize(i: number, px: number): void {
    const next = Math.max(0, px);
    const delta = next - this.size(i);
    if (delta === 0) return;
    this.deltas.add(i, delta);
    if (next === this.defaultSize) this.overrides.delete(i);
    else this.overrides.set(i, next);
  }

  // Pixel offset of the start of index i; offset(count) is the total size.
  offset(i: number): number {
    return i * this.defaultSize + this.deltas.prefix(i);
  }

  get total(): number {
    return this.offset(this.count);
  }

  // Index containing pixel px: the largest i with offset(i) <= px, which
  // skips zero-size (hidden) entries at a boundary.
  indexAt(px: number): number {
    if (px <= 0) return this.firstVisibleFrom(0);
    let lo = 0;
    let hi = this.count - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >>> 1;
      if (this.offset(mid) <= px) lo = mid;
      else hi = mid - 1;
    }
    return lo;
  }

  private firstVisibleFrom(i: number): number {
    while (i < this.count - 1 && this.size(i) === 0) i++;
    return i;
  }
}

// Column index (0-based) to Excel letters: 0 -> A, 25 -> Z, 26 -> AA.
export function colName(index: number): string {
  let s = '';
  for (let n = index + 1; n > 0; n = Math.floor((n - 1) / 26)) {
    s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
  }
  return s;
}

export function cellAddress(row: number, col: number): string {
  return `${colName(col)}${row + 1}`;
}
