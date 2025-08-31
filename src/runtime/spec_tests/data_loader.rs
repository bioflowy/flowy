use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};

/// Handles loading and managing test data files for specification tests
pub struct TestDataLoader {
    /// Base directory containing test data files
    data_dir: PathBuf,
    /// Cache of loaded file contents
    file_cache: HashMap<String, String>,
}

impl TestDataLoader {
    /// Create a new test data loader
    pub fn new<P: AsRef<Path>>(data_dir: P) -> Self {
        Self {
            data_dir: data_dir.as_ref().to_path_buf(),
            file_cache: HashMap::new(),
        }
    }

    /// Load a test data file by name
    pub fn load_file(&mut self, filename: &str) -> Result<String, Box<dyn std::error::Error>> {
        // Check cache first
        if let Some(cached_content) = self.file_cache.get(filename) {
            return Ok(cached_content.clone());
        }

        // Try to find the file in the data directory
        let file_path = self.find_data_file(filename)?;
        let content = fs::read_to_string(&file_path)?;

        // Cache the content
        self.file_cache
            .insert(filename.to_string(), content.clone());

        Ok(content)
    }

    /// Find a data file in the test data directory (with recursive search)
    fn find_data_file(&self, filename: &str) -> Result<PathBuf, Box<dyn std::error::Error>> {
        // First try direct path
        let direct_path = self.data_dir.join(filename);
        if direct_path.exists() {
            return Ok(direct_path);
        }

        // Try recursive search
        if let Some(found_path) = self.search_recursive(&self.data_dir, filename)? {
            return Ok(found_path);
        }

        Err(format!(
            "Test data file '{}' not found in {}",
            filename,
            self.data_dir.display()
        )
        .into())
    }

    /// Recursively search for a file in a directory
    fn search_recursive(
        &self,
        dir: &Path,
        filename: &str,
    ) -> Result<Option<PathBuf>, Box<dyn std::error::Error>> {
        if !dir.is_dir() {
            return Ok(None);
        }

        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();

            if path.is_file() && path.file_name().unwrap_or_default() == filename {
                return Ok(Some(path));
            } else if path.is_dir() {
                if let Some(found) = self.search_recursive(&path, filename)? {
                    return Ok(Some(found));
                }
            }
        }

