import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { FileBrowserListing, FileBrowserSearchResult } from '@/lib/models/filesystem/file-browser';
import { FileManagerService } from './file-manager-service';

const invokeMock = vi.hoisted(() => vi.fn());
const dirnameMock = vi.hoisted(() => vi.fn());

vi.mock('@tauri-apps/api/core', () => ({ invoke: invokeMock }));
vi.mock('@tauri-apps/api/path', () => ({ dirname: dirnameMock }));

describe('file browser service', () => {
  beforeEach(() => invokeMock.mockReset());

  it('requests a directory listing from the native boundary', async () => {
    const listing: FileBrowserListing = { path: '/home/owner', entries: [], skippedCount: 0 };
    invokeMock.mockResolvedValue(listing);

    await expect(FileManagerService.listBrowserDirectory('/home/owner')).resolves.toEqual(listing);
    expect(invokeMock).toHaveBeenCalledWith('list_file_browser_directory', { path: '/home/owner' });
  });

  it('passes the current folder and query to recursive search', async () => {
    const result: FileBrowserSearchResult = {
      root: '/home/owner',
      query: 'notes',
      entries: [],
      scannedEntries: 7,
      skippedCount: 0,
      truncated: false,
    };
    invokeMock.mockResolvedValue(result);

    await expect(FileManagerService.searchBrowserDirectory('/home/owner', 'notes')).resolves.toEqual(result);
    expect(invokeMock).toHaveBeenCalledWith('search_file_browser_directory', {
      path: '/home/owner',
      query: 'notes',
    });
  });

  it('opens a selected file through its system handler', async () => {
    invokeMock.mockResolvedValue(undefined);

    await expect(FileManagerService.openBrowserEntry('/home/owner/notes.txt')).resolves.toBeUndefined();
    expect(invokeMock).toHaveBeenCalledWith('open_file_browser_entry', { path: '/home/owner/notes.txt' });
  });

  it('resolves a directory parent through the platform path service', async () => {
    dirnameMock.mockResolvedValue('/home/owner');

    await expect(FileManagerService.parentDirectory('/home/owner/notes.txt')).resolves.toBe('/home/owner');
    expect(dirnameMock).toHaveBeenCalledWith('/home/owner/notes.txt');
  });
});
