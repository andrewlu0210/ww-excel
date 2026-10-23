import { describe, expect, it } from 'vitest';
import { editAction, idleAction, type KeyLike } from './keymap';

const key = (k: string, mods: Partial<KeyLike> = {}): KeyLike => ({
  key: k,
  shiftKey: false,
  metaKey: false,
  altKey: false,
  ctrlKey: false,
  ...mods,
});

describe('idleAction', () => {
  it.each([
    [key('ArrowDown'), { type: 'move', dr: 1, dc: 0, jump: false, extend: false }],
    [key('ArrowLeft', { metaKey: true, shiftKey: true }), { type: 'move', dr: 0, dc: -1, jump: true, extend: true }],
    [key('PageDown', { altKey: true }), { type: 'page', dir: 1, horizontal: true, extend: false }],
    [key('Home'), { type: 'rowStart', extend: false }],
    [key('Home', { metaKey: true }), { type: 'docStart', extend: false }],
    [key('End', { metaKey: true, shiftKey: true }), { type: 'docEnd', extend: true }],
    [key('Enter', { shiftKey: true }), { type: 'enter', backward: true }],
    [key('Tab'), { type: 'tab', backward: false }],
    [key('F2'), { type: 'beginEdit' }],
    [key('Backspace'), { type: 'clear' }],
    [key(' ', { shiftKey: true }), { type: 'selectRow' }],
    [key(' ', { ctrlKey: true }), { type: 'selectCol' }],
    [key('z', { metaKey: true }), { type: 'menu', action: 'undo' }],
    [key('Z', { metaKey: true, shiftKey: true }), { type: 'menu', action: 'redo' }],
    [key('s', { metaKey: true, shiftKey: true }), { type: 'menu', action: 'saveAs' }],
  ])('%o', (e, want) => {
    expect(idleAction(e)).toEqual(want);
  });

  it('leaves printable keys to the text input', () => {
    expect(idleAction(key('a'))).toBeNull();
    expect(idleAction(key(' '))).toBeNull();
    expect(idleAction(key('1'))).toBeNull();
    expect(idleAction(key('c', { metaKey: true }))).toBeNull(); // native Copy role
  });

  it('ignores keys during IME composition', () => {
    expect(idleAction(key('ArrowDown', { isComposing: true }))).toBeNull();
    expect(idleAction(key('Enter', { keyCode: 229 }))).toBeNull();
  });
});

describe('editAction', () => {
  it('commits and moves on Enter/Tab', () => {
    expect(editAction(key('Enter'), 'edit')).toEqual({ type: 'commit', then: 'down' });
    expect(editAction(key('Enter', { shiftKey: true }), 'edit')).toEqual({ type: 'commit', then: 'up' });
    expect(editAction(key('Tab', { shiftKey: true }), 'edit')).toEqual({ type: 'commit', then: 'left' });
    expect(editAction(key('Enter', { metaKey: true }), 'edit')).toEqual({ type: 'commitFill' });
    expect(editAction(key('Enter', { altKey: true }), 'edit')).toEqual({ type: 'newline' });
    expect(editAction(key('Escape'), 'enter')).toEqual({ type: 'cancel' });
  });

  it('arrows commit in Enter mode but move the caret in Edit mode', () => {
    expect(editAction(key('ArrowRight'), 'enter')).toEqual({ type: 'commit', then: 'right' });
    expect(editAction(key('ArrowRight'), 'edit')).toBeNull();
    expect(editAction(key('ArrowRight', { shiftKey: true }), 'enter')).toBeNull();
  });

  it('does not commit while the IME confirms a candidate', () => {
    expect(editAction(key('Enter', { keyCode: 229 }), 'enter')).toBeNull();
    expect(editAction(key('ArrowDown', { isComposing: true }), 'enter')).toBeNull();
  });
});
