//! File system utilities for runtime execution
//!
//! This module provides file system operations needed for workflow execution,
//! including directory management, file staging, and path utilities.

use crate::runtime::error::{IntoRuntimeError, RuntimeResult};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::SystemTime;

/// Create a directory and all parent directories if they don't exist
pub fn create_dir_all<P: AsRef<Path>>(path: P) -> RuntimeResult<()> {
    let path = path.as_ref();
    fs::create_dir_all(path)
        .runtime_context_with_path("Failed to create directory", &path.display().to_string())
}

/// Remove a directory and all its contents
pub fn remove_dir_all<P: AsRef<Path>>(path: P) -> RuntimeResult<()> {
    let path = path.as_ref();
    if path.exists() {
        fs::remove_dir_all(path)
            .runtime_context_with_path("Failed to remove directory", &path.display().to_string())
    } else {
        Ok(())
    }
}

/// Copy a file from source to destination
pub fn copy_file<P: AsRef<Path>, Q: AsRef<Path>>(from: P, to: Q) -> RuntimeResult<u64> {
    let from = from.as_ref();
    let to = to.as_ref();

    // Ensure destination directory exists
    if let Some(parent) = to.parent() {
        create_dir_all(parent)?;
    }

    fs::copy(from, to).runtime_context(&format!(
        "Failed to copy file from {} to {}",
        from.display(),
        to.display()
    ))
}

/// Create a symbolic link
pub fn symlink<P: AsRef<Path>, Q: AsRef<Path>>(original: P, link: Q) -> RuntimeResult<()> {
    let original = original.as_ref();
    let link = link.as_ref();

    // Ensure destination directory exists
    if let Some(parent) = link.parent() {
        create_dir_all(parent)?;
    }

    #[cfg(unix)]
    {
        std::os::unix::fs::symlink(original, link).runtime_context(&format!(
            "Failed to create symlink from {} to {}",
            original.display(),
            link.display()
        ))
    }

    #[cfg(windows)]
    {
        if original.is_dir() {
            std::os::windows::fs::symlink_dir(original, link)
        } else {
            std::os::windows::fs::symlink_file(original, link)
        }
        .runtime_context(&format!(
            "Failed to create symlink from {} to {}",
            original.display(),
            link.display()
        ))
    }
}

/// Write content to a file atomically
pub fn write_file_atomic<P: AsRef<Path>, C: AsRef<[u8]>>(
    path: P,
    contents: C,
) -> RuntimeResult<()> {
    let path = path.as_ref();
    let contents = contents.as_ref();

    // Ensure parent directory exists
    if let Some(parent) = path.parent() {
        create_dir_all(parent)?;
    }

    // Write to temporary file first
    let temp_path = path.with_extension("tmp");
    fs::write(&temp_path, contents).runtime_context_with_path(
        "Failed to write temporary file",
        &temp_path.display().to_string(),
    )?;

    // Atomically move to final location
    fs::rename(&temp_path, path).runtime_context(&format!(
        "Failed to move temporary file {} to {}",
        temp_path.display(),
        path.display()
    ))
}

/// Read file contents as string
pub fn read_file_to_string<P: AsRef<Path>>(path: P) -> RuntimeResult<String> {
    let path = path.as_ref();
    fs::read_to_string(path)
        .runtime_context_with_path("Failed to read file", &path.display().to_string())
}

/// Read file contents as bytes
pub fn read_file_to_bytes<P: AsRef<Path>>(path: P) -> RuntimeResult<Vec<u8>> {
    let path = path.as_ref();
    fs::read(path).runtime_context_with_path("Failed to read file", &path.display().to_string())
}

/// Get file size in bytes
pub fn file_size<P: AsRef<Path>>(path: P) -> RuntimeResult<u64> {
    let path = path.as_ref();
    let metadata = fs::metadata(path)
        .runtime_context_with_path("Failed to get file metadata", &path.display().to_string())?;
    Ok(metadata.len())
}

