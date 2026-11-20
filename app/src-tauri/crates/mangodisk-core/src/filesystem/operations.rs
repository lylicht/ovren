//! User-initiated file manager operations. These never elevate or overwrite.
use std::{
    collections::HashSet,
    fs::{self, File, OpenOptions},
    io::{self, ErrorKind, Read},
    path::{Path, PathBuf},
};

use mangodisk_platform::{current_platform, Platform};
use serde::{Deserialize, Serialize};

const MAX_BATCH: usize = 128;
const MAX_DEPTH: usize = 256;

const MAX_PREVIEW_BYTES: usize = 64 * 1024;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FileTextPreview {
    pub text: String,
    pub truncated: bool,
}

/// Only opens bounded, known-text formats; never executes or edits file content.
pub fn preview_text(path: &Path) -> io::Result<FileTextPreview> {
    let path = source(path)?;
    let metadata = fs::symlink_metadata(&path)?;
    if !metadata.is_file() || metadata.file_type().is_symlink() {
        return Err(invalid("preview requires a regular file"));
    }
    let extension = path
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("");
    if !matches!(
        extension.to_ascii_lowercase().as_str(),
        "txt"
            | "md"
            | "json"
            | "toml"
            | "yaml"
            | "yml"
            | "xml"
            | "ini"
            | "log"
            | "csv"
            | "rs"
            | "ts"
            | "tsx"
            | "js"
            | "jsx"
            | "py"
            | "go"
            | "java"
            | "kt"
            | "swift"
            | "html"
            | "css"
            | "sh"
    ) {
        return Err(invalid("file format is not eligible for text preview"));
    }
    let mut bytes = Vec::with_capacity((metadata.len() as usize).min(MAX_PREVIEW_BYTES + 1));
    File::open(&path)?
        .take((MAX_PREVIEW_BYTES + 1) as u64)
        .read_to_end(&mut bytes)?;
    let truncated = bytes.len() > MAX_PREVIEW_BYTES;
    bytes.truncate(MAX_PREVIEW_BYTES);
    if bytes.contains(&0) {
        return Err(invalid("binary file is not eligible for text preview"));
    }
    Ok(FileTextPreview {
        text: String::from_utf8_lossy(&bytes).into_owned(),
        truncated,
    })
}

#[derive(Debug, Clone, Copy, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum FileTransferMode {
    Copy,
    Move,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FileMutationFailure {
    pub path: String,
    pub reason: &'static str,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct FileMutationReport {
    pub completed: Vec<String>,
    pub failed: Vec<FileMutationFailure>,
}

fn invalid(message: &'static str) -> io::Error {
    io::Error::new(ErrorKind::InvalidInput, message)
}

fn validate_name(name: &str) -> io::Result<()> {
    if name.trim().is_empty()
        || name != name.trim()
        || name == "."
        || name == ".."
        || name
            .chars()
            .any(|c| c.is_control() || c == '/' || c == '\\' || c == ':')
        || Path::new(name)
            .file_name()
            .is_none_or(|file| file.to_string_lossy() != name)
    {
        return Err(invalid("invalid file name"));
    }
    // Windows reserves device basenames even with an extension.
    let stem = name.split('.').next().unwrap_or("").to_ascii_uppercase();
    if matches!(stem.as_str(), "CON" | "PRN" | "AUX" | "NUL")
        || (stem.len() == 4
            && (stem.starts_with("COM") || stem.starts_with("LPT"))
            && stem.as_bytes()[3].is_ascii_digit()
            && stem.as_bytes()[3] != b'0')
        || name.ends_with('.')
    {
        return Err(invalid("reserved file name"));
    }
    Ok(())
}

fn directory(path: &Path) -> io::Result<PathBuf> {
    if !path.is_absolute() {
        return Err(invalid("expected absolute directory"));
    }
    let path = fs::canonicalize(path)?;
    if !fs::metadata(&path)?.is_dir() {
        return Err(invalid("expected directory"));
    }
    Ok(path)
}

fn source(path: &Path) -> io::Result<PathBuf> {
    if !path.is_absolute() {
        return Err(invalid("expected absolute source"));
    }
    let parent = path
        .parent()
        .ok_or_else(|| invalid("cannot mutate volume root"))?;
    let parent = directory(parent)?;
    let name = path
        .file_name()
        .ok_or_else(|| invalid("cannot mutate volume root"))?;
    if name == "." || name == ".." {
        return Err(invalid("invalid source"));
    }
    let source = parent.join(name);
    fs::symlink_metadata(&source)?;
    // A mistaken selection must not move/trash the entire user home.
    for key in ["HOME", "USERPROFILE"] {
        if let Some(home) = std::env::var_os(key) {
            if let (Ok(candidate), Ok(home)) = (fs::canonicalize(&source), fs::canonicalize(home)) {
                if candidate == home {
                    return Err(invalid("cannot mutate home directory"));
                }
            }
        }
    }
    Ok(source)
}

