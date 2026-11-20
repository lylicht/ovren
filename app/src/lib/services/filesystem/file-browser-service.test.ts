import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { FileBrowserListing, FileBrowserSearchResult } from '@/lib/models/filesystem/file-browser';
import { FileManagerService } from './file-manager-service';

const invokeMock = vi.hoisted(() => vi.fn());
const dirnameMock = vi.hoisted(() => vi.fn());
const homeDirMock = vi.hoisted(() => vi.fn());

vi.mock('@tauri-apps/api/core', () => ({ invoke: invokeMock }));
vi.mock('@tauri-apps/api/path', () => ({ dirname: dirnameMock, homeDir: homeDirMock }));

describe('file browser service', () => {
  beforeEach(() => invokeMock.mockReset());
  afterEach(() => vi.useRealTimers());

  it('requests a directory listing from the native boundary', async () => {
    const listing: FileBrowserListing = { path: '/home/owner', entries: [], skippedCount: 0 };
    invokeMock.mockResolvedValue(listing);

    await expect(FileManagerService.listBrowserDirectory('/home/owner')).resolves.toEqual(listing);
    expect(invokeMock).toHaveBeenCalledWith('list_file_browser_directory', { path: '/home/owner' });
  });

  it('reuses recent small directory listings and refreshes them on demand', async () => {
    const listing: FileBrowserListing = { path: '/home/cache-check', entries: [], skippedCount: 0 };
    invokeMock.mockResolvedValue(listing);

    await FileManagerService.listBrowserDirectory('/home/cache-check');
    await FileManagerService.listBrowserDirectory('/home/cache-check/');
    expect(invokeMock).toHaveBeenCalledOnce();

    await FileManagerService.listBrowserDirectory('/home/cache-check', true);
    expect(invokeMock).toHaveBeenCalledTimes(2);

    await FileManagerService.listBrowserDirectory('/home/cache-check');
    expect(invokeMock).toHaveBeenCalledTimes(2);
  });

  it('expires cached directory listings after two seconds', async () => {
    const listing: FileBrowserListing = { path: '/home/cache-expiry', entries: [], skippedCount: 0 };
    invokeMock.mockResolvedValue(listing);
    vi.useFakeTimers();

    await FileManagerService.listBrowserDirectory('/home/cache-expiry');
    vi.advanceTimersByTime(2001);
    await FileManagerService.listBrowserDirectory('/home/cache-expiry');

    expect(invokeMock).toHaveBeenCalledTimes(2);
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

  it('loads the current user home directory from the native path adapter', async () => {
    homeDirMock.mockResolvedValue('/home/owner');
    await expect(FileManagerService.homeDirectory()).resolves.toBe('/home/owner');
    expect(homeDirMock).toHaveBeenCalled();
  });

  it('loads a bounded text preview from Rust', async () => {
    invokeMock.mockResolvedValue({ text: 'sample', truncated: false });
    await expect(FileManagerService.previewBrowserEntry('/home/owner/sample.txt')).resolves.toEqual({
      text: 'sample',
      truncated: false,
    });
    expect(invokeMock).toHaveBeenCalledWith('preview_file_browser_entry', { path: '/home/owner/sample.txt' });
  });

  it('creates and renames through guarded Rust commands', async () => {
    invokeMock.mockResolvedValue('/home/owner/created.txt');
    await FileManagerService.createBrowserEntry('/home/owner', 'created.txt', false);
    expect(invokeMock).toHaveBeenCalledWith('create_file_browser_entry', {
      parent: '/home/owner',
      name: 'created.txt',
      folder: false,
    });
    await FileManagerService.renameBrowserEntry('/home/owner/created.txt', 'renamed.txt');
    expect(invokeMock).toHaveBeenCalledWith('rename_file_browser_entry', {
      path: '/home/owner/created.txt',
      name: 'renamed.txt',
    });
  });

  it('invalidates directory listings around filesystem mutations', async () => {
    const listing: FileBrowserListing = { path: '/home/cache-mutation', entries: [], skippedCount: 0 };
    invokeMock.mockResolvedValue(listing);
    await FileManagerService.listBrowserDirectory('/home/cache-mutation');
    await FileManagerService.createBrowserEntry('/home/cache-mutation', 'new.txt', false);
    await FileManagerService.listBrowserDirectory('/home/cache-mutation');

    expect(invokeMock).toHaveBeenCalledTimes(3);
  });

  it('preserves per-item copy, move and trash results from native commands', async () => {
    const report = { completed: ['/home/owner/item'], failed: [] };
    invokeMock.mockResolvedValue(report);
    await expect(FileManagerService.transferBrowserEntries(['/home/owner/item'], '/tmp', 'copy')).resolves.toEqual(
      report
    );
    expect(invokeMock).toHaveBeenCalledWith('transfer_file_browser_entries', {
      paths: ['/home/owner/item'],
      destination: '/tmp',
      mode: 'copy',
    });
    await FileManagerService.transferBrowserEntries(['/home/owner/item'], '/tmp', 'move');
    expect(invokeMock).toHaveBeenCalledWith('transfer_file_browser_entries', {
      paths: ['/home/owner/item'],
      destination: '/tmp',
      mode: 'move',
    });
    await FileManagerService.trashBrowserEntries(['/home/owner/item']);
    expect(invokeMock).toHaveBeenCalledWith('trash_file_browser_entries', { paths: ['/home/owner/item'] });
  });

  it('resolves a directory parent through the platform path service', async () => {
    dirnameMock.mockResolvedValue('/home/owner');

    await expect(FileManagerService.parentDirectory('/home/owner/notes.txt')).resolves.toBe('/home/owner');
    expect(dirnameMock).toHaveBeenCalledWith('/home/owner/notes.txt');
  });
});