/// Check if a path is within another path (prevents directory traversal)
pub fn path_is_within<P: AsRef<Path>, Q: AsRef<Path>>(path: P, base: Q) -> RuntimeResult<bool> {
    let path_ref = path.as_ref();
    let base_ref = base.as_ref();
    let path = path_ref.canonicalize().runtime_context_with_path(
        "Failed to canonicalize path",
        &path_ref.display().to_string(),
    )?;
    let base = base_ref.canonicalize().runtime_context_with_path(
        "Failed to canonicalize base path",
        &base_ref.display().to_string(),
    )?;

    Ok(path.starts_with(base))
}

/// Get the absolute path
pub fn absolute_path<P: AsRef<Path>>(path: P) -> RuntimeResult<PathBuf> {
    let path = path.as_ref();
    path.canonicalize()
        .runtime_context_with_path("Failed to get absolute path", &path.display().to_string())
}

/// Create a unique temporary directory
pub fn create_temp_dir(prefix: &str) -> RuntimeResult<PathBuf> {
    let timestamp = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap()
        .as_secs();
    let pid = std::process::id();
    let dir_name = format!("{}_{}_{}_{}", prefix, timestamp, pid, rand_string(8));

    let temp_dir = std::env::temp_dir().join(dir_name);
    create_dir_all(&temp_dir)?;
    Ok(temp_dir)
}

/// Generate a random string of given length
fn rand_string(len: usize) -> String {
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};

    let mut hasher = DefaultHasher::new();
    SystemTime::now().hash(&mut hasher);
    std::process::id().hash(&mut hasher);
    std::thread::current().id().hash(&mut hasher);

    let hash = hasher.finish();
    format!("{:016x}", hash)[..len.min(16)].to_string()
}

/// Set file permissions (Unix only)
#[cfg(unix)]
pub fn set_permissions<P: AsRef<Path>>(path: P, mode: u32) -> RuntimeResult<()> {
    use std::os::unix::fs::PermissionsExt;
    let path = path.as_ref();
    let permissions = fs::Permissions::from_mode(mode);
    fs::set_permissions(path, permissions)
        .runtime_context_with_path("Failed to set permissions", &path.display().to_string())
}

/// Set file permissions (Windows - no-op)
#[cfg(windows)]
pub fn set_permissions<P: AsRef<Path>>(_path: P, _mode: u32) -> RuntimeResult<()> {
    // Windows doesn't have Unix-style permissions
    Ok(())
}

/// Make file executable (Unix only)
#[cfg(unix)]
pub fn make_executable<P: AsRef<Path>>(path: P) -> RuntimeResult<()> {
    set_permissions(path, 0o755)
}

/// Make file executable (Windows - no-op)
#[cfg(windows)]
pub fn make_executable<P: AsRef<Path>>(_path: P) -> RuntimeResult<()> {
    Ok(())
}

/// Directory management for workflow execution
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkflowDirectory {
    /// Root directory for this workflow run
    pub root: PathBuf,
    /// Working directory for task execution
    pub work: PathBuf,
    /// Directory for input files
    pub inputs: PathBuf,
    /// Directory for output files
    pub outputs: PathBuf,
    /// Directory for temporary files
    pub temp: PathBuf,
}

impl WorkflowDirectory {
    /// Create a new workflow directory structure
    pub fn create<P: AsRef<Path>>(base_dir: P, run_id: &str) -> RuntimeResult<Self> {
        let root = base_dir.as_ref().join(run_id);
        let work = root.join("work");
        let inputs = root.join("inputs");
        let outputs = root.join("outputs");
        let temp = root.join("temp");

        // Create all directories
        create_dir_all(&root)?;
        create_dir_all(&work)?;
        create_dir_all(&inputs)?;
        create_dir_all(&outputs)?;
        create_dir_all(&temp)?;

        Ok(Self {
            root,
            work,
            inputs,
            outputs,
            temp,
        })
    }

    /// Clean up all files in the workflow directory
    pub fn cleanup(&self) -> RuntimeResult<()> {
        remove_dir_all(&self.root)
    }

