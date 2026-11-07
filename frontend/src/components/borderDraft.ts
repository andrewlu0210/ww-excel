import type { BorderLineDTO } from '../../bindings/ww-excel/internal/service/models';
export type BorderEdge = 'top' | 'bottom' | 'left' | 'right' | 'insideH' | 'insideV';
export type BorderEdits = Partial<Record<BorderEdge, BorderLineDTO>>;
export const BORDER_EDGES: BorderEdge[] = ['top', 'bottom', 'left', 'right', 'insideH', 'insideV'];

export function toggleBorderEdge(draft: BorderEdits, edge: BorderEdge, brush: BorderLineDTO): BorderEdits {
  const next = { ...draft }, current = draft[edge];
  if (current === undefined || (current.style && (current.style !== brush.style || (current.color ?? '') !== (brush.color ?? '')))) next[edge] = { ...brush };
  else if (current.style) next[edge] = {};
  else delete next[edge];
  return next;
}
export function borderPresetDraft(draft: BorderEdits, preset: 'none' | 'outside' | 'inside' | 'all', brush: BorderLineDTO): BorderEdits {
  const next = { ...draft };
  for (const edge of BORDER_EDGES) {
    if (preset === 'outside' && edge.startsWith('inside')) continue;
    if (preset === 'inside' && !edge.startsWith('inside')) continue;
    next[edge] = preset === 'none' ? {} : { ...brush };
  }
  return next;
}
export function borderLineForPreview(draft: BorderEdits, edge: BorderEdge, original?: BorderLineDTO): BorderLineDTO {
  return draft[edge] ?? original ?? {};
}

// Read the selection summary rather than treating the brush's default as the
// saved border color. Mixed range edges must not fall back to the active cell.
export function initialBorderBrush(
  style: import('../bridge/sheet').StyleDTO | null,
  original?: Partial<Record<BorderEdge, BorderLineDTO | null>> | null,
  mixed: string[] = [],
): { style: string; color: string; mixed: boolean; styleMixed: boolean } {
  const lines = BORDER_EDGES.flatMap(edge => {
    const line = original ? original[edge] : edge.startsWith('inside') ? undefined : style?.[edge as 'top'];
    return line?.style ? [line] : [];
  });
  if (style?.diagonal?.style && (style.diagonalUp || style.diagonalDown)) lines.push(style.diagonal);
  const colors = new Set(lines.map(line => line.color ?? ''));
  const styles = new Set(lines.map(line => line.style));
  const unknown = mixed.some(key => BORDER_EDGES.includes(key as BorderEdge) || key === 'diagonal');
  return { style: styles.size === 1 ? lines[0].style! : 'thin', color: colors.size === 1 ? [...colors][0] : '', mixed: unknown || colors.size > 1, styleMixed: unknown || styles.size > 1 };
}

// Brush edits immediately preview on the last chosen edges. Retain the prior
// draft so choosing a different target can use the brush without recoloring
// the previous group (e.g. red outside followed by blue inside).
export function previewBorderBrush(
  draft: BorderEdits, targets: BorderEdge[], baseline: BorderEdits | null,
  brush: BorderLineDTO,
): { value: BorderEdits; baseline: BorderEdits | null } {
  if (!targets.some(edge => draft[edge]?.style)) return { value: draft, baseline };
  const value = { ...draft };
  for (const edge of targets) if (value[edge]?.style) value[edge] = { ...brush };
  return { value, baseline: baseline ?? draft };
}
