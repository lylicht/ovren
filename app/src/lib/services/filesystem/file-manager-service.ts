import { invoke } from '@tauri-apps/api/core';
import { dirname, homeDir } from '@tauri-apps/api/path';
import * as PathUtils from '@/lib/utils/filesystem/path';
import type {
  FileBrowserListing,
  FileBrowserSearchResult,
  FileMutationReport,
  FileTransferMode,
  FileTextPreview,
} from '@/lib/models/filesystem/file-browser';

const DIRECTORY_CACHE_TTL_MS = 2000;
const DIRECTORY_CACHE_LIMIT = 16;
const DIRECTORY_CACHE_ENTRY_LIMIT = 4096;

/** Exposes native file-system item actions without leaking Tauri into pages. */
export class FileManagerService {
  private static readonly browserListingCache = new Map<string, { expiresAt: number; listing: FileBrowserListing }>();

  static homeDirectory(): Promise<string> {
    return homeDir();
  }

  static parentDirectory(path: string): Promise<string> {
    return dirname(path);
  }

  static async listBrowserDirectory(path: string, refresh = false): Promise<FileBrowserListing> {
    const key = PathUtils.comparisonKey(path);
    const cached = key ? this.browserListingCache.get(key) : undefined;
    if (!refresh && cached && cached.expiresAt > Date.now()) {
      this.browserListingCache.delete(key);
      this.browserListingCache.set(key, cached);
      return cached.listing;
    }
    if (key) this.browserListingCache.delete(key);

    const listing = await invoke<FileBrowserListing>('list_file_browser_directory', { path });
    if (key && listing.entries.length <= DIRECTORY_CACHE_ENTRY_LIMIT) {
      this.browserListingCache.set(key, { listing, expiresAt: Date.now() + DIRECTORY_CACHE_TTL_MS });
      while (this.browserListingCache.size > DIRECTORY_CACHE_LIMIT) {
        this.browserListingCache.delete(this.browserListingCache.keys().next().value!);
      }
    }
    return listing;
  }

  static searchBrowserDirectory(path: string, query: string): Promise<FileBrowserSearchResult> {
    return invoke<FileBrowserSearchResult>('search_file_browser_directory', { path, query });
  }

  static previewBrowserEntry(path: string): Promise<FileTextPreview> {
    return invoke<FileTextPreview>('preview_file_browser_entry', { path });
  }

  static createBrowserEntry(parent: string, name: string, folder: boolean): Promise<string> {
    return this.mutateBrowserEntries(() => invoke<string>('create_file_browser_entry', { parent, name, folder }));
  }

  static renameBrowserEntry(path: string, name: string): Promise<string> {
    return this.mutateBrowserEntries(() => invoke<string>('rename_file_browser_entry', { path, name }));
  }

  static transferBrowserEntries(
    paths: string[],
    destination: string,
    mode: FileTransferMode
  ): Promise<FileMutationReport> {
    return this.mutateBrowserEntries(() =>
      invoke<FileMutationReport>('transfer_file_browser_entries', { paths, destination, mode })
    );
  }

  static trashBrowserEntries(paths: string[]): Promise<FileMutationReport> {
    return this.mutateBrowserEntries(() => invoke<FileMutationReport>('trash_file_browser_entries', { paths }));
  }

  static openBrowserEntry(path: string): Promise<void> {
    return invoke<void>('open_file_browser_entry', { path });
  }

  static openAnalysisEntry(scanId: number, selectedPath: string): Promise<void> {
    return invoke<void>('open_analysis_entry', { scanId, selectedPath });
  }

  static openLargeFileEntry(scanId: number, selectedPath: string): Promise<void> {
    return invoke<void>('open_large_file_entry', { scanId, selectedPath });
  }

  static openDuplicateFileEntry(scanId: number, selectedPath: string): Promise<void> {
    return invoke<void>('open_duplicate_file_entry', { scanId, selectedPath });
  }

  static reveal(path: string): Promise<void> {
    return invoke<void>('reveal_in_file_manager', { path });
  }

  static openApplicationLogs(): Promise<void> {
    return invoke<void>('open_application_log_directory');
  }

  private static async mutateBrowserEntries<T>(action: () => Promise<T>): Promise<T> {
    this.browserListingCache.clear();
    try {
      return await action();
    } finally {
      this.browserListingCache.clear();
    }
  }
}
