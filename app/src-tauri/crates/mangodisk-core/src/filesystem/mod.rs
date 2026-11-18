pub(crate) mod atomic_replace;
mod browser;
pub(crate) mod exclusion_paths;
mod scan_exclusions;
pub use browser::{
    list_directory, search_directory, FileBrowserEntry, FileBrowserListing, FileBrowserSearchResult,
};
pub use scan_exclusions::ScanExclusionOptions;
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
