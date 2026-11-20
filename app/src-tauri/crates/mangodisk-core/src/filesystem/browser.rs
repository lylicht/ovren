use std::{
    fs,
    io::{self, ErrorKind},
    path::{Path, PathBuf},
    time::UNIX_EPOCH,
};

use mangodisk_platform::{current_platform, Platform};
use serde::Serialize;

const MAX_SEARCH_ENTRIES: usize = 100_000;
const MAX_SEARCH_RESULTS: usize = 500;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FileBrowserEntry {
    pub name: String,
    pub path: String,
    pub is_directory: bool,
    pub is_symlink: bool,
    pub size: u64,
    pub modified_at_ms: Option<u64>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FileBrowserListing {
    pub path: String,
    pub entries: Vec<FileBrowserEntry>,
    pub skipped_count: usize,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FileBrowserSearchResult {
    pub root: String,
    pub query: String,
    pub entries: Vec<FileBrowserEntry>,
    pub scanned_entries: usize,
    pub skipped_count: usize,
    pub truncated: bool,
}

pub fn list_directory(path: &Path) -> io::Result<FileBrowserListing> {
    let root = validated_directory(path)?;
    let (mut entries, skipped_count) = read_entries(&root)?;
    sort_entries(&mut entries);

    Ok(FileBrowserListing {
        path: current_platform().display_path(&root),
        entries,
        skipped_count,
    })
}

pub fn search_directory(root: &Path, query: &str) -> io::Result<FileBrowserSearchResult> {
    let query = query.trim();
    if query.is_empty() {
        return Err(io::Error::new(
            ErrorKind::InvalidInput,
            "search query is empty",
        ));
    }

    let root = validated_directory(root)?;
    let needle = query.to_lowercase();
    let mut pending = vec![root.clone()];
    let mut entries = Vec::new();
    let mut scanned_entries = 0;
    let mut skipped_count = 0;
    let mut truncated = false;

    'directories: while let Some(directory) = pending.pop() {
        let children = match fs::read_dir(&directory) {
            Ok(children) => children,
            Err(_) => {
                skipped_count += 1;
                continue;
            }
        };

        for child in children {
            if scanned_entries == MAX_SEARCH_ENTRIES {
                truncated = true;
                break 'directories;
            }
            let child = match child {
                Ok(child) => child,
                Err(_) => {
                    skipped_count += 1;
                    continue;
                }
            };
            scanned_entries += 1;

            let path = child.path();
            let file_type = match child.file_type() {
                Ok(file_type) => file_type,
                Err(_) => {
                    skipped_count += 1;
                    continue;
                }
            };
            let name = child.file_name().to_string_lossy().into_owned();
            if name.to_lowercase().contains(&needle) {
                if entries.len() == MAX_SEARCH_RESULTS {
                    truncated = true;
                    break 'directories;
                }
                match browser_entry(path.clone(), file_type.is_dir(), file_type.is_symlink()) {
                    Ok(entry) => entries.push(entry),
                    Err(_) => skipped_count += 1,
                }
            }
            if file_type.is_dir() {
                pending.push(path);
            }
        }
    }

    sort_entries(&mut entries);

    Ok(FileBrowserSearchResult {
        root: current_platform().display_path(&root),
        query: query.to_string(),
        entries,
        scanned_entries,
        skipped_count,
        truncated,
    })
}

fn validated_directory(path: &Path) -> io::Result<PathBuf> {
    if path.as_os_str().is_empty() {
        return Err(io::Error::new(
            ErrorKind::InvalidInput,
            "directory path is empty",
        ));
    }
    let canonical = fs::canonicalize(path)?;
    if !fs::metadata(&canonical)?.is_dir() {
        return Err(io::Error::new(
            ErrorKind::NotADirectory,
            "path is not a directory",
        ));
    }
    Ok(canonical)
}

fn read_entries(directory: &Path) -> io::Result<(Vec<FileBrowserEntry>, usize)> {
    let mut entries = Vec::new();
    let mut skipped_count = 0;
    for child in fs::read_dir(directory)? {
        let child = match child {
            Ok(child) => child,
            Err(_) => {
                skipped_count += 1;
                continue;
            }
        };
        let file_type = match child.file_type() {
            Ok(file_type) => file_type,
            Err(_) => {
                skipped_count += 1;
                continue;
            }
        };
        match browser_entry(child.path(), file_type.is_dir(), file_type.is_symlink()) {
            Ok(entry) => entries.push(entry),
            Err(_) => skipped_count += 1,
        }
    }
    Ok((entries, skipped_count))
}

fn browser_entry(
    path: PathBuf,
    is_directory: bool,
    is_symlink: bool,
) -> io::Result<FileBrowserEntry> {
    let metadata = fs::symlink_metadata(&path)?;
    let modified_at_ms = metadata
        .modified()
        .ok()
        .and_then(|modified| modified.duration_since(UNIX_EPOCH).ok())
        .and_then(|duration| u64::try_from(duration.as_millis()).ok());

    Ok(FileBrowserEntry {
        name: path
            .file_name()
            .unwrap_or(path.as_os_str())
            .to_string_lossy()
            .into_owned(),
        path: current_platform().display_path(&path),
        is_directory,
        is_symlink,
        size: metadata.len(),
        modified_at_ms,
    })
}

fn sort_entries(entries: &mut [FileBrowserEntry]) {
    // Compute Unicode case-folded keys once per entry, rather than allocating
    // lowercase strings for each comparison in large directories.
    entries.sort_by_cached_key(|entry| (!entry.is_directory, entry.name.to_lowercase()));
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::{list_directory, search_directory};

    #[test]
    fn search_matches_names_recursively_and_ignores_directory_links() {
        let temporary = tempfile::tempdir().unwrap();
        let root = temporary.path().join("root");
        let nested = root.join("nested");
        fs::create_dir_all(&nested).unwrap();
        fs::write(root.join("Needle.txt"), b"one").unwrap();
        fs::write(nested.join("another-needle.log"), b"two").unwrap();

        #[cfg(unix)]
        std::os::unix::fs::symlink(&nested, root.join("linked")).unwrap();

        let result = search_directory(&root, " NEEDLE ").unwrap();
        let mut names = result
            .entries
            .iter()
            .map(|entry| entry.name.as_str())
            .collect::<Vec<_>>();
        names.sort_unstable();

        assert_eq!(names, ["Needle.txt", "another-needle.log"]);
    }

    #[test]
    fn directory_listing_places_folders_before_files() {
        let temporary = tempfile::tempdir().unwrap();
        fs::create_dir(temporary.path().join("folder")).unwrap();
        fs::write(temporary.path().join("file.txt"), b"contents").unwrap();

        let listing = list_directory(temporary.path()).unwrap();

        assert_eq!(listing.entries[0].name, "folder");
        assert!(listing.entries[0].is_directory);
        assert_eq!(listing.entries[1].name, "file.txt");
        assert_eq!(listing.entries[1].size, 8);
    }

    #[test]
    fn search_rejects_an_empty_query() {
        let temporary = tempfile::tempdir().unwrap();

        assert!(search_directory(temporary.path(), "  ").is_err());
    }
}
