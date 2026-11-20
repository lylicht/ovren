export const STANDARD_SCAN_FOLDER_IDS = {
  desktop: 'desktop',
  downloads: 'downloads',
  documents: 'documents',
  pictures: 'pictures',
  videos: 'videos',
  music: 'music',
} as const;

export type StandardScanFolderId = (typeof STANDARD_SCAN_FOLDER_IDS)[keyof typeof STANDARD_SCAN_FOLDER_IDS];

export interface StandardScanFolder {
  id: StandardScanFolderId;
  path: string;
}
