<script lang="ts">
  // Floating charts and pictures over the grid. Positions come from the
  // sheet (cell anchor + offset); dragging moves or resizes an object
  // locally and reports the final rectangle when the pointer is released.
  import { t } from '../i18n';
  import type { ChartDataDTO, ObjectDTO } from '../bridge/sheet';
  import { drawShape } from '../shapes/render';
  import { fontEpoch } from '../stores/fontLoader';
  import { drawChart, LIGHT_THEME } from '../charts/render';

  interface Rect {
    x: number;
    y: number;
    w: number;
    h: number;
  }

  interface Props {
    scale?: number;
    objects: ObjectDTO[];
    // Rectangle of an object in host coordinates, recomputed when tick
    // changes (scrolling, layout).
    rectOf: (o: ObjectDTO) => Rect;
    tick: number;
    // Offset of the content area (row header width, column header height).
    left: number;
    top: number;
    selected: number | null;
    chartData: Map<number, ChartDataDTO>;
    images: Map<string, string>;
    onselect: (id: number | null) => void;
    oncommit: (o: ObjectDTO, rect: Rect, label: string) => void;
    onedit: (id: number) => void;
  }

  let { scale = 1, objects, rectOf, tick, left, top, selected, chartData, images, onselect, oncommit, onedit }: Props = $props();

  // An object being dragged: which edges move (move = all).
  let drag = $state<{ id: number; edges: string; x0: number; y0: number; start: Rect; now: Rect } | null>(null);

  function rect(o: ObjectDTO): Rect {
    void tick;
    if (drag && drag.id === o.id) return drag.now;
    return rectOf(o);
  }

  function onDown(e: PointerEvent, o: ObjectDTO, edges: string): void {
    if (e.button !== 0 || o.shape) return;
    e.preventDefault();
    e.stopPropagation();
    onselect(o.id);
    const start = rectOf(o);
    drag = { id: o.id, edges, x0: e.clientX, y0: e.clientY, start, now: start };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }

  function onMove(e: PointerEvent): void {
    if (!drag) return;
    const dx = e.clientX - drag.x0;
    const dy = e.clientY - drag.y0;
    const s = drag.start;
    let { x, y, w, h } = s;
    if (drag.edges === 'move') {
      x += dx;
      y += dy;
    } else {
      if (drag.edges.includes('w')) {
        x = Math.min(s.x + dx, s.x + s.w - 10 * scale);
        w = s.w - (x - s.x);
      }
      if (drag.edges.includes('e')) w = Math.max(10 * scale, s.w + dx);
      if (drag.edges.includes('n')) {
        y = Math.min(s.y + dy, s.y + s.h - 10 * scale);
        h = s.h - (y - s.y);
      }
      if (drag.edges.includes('s')) h = Math.max(10 * scale, s.h + dy);
      // Shift keeps the aspect ratio (always for corner drags of pictures).
      const o = objects.find((x) => x.id === drag!.id);
      if ((e.shiftKey || o?.kind === 'image') && drag.edges.length === 2) {
        const ratio = s.w / s.h;
        if (w / h > ratio) w = h * ratio;
        else h = w / ratio;
        if (drag.edges.includes('w')) x = s.x + s.w - w;
        if (drag.edges.includes('n')) y = s.y + s.h - h;
      }
    }
    // Keep the top-left inside the sheet.
    drag.now = { x: Math.max(left, x), y: Math.max(top, y), w, h };
  }

  function onUp(): void {
    if (!drag) return;
    const d = drag;
    drag = null;
    const o = objects.find((x) => x.id === d.id);
    const moved = d.now.x !== d.start.x || d.now.y !== d.start.y || d.now.w !== d.start.w || d.now.h !== d.start.h;
    if (o && moved) oncommit(o, d.now, d.edges === 'move' ? t('移動物件') : t('調整物件大小'));
  }

  // Draws a chart into its canvas whenever size or data change.
  function chart(canvas: HTMLCanvasElement, args: { o: ObjectDTO; w: number; h: number; data: ChartDataDTO | undefined; scale: number }) {
    const paint = (a: typeof args) => {
      const dpr = window.devicePixelRatio || 1;
      canvas.width = Math.max(1, Math.round(a.w * dpr));
      canvas.height = Math.max(1, Math.round(a.h * dpr));
      const ctx = canvas.getContext('2d');
      if (!ctx || !a.o.chart) return;
      ctx.setTransform(dpr * a.scale, 0, 0, dpr * a.scale, 0, 0);
      drawChart(ctx, a.w / a.scale, a.h / a.scale, a.o.chart, toChartData(a.data), LIGHT_THEME);
    };
    paint(args);
    return { update: paint };
  }

  function shape(canvas: HTMLCanvasElement, args: { o: ObjectDTO; w: number; h: number; scale: number; tick: number }) {
    const paint = (a: typeof args) => {
      const dpr = window.devicePixelRatio || 1;
      canvas.width = Math.max(1, Math.round(a.w * dpr));
      canvas.height = Math.max(1, Math.round(a.h * dpr));
      const ctx = canvas.getContext('2d');
      if (!ctx || !a.o.shape) return;
      ctx.setTransform(dpr * a.scale, 0, 0, dpr * a.scale, 0, 0);
      drawShape(ctx, a.w / a.scale, a.h / a.scale, a.o.shape);
    };
    paint(args);
    return { update: paint };
  }

  const handles = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'];
