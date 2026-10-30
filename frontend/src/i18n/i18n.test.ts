import { describe, expect, it } from 'vitest';
import { setLanguage, t, tr } from './index';

describe('t', () => {
  it('returns the source text in Chinese', () => {
    setLanguage('zh-TW', { 確定: 'OK' });
    expect(t('確定')).toBe('確定');
    expect(t('已完成取代 %d 項。', 3)).toBe('已完成取代 3 項。');
  });
  it('translates and fills placeholders in English', () => {
    setLanguage('en', { 確定: 'OK', '已完成取代 %d 項。': 'Replaced %d items.' });
    expect(t('確定')).toBe('OK');
    expect(t('已完成取代 %d 項。', 3)).toBe('Replaced 3 items.');
    expect(t('沒有翻譯')).toBe('沒有翻譯');
  });
});

describe('tr', () => {
  it('translates formatted messages, nesting included', () => {
    setLanguage('en', {
      '這裡不能出現「%s」': 'Unexpected "%s"',
      '%s（位置 %d）': '%s (position %d)',
      '已經有名為「%s」的工作表': 'A sheet named "%s" already exists',
    });
    expect(tr('這裡不能出現「)」（位置 4）')).toBe('Unexpected ")" (position 4)');
    expect(tr('已經有名為「預算」的工作表')).toBe('A sheet named "預算" already exists');
    expect(tr('不在表中')).toBe('不在表中');
  });
});

describe('Simplified Chinese', () => {
 it('translates UI and formatted messages without changing document names', () => {
  setLanguage('zh-CN', {'儲存':'保存', '每 %d 分鐘':'每 %d 分钟', '無法開啟「%s」':'无法打开「%s」'});
  expect(t('儲存')).toBe('保存');
  expect(t('每 %d 分鐘', 5)).toBe('每 5 分钟');
  expect(tr('無法開啟「客戶資料.xlsx」')).toBe('无法打开「客戶資料.xlsx」');
  expect(tr('客戶資料.xlsx')).toBe('客戶資料.xlsx');
  setLanguage('zh-TW', null);
  expect(t('儲存')).toBe('儲存');
 });
});