    /// Get a subdirectory path within the workflow
    pub fn subdir<P: AsRef<Path>>(&self, path: P) -> PathBuf {
        self.root.join(path)
    }

    /// Stage an input file (copy or symlink)
    pub fn stage_input<P: AsRef<Path>>(
        &self,
        source: P,
        name: &str,
        copy: bool,
    ) -> RuntimeResult<PathBuf> {
        let source = source.as_ref();
        let dest = self.inputs.join(name);

        if copy {
            copy_file(source, &dest)?;
        } else {
            symlink(source, &dest)?;
        }

        Ok(dest)
    }

    /// Collect an output file
    pub fn collect_output<P: AsRef<Path>>(&self, source: P, name: &str) -> RuntimeResult<PathBuf> {
        let source = source.as_ref();
        let dest = self.outputs.join(name);

        copy_file(source, &dest)?;
        Ok(dest)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::tempdir;

    #[test]
    fn test_create_and_remove_dir() {
        let temp_dir = tempdir().unwrap();
        let test_dir = temp_dir.path().join("test_dir");

        // Create directory
        create_dir_all(&test_dir).unwrap();
        assert!(test_dir.exists());

        // Remove directory
        remove_dir_all(&test_dir).unwrap();
        assert!(!test_dir.exists());
    }

    #[test]
    fn test_copy_file() {
        let temp_dir = tempdir().unwrap();
        let source = temp_dir.path().join("source.txt");
        let dest = temp_dir.path().join("dest.txt");

        // Create source file
        fs::write(&source, "test content").unwrap();

        // Copy file
        let bytes_copied = copy_file(&source, &dest).unwrap();
        assert_eq!(bytes_copied, 12);
        assert_eq!(read_file_to_string(&dest).unwrap(), "test content");
    }

    #[test]
    fn test_atomic_write() {
        let temp_dir = tempdir().unwrap();
        let file_path = temp_dir.path().join("test.txt");

        write_file_atomic(&file_path, "test content").unwrap();
        assert_eq!(read_file_to_string(&file_path).unwrap(), "test content");
    }

    #[test]
    fn test_path_within() {
        let temp_dir = tempdir().unwrap();
        let base = temp_dir.path();
        let inside = base.join("inside");
        let outside = temp_dir.path().parent().unwrap().join("outside");

        fs::create_dir_all(&inside).unwrap();
        fs::create_dir_all(&outside).unwrap();

        assert!(path_is_within(&inside, &base).unwrap());
        assert!(!path_is_within(&outside, &base).unwrap());
    }

    #[test]
    fn test_workflow_directory() {
        let temp_dir = tempdir().unwrap();
        let workflow_dir = WorkflowDirectory::create(temp_dir.path(), "test_run").unwrap();

        // Check all directories were created
        assert!(workflow_dir.root.exists());
        assert!(workflow_dir.work.exists());
        assert!(workflow_dir.inputs.exists());
        assert!(workflow_dir.outputs.exists());
        assert!(workflow_dir.temp.exists());

        // Test input staging
        let source_file = temp_dir.path().join("source.txt");
        fs::write(&source_file, "input content").unwrap();

        let staged = workflow_dir
            .stage_input(&source_file, "input.txt", true)
            .unwrap();
        assert!(staged.exists());
        assert_eq!(read_file_to_string(&staged).unwrap(), "input content");

        // Cleanup
        workflow_dir.cleanup().unwrap();
        assert!(!workflow_dir.root.exists());
    }

    #[test]
    fn test_temp_dir_creation() {
        let temp_dir = create_temp_dir("test_prefix").unwrap();
        assert!(temp_dir.exists());
        assert!(temp_dir
            .file_name()
            .unwrap()
            .to_str()
            .unwrap()
            .starts_with("test_prefix"));

        // Cleanup
        remove_dir_all(temp_dir).unwrap();
    }

    #[test]
    fn test_file_size() {
        let temp_dir = tempdir().unwrap();
        let file_path = temp_dir.path().join("test.txt");

        fs::write(&file_path, "hello").unwrap();
        assert_eq!(file_size(&file_path).unwrap(), 5);
    }
}
