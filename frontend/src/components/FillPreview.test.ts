import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import FillPreview from './FillPreview.svelte';
import { createFormatDraft, formatDraftPatch } from './cellFormatDraft';

describe('solid fill preview and draft', () => {
  it('shows no fill as transparent with an explicit label', () => {
    const { body } = render(FillPreview, { props: { color: '' } });
    expect(body).toContain('background: transparent');
    expect(body).toContain('無填滿');
  });
  it('shows white as an opaque color distinct from no fill', () => {
    const { body } = render(FillPreview, { props: { color: 'FFFFFF' } });
    expect(body).toContain('background: #FFFFFF');
    expect(body).toContain('#FFFFFF');
    expect(body).not.toContain('無填滿');
  });
  it('does not mutate or submit an unconfirmed fill draft', () => {
    const initial = createFormatDraft(null), draft = { ...initial, fill: 'FFFFFF' };
    expect(initial.fill).toBe('');
    expect(formatDraftPatch(initial, draft)).toEqual({ fill: 'FFFFFF' });
    draft.fill = '';
    expect(formatDraftPatch(initial, draft)).toEqual({});
  });
});

it('does not display the first cell color as a common fill for mixed selections', () => {
  const { body } = render(FillPreview, { props: { color: 'FF0000', mixed: true } });
  expect(body).toContain('混合');
  expect(body).toContain('background: transparent');
  expect(body).not.toContain('#FF0000');
});
