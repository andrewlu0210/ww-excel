import { describe, expect, it } from 'vitest';
import { fontFamilies, fontLabel } from './fonts';

describe('fontFamilies', () => {
  it('maps Windows Chinese fonts to macOS ones', () => {
    expect(fontFamilies('新細明體')).toEqual(['新細明體', 'LiSong Pro', 'Apple LiSung', 'Songti TC']);
    expect(fontFamilies('DFKai-SB')[1]).toBe('BiauKaiTC');
    expect(fontFamilies('Arial')).toEqual(['Arial']);
  });
  it('labels macOS fonts in Chinese', () => {
    expect(fontLabel('Songti TC')).toBe('宋體-繁');
    expect(fontLabel('Comic Sans MS')).toBe('Comic Sans MS');
  });
});
