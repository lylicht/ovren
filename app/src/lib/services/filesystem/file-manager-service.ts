import { invoke } from '@tauri-apps/api/core';
import { dirname } from '@tauri-apps/api/path';
import type { FileBrowserListing, FileBrowserSearchResult } from '@/lib/models/filesystem/file-browser';

/** Exposes native file-system item actions without leaking Tauri into pages. */
export class FileManagerService {
  static parentDirectory(path: string): Promise<string> {
    return dirname(path);
  }

  static listBrowserDirectory(path: string): Promise<FileBrowserListing> {
    return invoke<FileBrowserListing>('list_file_browser_directory', { path });
  }

  static searchBrowserDirectory(path: string, query: string): Promise<FileBrowserSearchResult> {
    return invoke<FileBrowserSearchResult>('search_file_browser_directory', { path, query });
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
}
