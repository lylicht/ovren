import type { FileBrowserEntry } from '@/lib/models/filesystem/file-browser';
import { ICON_NAMES, type IconName } from '@/lib/models/presentation/ui';

export type BrowserSort = 'name' | 'size' | 'modified';
export type BrowserView = 'details' | 'grid';

const IMAGE_TYPES = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'tiff', 'heic']);
const AUDIO_TYPES = new Set(['mp3', 'wav', 'flac', 'm4a', 'aac', 'ogg', 'opus']);
const VIDEO_TYPES = new Set(['mp4', 'mkv', 'mov', 'avi', 'webm']);
const ARCHIVE_TYPES = new Set(['zip', '7z', 'rar', 'tar', 'gz', 'bz2', 'xz']);
const CODE_TYPES = new Set(['rs', 'ts', 'tsx', 'js', 'jsx', 'py', 'go', 'java', 'kt', 'swift', 'html', 'css', 'sh']);
const TEXT_TYPES = new Set(['txt', 'md', 'json', 'toml', 'yaml', 'yml', 'xml', 'ini', 'log', 'csv']);

export function browserEntryIcon(entry: FileBrowserEntry): IconName {
  if (entry.isSymlink) return ICON_NAMES.external;
  if (entry.isDirectory) return ICON_NAMES.folder;
  const extension = entry.name.split('.').pop()?.toLowerCase() ?? '';
  if (IMAGE_TYPES.has(extension)) return ICON_NAMES.fileImage;
  if (AUDIO_TYPES.has(extension)) return ICON_NAMES.fileAudio;
  if (VIDEO_TYPES.has(extension)) return ICON_NAMES.fileVideo;
  if (ARCHIVE_TYPES.has(extension)) return ICON_NAMES.fileArchive;
  if (CODE_TYPES.has(extension)) return ICON_NAMES.fileCode;
  if (TEXT_TYPES.has(extension)) return ICON_NAMES.fileText;
  if (['pdf', 'doc', 'docx', 'odt'].includes(extension)) return ICON_NAMES.fileText;
  if (['xls', 'xlsx', 'ods'].includes(extension)) return ICON_NAMES.fileSpreadsheet;
  if (['ppt', 'pptx', 'odp'].includes(extension)) return ICON_NAMES.filePresentation;
  return ICON_NAMES.file;
}

export function sortBrowserEntries(
  entries: readonly FileBrowserEntry[],
  sort: BrowserSort,
  descending: boolean,
  locale: string
): FileBrowserEntry[] {
  const collator = new Intl.Collator(locale, { numeric: true, sensitivity: 'base' });
  const direction = descending ? -1 : 1;
  return [...entries].sort((left, right) => {
    // Preserve Explorer's folder-first grouping regardless of descending order.
    if (left.isDirectory !== right.isDirectory) return left.isDirectory ? -1 : 1;
    let order = 0;
    if (sort === 'size') order = left.size - right.size;
    if (sort === 'modified') order = (left.modifiedAtMs ?? -1) - (right.modifiedAtMs ?? -1);
    return direction * (order || collator.compare(left.name, right.name));
  });
}

/** Selection math is independent from rendering/virtual scrolling. */
export function browserSelectionRange(entries: readonly FileBrowserEntry[], from: number, to: number): string[] {
  if (from < 0 || to < 0 || from >= entries.length || to >= entries.length) return [];
  return entries.slice(Math.min(from, to), Math.max(from, to) + 1).map(entry => entry.path);
}

export function browserName(path: string): string {
  return (
    path
      .replace(/[\\/]+$/u, '')
      .split(/[\\/]/u)
      .pop() || path
  );
}
