import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import BorderEditor from './BorderEditor.svelte';
import type { StyleDTO } from '../bridge/sheet';

describe('border preview initial state', () => {
  it('renders the original edge color before any edits', () => {
    const original: StyleDTO = { id: 5, font: { name: 'Arial', size: 11 }, top: { style: 'thin', color: '00FF00' }, bottom: {}, left: {}, right: {} };
    const { body } = render(BorderEditor, { props: { value: {}, lineStyle: 'thin', color: '', original, selection: [{r0:0,c0:0,r1:2,c1:2}], merges: [] } });
    expect(body).toContain('stroke="#00FF00"');
    expect(body).toContain('aria-pressed="mixed"');
  });
});

it('uses common range edges and marks mixed edges without showing the first cell color', () => {
  const original: StyleDTO = { id: 5, font: { name: 'Arial', size: 11 }, top: { style: 'thin', color: 'FF0000' }, bottom: {}, left: {}, right: {} };
  const { body } = render(BorderEditor, { props: { value: {}, lineStyle: 'thin', color: '', original, originalEdges: { bottom: { style: 'double', color: '0000FF' }, insideH: { style: 'dotted', color: '00FF00' } }, mixedEdges: ['top'], selection: [{r0:0,c0:0,r1:2,c1:2}], merges: [] } });
  expect(body).toContain('混合');
  expect(body).not.toContain('stroke="#FF0000"');
  expect(body).toContain('stroke="#0000FF"');
  expect(body).toContain('stroke="#00FF00"');
});
