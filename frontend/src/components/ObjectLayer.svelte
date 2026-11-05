<script lang="ts">
  // Floating shapes, charts and pictures over the grid. Positions come from the
  // sheet (cell anchor + offset); dragging moves or resizes an object
  // locally and reports the final rectangle when the pointer is released.
  import { onMount } from 'svelte';
  import { draggedLabelLayout, chartWithLabelLayout } from '../charts/labelDrag';
  import type { ChartLabelHit, LabelLayout } from '../charts/labels';
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
    editing?: number | null;
    chartData: Map<number, ChartDataDTO>;
    images: Map<string, string>;
    onselect: (id: number | null) => void;
    oncommit: (o: ObjectDTO, rect: Rect, label: string) => void;
    onedit: (id: number) => void;
    onlabelcommit:(id:number,series:number,point:number,x:number,y:number)=>Promise<void>;
  }

  let { scale = 1, objects, rectOf, tick, left, top, selected, editing = null, chartData, images, onselect, oncommit, onedit, onlabelcommit }: Props = $props();


  let labelHits=$state.raw(new Map<number,ChartLabelHit[]>());
  let selectedLabel=$state<{id:number;series:number;point:number}|null>(null);
  let labelDrag=$state<{
    id:number;hit:ChartLabelHit;canvas:HTMLCanvasElement;target:HTMLElement;pointerId:number;
    x0:number;y0:number;clientX0:number;clientY0:number;width:number;height:number;
    previous:LabelLayout|null|undefined;layout:LabelLayout|null;moved:boolean;pending:boolean;
  }|null>(null);

  function cancelLabelDrag(event?:Event):void {
    if(!labelDrag||labelDrag.pending)return;
    if(event instanceof PointerEvent&&event.pointerId!==labelDrag.pointerId)return;
    const {target,pointerId}=labelDrag;
    labelDrag=null;
    if(target.hasPointerCapture(pointerId))target.releasePointerCapture(pointerId);
  }
  onMount(()=>{
    const escape=(e:KeyboardEvent)=>{
      if(e.key==='Escape'&&labelDrag&&!labelDrag.pending){e.preventDefault();e.stopPropagation();cancelLabelDrag();}
    };
    window.addEventListener('keydown',escape,true);
    window.addEventListener('blur',cancelLabelDrag);
    return ()=>{window.removeEventListener('keydown',escape,true);window.removeEventListener('blur',cancelLabelDrag);};
  });

  function onLabelDown(e:PointerEvent,o:ObjectDTO,hit:ChartLabelHit):void {
    if(e.button!==0||o.readonly||!o.chart)return;
    e.preventDefault();e.stopPropagation();
    if(labelDrag)return;
    const target=e.currentTarget as HTMLElement;
    const canvas=target.closest('.object')!.querySelector('canvas')!;
    const bounds=canvas.getBoundingClientRect();
    if(bounds.width<=0||bounds.height<=0)return;
    onselect(o.id);
    selectedLabel={id:o.id,series:hit.series,point:hit.point};
    labelDrag={id:o.id,hit:{...hit},canvas,target,pointerId:e.pointerId,
      x0:(e.clientX-bounds.left)*o.w/bounds.width,y0:(e.clientY-bounds.top)*o.h/bounds.height,
      clientX0:e.clientX,clientY0:e.clientY,width:o.w,height:o.h,
      previous:o.chart.series?.[hit.series]?.labels?.points?.[`${hit.point}`]?.layout,
      layout:null,moved:false,pending:false};
    if(e.isTrusted)target.setPointerCapture(e.pointerId);
  }
  function onLabelMove(e:PointerEvent):void {
    const d=labelDrag;
    if(!d||d.pending||e.pointerId!==d.pointerId)return;
    if(!d.moved&&Math.hypot(e.clientX-d.clientX0,e.clientY-d.clientY0)<3)return;
    e.stopPropagation();
    const bounds=d.canvas.getBoundingClientRect();
    if(bounds.width<=0||bounds.height<=0){cancelLabelDrag();return;}
    d.moved=true;
    d.layout=draggedLabelLayout(d.hit,
      (e.clientX-bounds.left)*d.width/bounds.width-d.x0,
      (e.clientY-bounds.top)*d.height/bounds.height-d.y0,d.width,d.height,d.previous);
  }
  async function onLabelUp(e:PointerEvent):Promise<void> {
    const d=labelDrag;
    if(!d||d.pending||e.pointerId!==d.pointerId)return;
    e.stopPropagation();
    onLabelMove(e);
    if(!d.moved||!d.layout||Math.abs(d.layout.x*d.width-d.hit.x)<1e-6&&Math.abs(d.layout.y*d.height-d.hit.y)<1e-6){labelDrag=null;return;}
    d.pending=true;
    try {await onlabelcommit(d.id,d.hit.series,d.hit.point,d.layout.x,d.layout.y);}
    finally {if(labelDrag===d)labelDrag=null;}
  }
  function displayedObject(o:ObjectDTO):ObjectDTO {
    const d=labelDrag;
    if(d?.id!==o.id||!d.layout||!o.chart)return o;
    return {...o,chart:chartWithLabelLayout(o.chart,d.hit.series,d.hit.point,d.layout)};
  }

  // An object being dragged: which edges move (move = all).
  let drag = $state<{ id: number; edges: string; x0: number; y0: number; start: Rect; now: Rect } | null>(null);

  function rect(o: ObjectDTO): Rect {
    void tick;
    if (drag && drag.id === o.id) return drag.now;
    return rectOf(o);
  }

  function onDown(e: PointerEvent, o: ObjectDTO, edges: string): void {
    if (e.button !== 0 || labelDrag || o.readonly && !o.shape) return;
    selectedLabel=null;
    e.preventDefault();
    e.stopPropagation();
    onselect(o.id);
    if (o.readonly || editing === o.id) return;
    const start = rectOf(o);
    drag = { id: o.id, edges, x0: e.clientX, y0: e.clientY, start, now: start };
    if (e.isTrusted) (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
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
      const hits:ChartLabelHit[]=[];
      drawChart(ctx, a.w / a.scale, a.h / a.scale, a.o.chart, toChartData(a.data), LIGHT_THEME,hits);
      if(JSON.stringify(labelHits.get(a.o.id))!==JSON.stringify(hits))labelHits=new Map(labelHits).set(a.o.id,hits);
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
      class:readonly={!!o.readonly && !o.shape}
      style:left="{r.x - left}px"
      style:top="{r.y - top}px"
      style:width="{r.w}px"
      style:height="{r.h}px"
      onpointerdown={(e) => onDown(e, o, 'move')}
      onpointermove={onMove}
      onpointerup={onUp}
      onpointercancel={onUp}
      ondblclick={() => (o.kind === 'chart' || canEditShapeText(o)) && onedit(o.id)}
    >
      {#if o.shape}
        <canvas use:shape={{ o: editing === o.id ? {...o, shape:{...o.shape,runs:[]}} : o, w: r.w, h: r.h, scale, tick: tick + fontEpoch }} style:width="{r.w}px" style:height="{r.h}px"></canvas>
      {:else if o.kind === 'chart'}
        <canvas use:chart={{ o:displayedObject(o), w: r.w, h: r.h, data: chartData.get(o.id), scale }} style:width="{r.w}px" style:height="{r.h}px"></canvas>
      {:else if images.get(o.imageKey)}
        <img src={images.get(o.imageKey)} alt="" draggable="false" />
      {/if}
      {#if o.chart && !o.readonly}
        {#each labelHits.get(o.id) ?? [] as hit (`${hit.series}:${hit.point}`)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="chart-label-hit" class:label-selected={selected===o.id&&selectedLabel?.id===o.id&&selectedLabel.series===hit.series&&selectedLabel.point===hit.point}
            data-label-series={hit.series} data-label-point={hit.point}
            title={t('拖曳以移動資料標籤')}
            style:left="{hit.x*scale}px" style:top="{hit.y*scale}px" style:width="{hit.w*scale}px" style:height="{hit.h*scale}px"
            style:transform="rotate({hit.rotation}deg)"
            onpointerdown={e=>onLabelDown(e,o,hit)} onpointermove={onLabelMove} onpointerup={e=>void onLabelUp(e)}
            onpointercancel={cancelLabelDrag} onlostpointercapture={cancelLabelDrag}
          ></div>
        {/each}
      {/if}
      {#if selected === o.id && editing !== o.id && !o.readonly}
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
  import type { ObjectDTO as LayerObject } from '../bridge/sheet';
  export function canEditShapeText(o:LayerObject):boolean {
    return !!o.shape && (!['line','custom'].includes(o.shape.geometry) || (o.shape.runs??[]).some(r=>r.text.length>0));
  }
  import type { ChartDataDTO as Data } from '../bridge/sheet';
  import type { ChartData as Drawn } from '../charts/render';

  export function toChartData(d: Data | undefined): Drawn {
    if (!d) return { categories: [], series: [] };
    return {
      categories: d.categories ?? [],
      valueScale:d.valueScale,
      xScale:d.xScale,
      secondaryScale:d.secondaryScale,
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
  .chart-label-hit { position:absolute; box-sizing:border-box; cursor:grab; touch-action:none; border:1px solid transparent; }
  .chart-label-hit.label-selected { border-color:var(--accent); border-style:dashed; }
  .chart-label-hit:active { cursor:grabbing; }
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