        Ok(None)
    }

    /// Get the absolute path for a test data file
    pub fn get_file_path(&mut self, filename: &str) -> Result<PathBuf, Box<dyn std::error::Error>> {
        self.find_data_file(filename)
    }

    /// List all available test data files
    pub fn list_data_files(&self) -> Result<Vec<String>, Box<dyn std::error::Error>> {
        let mut files = Vec::new();
        self.collect_files_recursive(&self.data_dir, &mut files, "")?;
        Ok(files)
    }

    /// Recursively collect all files in a directory
    fn collect_files_recursive(
        &self,
        dir: &Path,
        files: &mut Vec<String>,
        prefix: &str,
    ) -> Result<(), Box<dyn std::error::Error>> {
        if !dir.is_dir() {
            return Ok(());
        }

        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            let path = entry.path();

            if path.is_file() {
                if let Some(filename) = path.file_name().and_then(|n| n.to_str()) {
                    let full_name = if prefix.is_empty() {
                        filename.to_string()
                    } else {
                        format!("{}/{}", prefix, filename)
                    };
                    files.push(full_name);
                }
            } else if path.is_dir() {
                if let Some(dirname) = path.file_name().and_then(|n| n.to_str()) {
                    let new_prefix = if prefix.is_empty() {
                        dirname.to_string()
                    } else {
                        format!("{}/{}", prefix, dirname)
                    };
                    self.collect_files_recursive(&path, files, &new_prefix)?;
                }
            }
        }

        Ok(())
    }

    /// Create a temporary work directory for test execution
    pub fn create_temp_dir(&self) -> Result<PathBuf, Box<dyn std::error::Error>> {
        let temp_dir = std::env::temp_dir().join(format!("wdl_spec_test_{}", std::process::id()));
        fs::create_dir_all(&temp_dir)?;
        Ok(temp_dir)
    }

    /// Copy a data file to a temporary directory
    pub fn copy_to_temp(
        &mut self,
        filename: &str,
        temp_dir: &Path,
    ) -> Result<PathBuf, Box<dyn std::error::Error>> {
        let source_path = self.find_data_file(filename)?;
        let dest_path = temp_dir.join(filename);

        // Ensure destination directory exists
        if let Some(parent) = dest_path.parent() {
            fs::create_dir_all(parent)?;
        }

        fs::copy(&source_path, &dest_path)?;
        Ok(dest_path)
    }

    /// Resolve file references in JSON input, converting relative paths to absolute paths
    pub fn resolve_file_references(
        &mut self,
        json_input: &str,
    ) -> Result<String, Box<dyn std::error::Error>> {
        // Simple implementation - look for common file patterns and resolve them
        let resolved = json_input.to_string();

        // Match patterns like "filename.txt" or "./filename.txt"
        let file_patterns = [
            r#""([^"]+\.txt)""#,
            r#""([^"]+\.json)""#,
            r#""([^"]+\.csv)""#,
            r#""([^"]+\.tsv)""#,
            r#""([^"]+\.wdl)""#,
        ];

        let mut final_resolved = resolved;

        for pattern in &file_patterns {
            let regex = ::regex::Regex::new(pattern).unwrap();
            let mut replacements = Vec::new();

            let temp_resolved = final_resolved.clone();
            for captures in regex.captures_iter(&temp_resolved) {
                if let Some(filename_match) = captures.get(1) {
                    let filename = filename_match.as_str();
                    // Skip if it's already an absolute path
                    if !filename.starts_with('/') && !filename.starts_with("http") {
                        if let Ok(abs_path) = self.get_file_path(filename) {
                            replacements.push((
                                filename_match.as_str(),
                                abs_path.to_string_lossy().to_string(),
                            ));
                        }
                    }
                }
            }

            for (old, new) in replacements {
                final_resolved =
                    final_resolved.replace(&format!(r#""{}"#, old), &format!(r#""{}""#, new));
            }
        }

        Ok(final_resolved)
    }

    /// Clean up temporary directories and cached files
    pub fn cleanup(&mut self) {
        self.file_cache.clear();
        // Note: In a more complete implementation, we would track and clean up temp directories
    }
}

/// Helper struct for managing test file resources during execution
pub struct TestFileManager {
    pub loader: TestDataLoader,
    temp_dir: Option<PathBuf>,
}

impl TestFileManager {
    /// Create a new file manager with the given data directory
    pub fn new<P: AsRef<Path>>(data_dir: P) -> Self {
        Self {
            loader: TestDataLoader::new(data_dir),
            temp_dir: None,
        }
    }

    /// Prepare files for test execution by creating temp directory and copying needed files
    pub fn prepare_test_files(
        &mut self,
        json_input: Option<&str>,
    ) -> Result<Option<PathBuf>, Box<dyn std::error::Error>> {
        if json_input.is_none() {
            return Ok(None);
        }

        let temp_dir = self.loader.create_temp_dir()?;
        self.temp_dir = Some(temp_dir.clone());

        // Parse JSON input to find file references and copy them to temp dir
        if let Some(input) = json_input {
            self.copy_referenced_files(input, &temp_dir)?;
        }

        Ok(Some(temp_dir))
    }

    /// Copy files referenced in JSON input to the temporary directory
    fn copy_referenced_files(
        &mut self,
        json_input: &str,
        temp_dir: &Path,
    ) -> Result<(), Box<dyn std::error::Error>> {
        // Simple extraction of potential filenames from JSON
        let potential_files: Vec<&str> = json_input
            .split('"')
            .filter(|s| s.contains('.') && !s.contains('/') && s.len() < 100)
            .collect();

        for filename in potential_files {
            // Try to copy the file if it exists in our data directory
            if self.loader.copy_to_temp(filename, temp_dir).is_ok() {
                println!("Copied test data file: {}", filename);
            }
        }

        Ok(())
    }

    /// Get the path to the temporary directory (if created)
    pub fn temp_dir(&self) -> Option<&Path> {
        self.temp_dir.as_deref()
    }

    /// Clean up temporary resources
    pub fn cleanup(&mut self) {
        if let Some(ref temp_dir) = self.temp_dir {
            if let Err(e) = fs::remove_dir_all(temp_dir) {
                eprintln!(
                    "Warning: Failed to clean up temp dir {}: {}",
                    temp_dir.display(),
                    e
                );
            }
        }
        self.temp_dir = None;
        self.loader.cleanup();
    }
}

impl Drop for TestFileManager {
    fn drop(&mut self) {
        self.cleanup();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    #[test]
    fn test_data_loader_creation() {
        let temp_dir = std::env::temp_dir().join("test_data_loader");
        let loader = TestDataLoader::new(&temp_dir);
        assert_eq!(loader.data_dir, temp_dir);
    }

    #[test]
    fn test_create_temp_dir() {
        let loader = TestDataLoader::new("/tmp");
        let temp_dir = loader.create_temp_dir().unwrap();
        assert!(temp_dir.exists());

        // Clean up
        let _ = fs::remove_dir_all(&temp_dir);
    }

    #[test]
    fn test_file_manager_creation() {
        let manager = TestFileManager::new("/tmp");
        assert!(manager.temp_dir.is_none());
    }
}