fn no_links(path: &Path, depth: usize) -> io::Result<()> {
    if depth > MAX_DEPTH {
        return Err(invalid("directory nesting limit exceeded"));
    }
    let metadata = fs::symlink_metadata(path)?;
    if metadata.file_type().is_symlink() {
        return Err(invalid("links are not copied or moved"));
    }
    if metadata.is_dir() {
        for child in fs::read_dir(path)? {
            no_links(&child?.path(), depth + 1)?;
        }
    } else if !metadata.is_file() {
        return Err(invalid("unsupported filesystem item"));
    }
    Ok(())
}

fn copy_item(from: &Path, to: &Path, depth: usize) -> io::Result<()> {
    if depth > MAX_DEPTH {
        return Err(invalid("directory nesting limit exceeded"));
    }
    let metadata = fs::symlink_metadata(from)?;
    if metadata.file_type().is_symlink() {
        return Err(invalid("links are not copied"));
    }
    if metadata.is_dir() {
        fs::create_dir(to)?;
        let result = (|| {
            for child in fs::read_dir(from)? {
                let child = child?;
                copy_item(&child.path(), &to.join(child.file_name()), depth + 1)?;
            }
            fs::set_permissions(to, metadata.permissions())?;
            Ok(())
        })();
        if result.is_err() {
            let _ = fs::remove_dir_all(to);
        }
        result
    } else if metadata.is_file() {
        let mut input = File::open(from)?;
        let mut output = OpenOptions::new().write(true).create_new(true).open(to)?;
        let result = (|| {
            io::copy(&mut input, &mut output)?;
            output.sync_all()?;
            fs::set_permissions(to, metadata.permissions())?;
            Ok(())
        })();
        if result.is_err() {
            drop(output);
            let _ = fs::remove_file(to);
        }
        result
    } else {
        Err(invalid("unsupported filesystem item"))
    }
}

fn reason(error: &io::Error) -> &'static str {
    match error.kind() {
        ErrorKind::AlreadyExists => "alreadyExists",
        ErrorKind::NotFound => "notFound",
        ErrorKind::PermissionDenied => "permissionDenied",
        ErrorKind::InvalidInput => "invalidInput",
        _ => "failed",
    }
}

fn selected(paths: &[String]) -> io::Result<()> {
    if paths.is_empty() || paths.len() > MAX_BATCH {
        return Err(invalid("invalid file selection size"));
    }
    let mut unique = HashSet::new();
    for path in paths {
        if !unique.insert(path) {
            return Err(invalid("duplicate file selection"));
        }
    }
    Ok(())
}

pub fn create_entry(parent: &Path, name: &str, folder: bool) -> io::Result<String> {
    validate_name(name)?;
    let parent = directory(parent)?;
    let target = parent.join(name);
    if folder {
        fs::create_dir(&target)?;
    } else {
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&target)?;
    }
    Ok(current_platform().display_path(&target))
}

pub fn rename_entry(path: &Path, name: &str) -> io::Result<String> {
    validate_name(name)?;
    let from = source(path)?;
    let to = from
        .parent()
        .ok_or_else(|| invalid("invalid source"))?
        .join(name);
    if from == to {
        return Err(invalid("source and target are identical"));
    }
    if fs::symlink_metadata(&to).is_ok() {
        return Err(io::Error::from(ErrorKind::AlreadyExists));
    }
    rename_no_replace(&from, &to)?;
    Ok(current_platform().display_path(&to))
}

#[cfg(unix)]
fn c_path(path: &Path) -> io::Result<std::ffi::CString> {
    use std::os::unix::ffi::OsStrExt;
    std::ffi::CString::new(path.as_os_str().as_bytes()).map_err(|_| invalid("invalid path"))
}

/// Refuse overwrites atomically, including if another process creates the
/// destination between validation and rename. Unsupported filesystems fail closed.
#[cfg(target_os = "linux")]
fn rename_no_replace(from: &Path, to: &Path) -> io::Result<()> {
    let from = c_path(from)?;
    let to = c_path(to)?;
    // SAFETY: both CStrings remain alive for the duration of renameat2.
    let result = unsafe {
        libc::syscall(
            libc::SYS_renameat2,
            libc::AT_FDCWD,
            from.as_ptr(),
            libc::AT_FDCWD,
            to.as_ptr(),
            libc::RENAME_NOREPLACE,
        )
    };
    if result == -1 {
        Err(io::Error::last_os_error())
    } else {
        Ok(())
    }
}

