pub(crate) mod atomic_replace;
mod browser;
mod operations;
pub(crate) mod exclusion_paths;
mod scan_exclusions;
pub use browser::{
    list_directory, search_directory, FileBrowserEntry, FileBrowserListing, FileBrowserSearchResult,
};
pub use scan_exclusions::ScanExclusionOptions;
pub use operations::{create_entry, rename_entry, transfer_entries, trash_entries, FileTransferMode, FileMutationReport, FileMutationFailure, FileTextPreview, preview_text};
mod directory_selection;
pub(crate) mod metadata;
mod models;
pub(crate) mod permanent_delete;
#[cfg(windows)]
pub(crate) mod permanent_delete_diagnostics;

pub use directory_selection::{
    DirectorySelectionOutcome, DirectorySelectionService, ResolvedDirectory,
};
pub use models::{
    PermanentDeleteBatchResult, PermanentDeleteCandidate, PermanentDeleteFailure, VolumeInfo,
};
