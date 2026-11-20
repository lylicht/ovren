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

/** Rust-owned browser mutation contract. Batch failures never overwrite targets. */
export type FileTransferMode = 'copy' | 'move';

export interface FileMutationFailure {
  path: string;
  reason: 'alreadyExists' | 'notFound' | 'permissionDenied' | 'invalidInput' | 'failed';
}

export interface FileMutationReport {
  completed: string[];
  failed: FileMutationFailure[];
}

export interface FileTextPreview {
  text: string;
  truncated: boolean;
}