#[cfg(target_os = "macos")]
fn rename_no_replace(from: &Path, to: &Path) -> io::Result<()> {
    let from = c_path(from)?;
    let to = c_path(to)?;
    // SAFETY: valid NUL-terminated path buffers are borrowed for the call only.
    let result = unsafe { libc::renamex_np(from.as_ptr(), to.as_ptr(), libc::RENAME_EXCL) };
    if result == -1 {
        Err(io::Error::last_os_error())
    } else {
        Ok(())
    }
}

#[cfg(windows)]
fn rename_no_replace(from: &Path, to: &Path) -> io::Result<()> {
    use std::os::windows::ffi::OsStrExt;
    use windows_sys::Win32::Storage::FileSystem::MoveFileExW;

    let from = from
        .as_os_str()
        .encode_wide()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();
    let to = to
        .as_os_str()
        .encode_wide()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();
    // SAFETY: both buffers are valid NUL-terminated UTF-16; 0 deliberately
    // excludes MOVEFILE_REPLACE_EXISTING and MOVEFILE_COPY_ALLOWED.
    let result = unsafe { MoveFileExW(from.as_ptr(), to.as_ptr(), 0) };
    if result == 0 {
        Err(io::Error::last_os_error())
    } else {
        Ok(())
    }
}

#[cfg(not(any(target_os = "linux", target_os = "macos", windows)))]
fn rename_no_replace(_from: &Path, _to: &Path) -> io::Result<()> {
    Err(io::Error::from(ErrorKind::Unsupported))
}

fn transfer_one(from: &Path, destination: &Path, mode: FileTransferMode) -> io::Result<String> {
    let from = source(from)?;
    // Renaming a directory is metadata-only; avoid a second full tree walk.
    // Copying still checks nested links before any destination is created.
    if matches!(mode, FileTransferMode::Copy) {
        no_links(&from, 0)?;
    } else if fs::symlink_metadata(&from)?.file_type().is_symlink() {
        return Err(invalid("links cannot be moved"));
    }
    let target = destination.join(
        from.file_name()
            .ok_or_else(|| invalid("missing source name"))?,
    );
    if from == target || fs::symlink_metadata(&target).is_ok() {
        return Err(io::Error::from(ErrorKind::AlreadyExists));
    }
    if fs::symlink_metadata(&from)?.is_dir() && destination.starts_with(&from) {
        return Err(invalid("cannot place a directory inside itself"));
    }
    match mode {
        FileTransferMode::Copy => {
            // Copy into a private sibling staging area. A crash, I/O failure,
            // or a concurrent destination creation cannot expose half-written
            // files or folders at the user-visible target.
            let stage = tempfile::Builder::new()
                .prefix(".ovren-copy-")
                .tempdir_in(destination)?;
            let staged = stage.path().join("entry");
            copy_item(&from, &staged, 0)?;
            rename_no_replace(&staged, &target)?;
            // The TempDir removes only its now-empty private staging directory.
        }
        // Cross-volume moves fail safely; no unverified copy-and-delete fallback.
        FileTransferMode::Move => rename_no_replace(&from, &target)?,
    }
    Ok(current_platform().display_path(&target))
}

pub fn transfer_entries(
    paths: Vec<String>,
    destination: &Path,
    mode: FileTransferMode,
) -> io::Result<FileMutationReport> {
    selected(&paths)?;
    let destination = directory(destination)?;
    let mut report = FileMutationReport {
        completed: Vec::new(),
        failed: Vec::new(),
    };
    for original in paths {
        match transfer_one(Path::new(&original), &destination, mode) {
            Ok(target) => report.completed.push(target),
            Err(error) => {
                log::warn!(
                    "file_browser_transfer_failed source={} destination={} error={error}",
                    original,
                    destination.display()
                );
                report.failed.push(FileMutationFailure {
                    path: original,
                    reason: reason(&error),
                });
            }
        }
    }
    Ok(report)
}

