// Rolling performance samples for the HUD and the bench harness.

export class Samples {
  private values: number[] = [];

  constructor(private capacity = 240) {}

  push(v: number): void {
    this.values.push(v);
    if (this.values.length > this.capacity) this.values.shift();
  }

  get count(): number {
    return this.values.length;
  }

  mean(): number {
    return this.values.length === 0 ? 0 : this.values.reduce((a, b) => a + b, 0) / this.values.length;
  }

  quantile(q: number): number {
    return quantile(this.values, q);
  }

  clear(): void {
    this.values = [];
  }
}

export function quantile(values: readonly number[], q: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil(q * sorted.length) - 1));
  return sorted[idx];
}

export function mean(values: readonly number[]): number {
  return values.length === 0 ? 0 : values.reduce((a, b) => a + b, 0) / values.length;
}

// Tracks real frame cadence via requestAnimationFrame timestamps.
export class FpsMeter {
  readonly frames = new Samples(120);
  private last = 0;
  private raf = 0;

  start(): void {
    const tick = (t: number) => {
      if (this.last > 0) this.frames.push(t - this.last);
      this.last = t;
      this.raf = requestAnimationFrame(tick);
    };
    this.raf = requestAnimationFrame(tick);
  }

  stop(): void {
    cancelAnimationFrame(this.raf);
    this.last = 0;
  }

  get fps(): number {
    const m = this.frames.mean();
    return m > 0 ? 1000 / m : 0;
  }
}

export interface ScrollBenchResult {
  frameMs: number[];
  drawMs: number[];
}
