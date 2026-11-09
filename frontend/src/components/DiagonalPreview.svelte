<script lang="ts">
 import { t } from '../i18n';
 import { BORDER_WIDTH, BORDER_DASH, slantedBorderSegments } from '../grid/borders';
 import type { BorderLineDTO } from '../../bindings/ww-excel/internal/service/models';
 let {line,up,down,mixedUp=false,mixedDown=false}:{line:BorderLineDTO;up:boolean;down:boolean;mixedUp?:boolean;mixedDown?:boolean}=$props();
</script>
<svg viewBox="0 0 160 60" width="160" height="60" role="img" aria-label={t('斜線預覽')}>
 <rect x="1" y="1" width="158" height="58" fill="none" stroke="var(--chrome-border)"/>
 {#each [{enabled:up,mixed:mixedUp,p:[1,59,159,1]},{enabled:down,mixed:mixedDown,p:[1,1,159,59]}] as edge}
  {#if edge.enabled || edge.mixed}
   {@const p=edge.p as [number,number,number,number]}
   {#if line.style==='slantDashDot' && !edge.mixed}<path d={slantedBorderSegments(...p).map(([a,b,c,d])=>`M${a},${b}L${c},${d}`).join(' ')} fill="none" stroke={line.color?'#'+line.color:'currentColor'}/>
   {:else}{#each line.style==='double'?[-1,1]:[0] as offset}<line x1={p[0]-(p[3]-p[1])*offset/Math.hypot(p[2]-p[0],p[3]-p[1])} y1={p[1]+(p[2]-p[0])*offset/Math.hypot(p[2]-p[0],p[3]-p[1])} x2={p[2]-(p[3]-p[1])*offset/Math.hypot(p[2]-p[0],p[3]-p[1])} y2={p[3]+(p[2]-p[0])*offset/Math.hypot(p[2]-p[0],p[3]-p[1])} stroke={line.color?'#'+line.color:'currentColor'} stroke-width={line.style==='double'?1:BORDER_WIDTH[line.style || ''] || 1} stroke-dasharray={edge.mixed?'2 4':(BORDER_DASH[line.style || ''] || []).join(' ')} opacity={edge.mixed?.4:1}/>{/each}{/if}
  {/if}
 {/each}
</svg>