pub fn trash_entries(paths: Vec<String>) -> io::Result<FileMutationReport> {
    selected(&paths)?;
    let mut report = FileMutationReport {
        completed: Vec::new(),
        failed: Vec::new(),
    };
    for original in paths {
        match source(Path::new(&original)) {
            Ok(path) => match trash::delete(&path) {
                Ok(()) => report.completed.push(original),
                Err(error) => {
                    log::warn!(
                        "file_browser_trash_failed path={} error={error}",
                        path.display()
                    );
                    report.failed.push(FileMutationFailure {
                        path: original,
                        reason: "failed",
                    });
                }
            },
            Err(error) => report.failed.push(FileMutationFailure {
                path: original,
                reason: reason(&error),
            }),
        }
    }
    Ok(report)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rename_refuses_existing_target_without_changing_contents() {
        let temp = tempfile::tempdir().unwrap();
        let from = temp.path().join("from.txt");
        let to = temp.path().join("to.txt");
        fs::write(&from, "source").unwrap();
        fs::write(&to, "destination").unwrap();
        assert!(rename_entry(&from, "to.txt").is_err());
        assert_eq!(fs::read_to_string(&from).unwrap(), "source");
        assert_eq!(fs::read_to_string(&to).unwrap(), "destination");
    }

    #[test]
    fn preview_is_bounded_and_rejects_binary_data() {
        let temp = tempfile::tempdir().unwrap();
        let file = temp.path().join("notes.txt");
        fs::write(&file, vec![b'a'; MAX_PREVIEW_BYTES + 20]).unwrap();
        let preview = preview_text(&file).unwrap();
        assert!(preview.truncated);
        assert_eq!(preview.text.len(), MAX_PREVIEW_BYTES);
        fs::write(&file, b"\0binary").unwrap();
        assert!(preview_text(&file).is_err());
        fs::write(temp.path().join("image.png"), b"not text").unwrap();
        assert!(preview_text(&temp.path().join("image.png")).is_err());
    }

    #[test]
    fn creates_renames_and_prevents_overwriting() {
        let temp = tempfile::tempdir().unwrap();
        let file = create_entry(temp.path(), "notes.txt", false).unwrap();
        assert!(Path::new(&file).is_file());
        assert!(create_entry(temp.path(), "notes.txt", false).is_err());
        let renamed = rename_entry(Path::new(&file), "renamed.txt").unwrap();
        assert!(Path::new(&renamed).is_file());
        assert!(!Path::new(&file).exists());
        assert!(rename_entry(Path::new(&renamed), "../escape").is_err());
    }

    #[test]
    fn copies_nested_directories_without_overwriting() {
        let temp = tempfile::tempdir().unwrap();
        let source = temp.path().join("source");
        let target = temp.path().join("target");
        fs::create_dir_all(source.join("deep")).unwrap();
        fs::create_dir(&target).unwrap();
        fs::write(source.join("deep/file.txt"), "original").unwrap();
        let report = transfer_entries(
            vec![source.to_string_lossy().into_owned()],
            &target,
            FileTransferMode::Copy,
        )
        .unwrap();
        assert!(report.failed.is_empty());
        assert_eq!(
            fs::read(target.join("source/deep/file.txt")).unwrap(),
            b"original"
        );
        let report = transfer_entries(
            vec![source.to_string_lossy().into_owned()],
            &target,
            FileTransferMode::Copy,
        )
        .unwrap();
        assert_eq!(report.failed[0].reason, "alreadyExists");
    }

    #[test]
    fn successful_copy_publishes_only_final_name_without_staging_residue() {
        let temp = tempfile::tempdir().unwrap();
        let source = temp.path().join("sample.txt");
        let destination = temp.path().join("destination");
        fs::write(&source, b"complete").unwrap();
        fs::create_dir(&destination).unwrap();

        let report = transfer_entries(
            vec![source.to_string_lossy().into_owned()],
            &destination,
            FileTransferMode::Copy,
        )
        .unwrap();

        assert!(report.failed.is_empty());
        assert_eq!(
            fs::read(destination.join("sample.txt")).unwrap(),
            b"complete"
        );
        let names = fs::read_dir(&destination)
            .unwrap()
            .map(|item| item.unwrap().file_name())
            .collect::<Vec<_>>();
        assert_eq!(names, [std::ffi::OsString::from("sample.txt")]);
        assert_eq!(fs::read(&source).unwrap(), b"complete");
    }

    #[test]
    fn moving_into_descendant_and_parent_name_traversal_are_denied() {
        let temp = tempfile::tempdir().unwrap();
        let parent = temp.path().join("a");
        let child = parent.join("b");
        fs::create_dir_all(&child).unwrap();
        let report = transfer_entries(
            vec![parent.to_string_lossy().into_owned()],
            &child,
            FileTransferMode::Move,
        )
        .unwrap();
        assert_eq!(report.failed[0].reason, "invalidInput");
        assert!(create_entry(&parent, "..", true).is_err());
    }

    #[cfg(unix)]
    #[test]
    fn copy_rejects_symlinks_instead_of_following_them() {
        use std::os::unix::fs::symlink;
        let temp = tempfile::tempdir().unwrap();
        let src = temp.path().join("src");
        let dest = temp.path().join("dest");
        fs::create_dir(&src).unwrap();
        fs::create_dir(&dest).unwrap();
        symlink(temp.path(), src.join("loop")).unwrap();
        let result = transfer_entries(
            vec![src.to_string_lossy().into_owned()],
            &dest,
            FileTransferMode::Copy,
        )
        .unwrap();
        assert_eq!(result.failed[0].reason, "invalidInput");
        assert!(!dest.join("src").exists());
    }
}
