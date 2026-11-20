import { invoke } from '@tauri-apps/api/core';
import { dirname, homeDir } from '@tauri-apps/api/path';
import type { FileBrowserListing, FileBrowserSearchResult, FileMutationReport, FileTransferMode, FileTextPreview } from '@/lib/models/filesystem/file-browser';

/** Exposes native file-system item actions without leaking Tauri into pages. */
export class FileManagerService {
  static homeDirectory(): Promise<string> {
    return homeDir();
  }

  static parentDirectory(path: string): Promise<string> {
    return dirname(path);
  }

  static listBrowserDirectory(path: string): Promise<FileBrowserListing> {
    return invoke<FileBrowserListing>('list_file_browser_directory', { path });
  }

  static searchBrowserDirectory(path: string, query: string): Promise<FileBrowserSearchResult> {
    return invoke<FileBrowserSearchResult>('search_file_browser_directory', { path, query });
  }

  static previewBrowserEntry(path: string): Promise<FileTextPreview> {
    return invoke<FileTextPreview>('preview_file_browser_entry', { path });
  }

  static createBrowserEntry(parent: string, name: string, folder: boolean): Promise<string> {
    return invoke<string>('create_file_browser_entry', { parent, name, folder });
  }

  static renameBrowserEntry(path: string, name: string): Promise<string> {
    return invoke<string>('rename_file_browser_entry', { path, name });
  }

  static transferBrowserEntries(paths: string[], destination: string, mode: FileTransferMode): Promise<FileMutationReport> {
    return invoke<FileMutationReport>('transfer_file_browser_entries', { paths, destination, mode });
  }

  static trashBrowserEntries(paths: string[]): Promise<FileMutationReport> {
    return invoke<FileMutationReport>('trash_file_browser_entries', { paths });
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
