import { describe, expect, it } from 'vitest';
import { arrowCommit, editAction, idleAction, type KeyLike } from './keymap';

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
    expect(idleAction(key('c', { metaKey: true }))).toEqual({ type: 'menu', action: 'copy' }); // the grid copies (v0.5)
  });

  it('ignores keys during IME composition', () => {
    expect(idleAction(key('ArrowDown', { isComposing: true }))).toBeNull();
    expect(idleAction(key('Enter', { keyCode: 229 }))).toBeNull();
  });
});

describe('editAction', () => {
  it('commits and moves on Enter/Tab', () => {
    expect(editAction(key('Enter'))).toEqual({ type: 'commit', then: 'down' });
    expect(editAction(key('Enter', { shiftKey: true }))).toEqual({ type: 'commit', then: 'up' });
    expect(editAction(key('Tab', { shiftKey: true }))).toEqual({ type: 'commit', then: 'left' });
    expect(editAction(key('Enter', { metaKey: true }))).toEqual({ type: 'commitFill' });
    expect(editAction(key('Enter', { altKey: true }))).toEqual({ type: 'newline' });
    expect(editAction(key('Escape'))).toEqual({ type: 'cancel' });
  });

  it('reports arrows; they commit in Enter mode and move the caret in Edit mode', () => {
    const right = editAction(key('ArrowRight'));
    expect(right).toEqual({ type: 'arrow', dr: 0, dc: 1, extend: false });
    expect(arrowCommit({ dr: 0, dc: 1, extend: false }, 'enter')).toEqual({ type: 'commit', then: 'right' });
    expect(arrowCommit({ dr: -1, dc: 0, extend: false }, 'enter')).toEqual({ type: 'commit', then: 'up' });
    expect(arrowCommit({ dr: 0, dc: 1, extend: false }, 'edit')).toBeNull();
    expect(arrowCommit({ dr: 0, dc: 1, extend: true }, 'enter')).toBeNull();
    expect(editAction(key('ArrowLeft', { metaKey: true }))).toBeNull();
  });

  it('toggles absolute references with F4 and ⌘T', () => {
    expect(editAction(key('F4'))).toEqual({ type: 'toggleAbs' });
    expect(editAction(key('t', { metaKey: true }))).toEqual({ type: 'toggleAbs' });
  });

  it('does not commit while the IME confirms a candidate', () => {
    expect(editAction(key('Enter', { keyCode: 229 }))).toBeNull();
    expect(editAction(key('ArrowDown', { isComposing: true }))).toBeNull();
  });
});

describe('v0.5 shortcuts', () => {
  const k = (key: string, mods: Partial<KeyLike> = {}): KeyLike => ({
    key,
    shiftKey: false,
    metaKey: false,
    altKey: false,
    ctrlKey: false,
    ...mods,
  });
  it('maps clipboard and editing commands when idle', () => {
    expect(idleAction(k('c', { metaKey: true }))).toEqual({ type: 'menu', action: 'copy' });
    expect(idleAction(k('v', { metaKey: true, ctrlKey: true }))).toEqual({ type: 'menu', action: 'pasteSpecial' });
    expect(idleAction(k('+', { metaKey: true, shiftKey: true }))).toEqual({ type: 'menu', action: 'insertCells' });
    expect(idleAction(k('-', { metaKey: true }))).toEqual({ type: 'menu', action: 'deleteCells' });
    expect(idleAction(k('(', { ctrlKey: true, shiftKey: true }))).toEqual({ type: 'menu', action: 'unhideRows' });
    expect(idleAction(k('f', { metaKey: true, shiftKey: true }))).toEqual({ type: 'menu', action: 'filter' });
    expect(idleAction(k('Escape'))).toEqual({ type: 'escape' });
  });
  it('leaves clipboard keys to the text while editing', () => {
    expect(editAction(k('c', { metaKey: true }))).toBeNull();
    expect(editAction(k('v', { metaKey: true }))).toBeNull();
    expect(editAction(k('z', { metaKey: true }))).toEqual({ type: 'menu', action: 'undo' });
  });
});
