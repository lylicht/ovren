import { describe, expect, it } from 'vitest';
import type { FileBrowserEntry } from '@/lib/models/filesystem/file-browser';
import { browserEntryIcon, browserName, browserSelectionRange, sortBrowserEntries } from './file-browser';

const item = (name: string, size: number, modifiedAtMs: number | null, isDirectory = false): FileBrowserEntry => ({
  name,
  path: '/fixture/' + name,
  isDirectory,
  isSymlink: false,
  size,
  modifiedAtMs,
});

describe('file explorer presentation', () => {
  const entries = [item('file10.txt', 10, 1), item('folder', 0, null, true), item('file2.txt', 2, 3)];

  it('sorts folders first using numeric-natural filenames', () => {
    expect(sortBrowserEntries(entries, 'name', false, 'en-US').map(entry => entry.name)).toEqual([
      'folder',
      'file2.txt',
      'file10.txt',
    ]);
    expect(sortBrowserEntries(entries, 'name', true, 'en-US').map(entry => entry.name)).toEqual([
      'folder',
      'file10.txt',
      'file2.txt',
    ]);
  });

  it('sorts by size and modification date without mutating source entries', () => {
    const before = entries.map(entry => entry.name);
    expect(sortBrowserEntries(entries, 'size', true, 'en-US').map(entry => entry.name)).toEqual([
      'folder',
      'file10.txt',
      'file2.txt',
    ]);
    expect(sortBrowserEntries(entries, 'modified', true, 'en-US').map(entry => entry.name)).toEqual([
      'folder',
      'file2.txt',
      'file10.txt',
    ]);
    expect(entries.map(entry => entry.name)).toEqual(before);
  });

  it('selects a bounded inclusive shift range', () => {
    expect(browserSelectionRange(entries, 2, 0)).toEqual(entries.map(entry => entry.path));
    expect(browserSelectionRange(entries, -1, 1)).toEqual([]);
    expect(browserSelectionRange(entries, 0, 10)).toEqual([]);
  });

  it('uses category icons without native icon loading on every scroll', () => {
    expect(browserEntryIcon(item('photo.webp', 100, 0))).toBe('fileImage');
    expect(browserEntryIcon(item('program.rs', 100, 0))).toBe('fileCode');
    expect(browserEntryIcon(item('folder', 0, 0, true))).toBe('folder');
    expect(browserName('C:\\Users\\Owner\\Documents')).toBe('Documents');
    expect(browserName('/')).toBe('/');
  });
});