</script>

<div class="objects" style:left="{left}px" style:top="{top}px">
  {#each objects as o (o.id)}
    {@const r = rect(o)}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="object"
      data-object-id={o.id}
      class:selected={selected === o.id}
      class:readonly={!!o.shape}
      style:left="{r.x - left}px"
      style:top="{r.y - top}px"
      style:width="{r.w}px"
      style:height="{r.h}px"
      onpointerdown={(e) => onDown(e, o, 'move')}
      onpointermove={onMove}
      onpointerup={onUp}
      onpointercancel={onUp}
      ondblclick={() => o.kind === 'chart' && onedit(o.id)}
    >
      {#if o.shape}
        <canvas use:shape={{ o, w: r.w, h: r.h, scale, tick: tick + fontEpoch }} style:width="{r.w}px" style:height="{r.h}px"></canvas>
      {:else if o.kind === 'chart'}
        <canvas use:chart={{ o, w: r.w, h: r.h, data: chartData.get(o.id), scale }} style:width="{r.w}px" style:height="{r.h}px"></canvas>
      {:else if images.get(o.imageKey)}
        <img src={images.get(o.imageKey)} alt="" draggable="false" />
      {/if}
      {#if selected === o.id && !o.shape}
        {#each handles as hd (hd)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="handle {hd}"
            onpointerdown={(e) => onDown(e, o, hd)}
            onpointermove={onMove}
            onpointerup={onUp}
            onpointercancel={onUp}
          ></div>
        {/each}
      {/if}
    </div>
  {/each}
</div>

<script module lang="ts">
  import type { ChartDataDTO as Data } from '../bridge/sheet';
  import type { ChartData as Drawn } from '../charts/render';

  export function toChartData(d: Data | undefined): Drawn {
    if (!d) return { categories: [], series: [] };
    return {
      categories: d.categories ?? [],
      valueScale:d.valueScale,
      series: (d.series ?? []).map((s) => ({ name: s.name, labels:s.labels ?? undefined, values: s.values ?? [], x: s.x ?? [] })),
    };
  }
</script>

<style>
  .object.readonly { pointer-events: none; }
  .objects {
    position: absolute;
    right: 0;
    bottom: 0;
    z-index: 1;
    overflow: hidden;
    pointer-events: none;
  }
  .object {
    position: absolute;
    box-sizing: border-box;
    pointer-events: auto;
    cursor: move;
    border: 1px solid transparent;
    background: transparent;
  }
  .object.selected {
    border-color: var(--accent);
  }
  canvas,
  img {
    display: block;
    width: 100%;
    height: 100%;
    pointer-events: none;
    -webkit-user-drag: none;
  }
  .handle {
    position: absolute;
    width: 8px;
    height: 8px;
    margin: -4px 0 0 -4px;
    background: #fff;
    border: 1px solid var(--accent);
    border-radius: 50%;
  }
  .nw {
    left: 0;
    top: 0;
    cursor: nwse-resize;
  }
  .n {
    left: 50%;
    top: 0;
    cursor: ns-resize;
  }
  .ne {
    left: 100%;
    top: 0;
    cursor: nesw-resize;
  }
  .e {
    left: 100%;
    top: 50%;
    cursor: ew-resize;
  }
  .se {
    left: 100%;
    top: 100%;
    cursor: nwse-resize;
  }
  .s {
    left: 50%;
    top: 100%;
    cursor: ns-resize;
  }
  .sw {
    left: 0;
    top: 100%;
    cursor: nesw-resize;
  }
  .w {
    left: 0;
    top: 50%;
    cursor: ew-resize;
  }
</style>
