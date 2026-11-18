export interface FileBrowserEntry {
  name: string;
  path: string;
  isDirectory: boolean;
  isSymlink: boolean;
  size: number;
  modifiedAtMs: number | null;
}

export interface FileBrowserListing {
  path: string;
  entries: FileBrowserEntry[];
  skippedCount: number;
}

export interface FileBrowserSearchResult {
  root: string;
  query: string;
  entries: FileBrowserEntry[];
  scannedEntries: number;
  skippedCount: number;
  truncated: boolean;
}
