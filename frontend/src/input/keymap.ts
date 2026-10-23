// Keyboard map for the grid, following Excel for Mac. Pure: maps a key
// event to an action; the sheet controller performs it.

export type EditMode = 'enter' | 'edit';

// Status bar mode, as in Excel: 就緒 / 輸入 / 編輯 / 指向.
export type SheetMode = 'ready' | EditMode | 'point';

export type MenuAction = 'undo' | 'redo' | 'selectAll' | 'new' | 'open' | 'save' | 'saveAs' | 'close';

export type Action =
  // Arrow keys. jump = ⌘ (data edge), extend = Shift.
  | { type: 'move'; dr: number; dc: number; jump: boolean; extend: boolean }
  // Page Up/Down (fn+↑/↓); horizontal with ⌥.
  | { type: 'page'; dir: 1 | -1; horizontal: boolean; extend: boolean }
  | { type: 'rowStart'; extend: boolean } // Home (fn+←)
  | { type: 'docStart'; extend: boolean } // ⌘Home
  | { type: 'docEnd'; extend: boolean } // ⌘End
  | { type: 'enter'; backward: boolean } // Return
  | { type: 'tab'; backward: boolean }
  | { type: 'beginEdit' } // F2
  | { type: 'clear' } // Delete / forward delete
  | { type: 'selectRow' } // Shift+Space
  | { type: 'selectCol' } // ⌃Space
  | { type: 'menu'; action: MenuAction }
  // Editing only
  | { type: 'commit'; then: 'down' | 'up' | 'right' | 'left' | 'stay' }
  | { type: 'commitFill' } // ⌘Return: fill the whole selection
  | { type: 'cancel' }
  | { type: 'newline' } // ⌥Return
  | { type: 'toggleMode' } // F2 while editing
  | { type: 'toggleAbs' } // F4 / ⌘T: cycle $ on the reference at the caret
  // Arrow while editing: the controller decides between point mode,
  // commit-and-move (Enter mode) and moving the caret (Edit mode).
  | { type: 'arrow'; dr: number; dc: number; extend: boolean };

export interface KeyLike {
  key: string;
  shiftKey: boolean;
  metaKey: boolean;
  altKey: boolean;
  ctrlKey: boolean;
  isComposing?: boolean;
  keyCode?: number;
}

// IME composition in progress. WebKit fires the keydown that confirms a
// candidate with keyCode 229 after compositionend, so check both.
export function isComposing(e: KeyLike): boolean {
  return e.isComposing === true || e.keyCode === 229;
}

const ARROWS: Record<string, [number, number]> = {
  ArrowUp: [-1, 0],
  ArrowDown: [1, 0],
  ArrowLeft: [0, -1],
  ArrowRight: [0, 1],
};

// ⌘ shortcuts that also exist as menu items. The menu may or may not see
// the key first inside WKWebView, so both paths dispatch through the
// same de-duplicating handler.
function menuShortcut(e: KeyLike): MenuAction | null {
  if (!e.metaKey || e.altKey || e.ctrlKey) return null;
  switch (e.key.toLowerCase()) {
    case 'z':
      return e.shiftKey ? 'redo' : 'undo';
    case 'a':
      return e.shiftKey ? null : 'selectAll';
    case 'n':
      return e.shiftKey ? null : 'new';
    case 'o':
      return e.shiftKey ? null : 'open';
    case 's':
      return e.shiftKey ? 'saveAs' : 'save';
    case 'w':
      return e.shiftKey ? null : 'close';
    default:
      return null;
  }
}

export function idleAction(e: KeyLike): Action | null {
  if (isComposing(e)) return null;
  const extend = e.shiftKey;

  const menu = menuShortcut(e);
  if (menu) return { type: 'menu', action: menu };

  const arrow = ARROWS[e.key];
  if (arrow && !e.altKey && !e.ctrlKey) {
    return { type: 'move', dr: arrow[0], dc: arrow[1], jump: e.metaKey, extend };
  }

  switch (e.key) {
    case 'PageDown':
    case 'PageUp':
      return { type: 'page', dir: e.key === 'PageDown' ? 1 : -1, horizontal: e.altKey, extend };
    case 'Home':
      return e.metaKey || e.ctrlKey ? { type: 'docStart', extend } : { type: 'rowStart', extend };
    case 'End':
      return e.metaKey || e.ctrlKey ? { type: 'docEnd', extend } : null;
    case 'Enter':
      if (e.metaKey || e.altKey || e.ctrlKey) return null;
      return { type: 'enter', backward: e.shiftKey };
    case 'Tab':
      if (e.metaKey || e.altKey || e.ctrlKey) return null;
      return { type: 'tab', backward: e.shiftKey };
    case 'F2':
      return { type: 'beginEdit' };
    case 'Backspace':
    case 'Delete':
      return e.metaKey || e.altKey ? null : { type: 'clear' };
    case ' ':
      if (e.shiftKey && !e.metaKey && !e.ctrlKey) return { type: 'selectRow' };
      if (e.ctrlKey && !e.shiftKey && !e.metaKey) return { type: 'selectCol' };
      return null;
    default:
      return null;
  }
}

export function editAction(e: KeyLike): Action | null {
  if (isComposing(e)) return null;

  const menu = menuShortcut(e);
  // Inside the editor ⌘A / ⌘Z act on the text; the controller decides.
  if (menu) return { type: 'menu', action: menu };
  if (e.key === 'F4' || (e.metaKey && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 't')) {
    return { type: 'toggleAbs' };
  }

  switch (e.key) {
    case 'Enter':
      if (e.altKey && !e.metaKey) return { type: 'newline' };
      if (e.metaKey) return { type: 'commitFill' };
      if (e.ctrlKey) return null;
      return { type: 'commit', then: e.shiftKey ? 'up' : 'down' };
    case 'Tab':
      return { type: 'commit', then: e.shiftKey ? 'left' : 'right' };
    case 'Escape':
      return { type: 'cancel' };
    case 'F2':
      return { type: 'toggleMode' };
  }

  const arrow = ARROWS[e.key];
  if (arrow && !e.metaKey && !e.altKey && !e.ctrlKey) {
    return { type: 'arrow', dr: arrow[0], dc: arrow[1], extend: e.shiftKey };
  }
  return null;
}

// What a plain arrow does while editing, when point mode does not apply:
// in Enter mode (started by typing) it commits and moves, like Excel; in
// Edit mode (F2) it moves the caret.
export function arrowCommit(a: { dr: number; dc: number; extend: boolean }, mode: EditMode): Action | null {
  if (mode !== 'enter' || a.extend) return null;
  const then = a.dr < 0 ? 'up' : a.dr > 0 ? 'down' : a.dc < 0 ? 'left' : 'right';
  return { type: 'commit', then };
}
