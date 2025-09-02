//! # miniwdl-rust
//!
//! Rust port of miniwdl - Workflow Description Language (WDL) parser and runtime.
//!
//! This crate provides parsing, static analysis, and runtime capabilities for WDL workflows.

// Allow large error enum variants for now - this is a known tradeoff for comprehensive error handling
#![allow(clippy::result_large_err)]
// Temporarily allow these clippy warnings while focusing on functionality
#![allow(clippy::collapsible_match)]
#![allow(clippy::needless_borrows_for_generic_args)]
#![allow(clippy::useless_vec)]
#![allow(unused_imports)]
#![allow(clippy::while_let_loop)]
#![allow(clippy::type_complexity)]
#![allow(clippy::should_implement_trait)]
#![allow(clippy::too_many_arguments)]
#![allow(clippy::large_enum_variant)]
#![allow(clippy::unnecessary_map_or)]
#![allow(clippy::ptr_arg)]
#![allow(clippy::only_used_in_recursion)]
#![allow(unused_variables)]
#![allow(clippy::missing_transmute_annotations)]

pub mod env;
pub mod error;
pub mod expr;
pub mod parser;
pub mod runtime;
pub mod stdlib;
pub mod tree;
pub mod types;
pub mod value;

pub use env::{Binding, Bindings};
pub use error::{SourcePosition, WdlError};
pub use expr::{BinaryOperator, Expression, ExpressionBase, StringPart, UnaryOperator};
pub use runtime::{Config, RuntimeBuilder, TaskResult, WorkflowResult};
pub use tree::{
    ASTNode, Call, Conditional, Declaration, Document, ImportDoc, Scatter, Task, Workflow, WorkflowNode,
};
pub use types::Type;
pub use value::{Value, ValueBase};

// Document loading functions
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};

/// Result type for file reading operations
pub struct ReadSourceResult {
    pub source_text: String,
    pub abspath: String,
}

/// Load a WDL document with import resolution
/// 
/// This is equivalent to the Python miniwdl `load` function.
/// It parses the main document, recursively loads imported documents,
/// and performs type checking.
pub fn load(
    uri: &str,
    path: Option<&[&str]>,
    check_quant: bool,
    import_max_depth: usize,
) -> Result<Document, WdlError> {
    let path_vec = path.map(|p| p.iter().map(|s| s.to_string()).collect()).unwrap_or_default();
    futures::executor::block_on(load_async(uri.to_string(), path_vec, check_quant, import_max_depth))
}

/// Async version of load function
pub async fn load_async(
    uri: String,
    path: Vec<String>,
    check_quant: bool,
    import_max_depth: usize,
) -> Result<Document, WdlError> {
    _load_async(uri, path, check_quant, import_max_depth, None).await
}

/// Internal async loading function with importer context
fn _load_async(
    uri: String,
    path: Vec<String>, 
    check_quant: bool,
    import_max_depth: usize,
    importer: Option<&Document>,
) -> std::pin::Pin<Box<dyn std::future::Future<Output = Result<Document, WdlError>> + Send + '_>> {
    Box::pin(async move {
    // Resolve file path
    let abspath = resolve_file_import(&uri, &path, importer).await?;
    
    // Read source file
    let read_result = read_source_default(&uri, &path, importer).await?;
    
    // Parse document
    let mut doc = parser::parse_document_with_filename(&read_result.source_text, "1.2", &abspath)?;
    
    // Recursively load imported documents
    let import_count = doc.imports.len();
    for i in 0..import_count {
        if import_max_depth <= 1 {
            return Err(WdlError::import_error(
                doc.imports[i].pos.clone(),
                doc.imports[i].uri.clone(),
                Some("exceeded import_max_depth; circular imports?".to_string()),
            ));
        }
        
        let import_uri = doc.imports[i].uri.clone();
        let import_pos = doc.imports[i].pos.clone();
        
        match _load_async(
            import_uri.clone(),
            path.clone(),
            check_quant,
            import_max_depth - 1,
            None, // Pass None to avoid borrowing issues for now
        ).await {
            Ok(subdoc) => {
                doc.imports[i].doc = Some(Box::new(subdoc));
            }
            Err(e) => {
                return Err(WdlError::import_error(
                    import_pos,
                    import_uri,
                    Some(format!("Import failed: {}", e)),
                ));
            }
        }
    }
    
    // Type check the document
    if check_quant {
        // Note: typecheck method would need to be implemented fully
        doc.typecheck()?;
    }
    
    Ok(doc)
    })
}

/// Resolve import URI to absolute file path
pub async fn resolve_file_import(
    uri: &str,
    path: &[String],
    importer: Option<&Document>,
) -> Result<String, WdlError> {
    // Handle HTTP/HTTPS URIs (no-op for now)
    if uri.starts_with("http://") || uri.starts_with("https://") {
        return Ok(uri.to_string());
    }
    
    // Handle file:/// URIs
    let uri = if uri.starts_with("file:///") {
        &uri[7..]
    } else {
        uri
    };
    
    let candidate_path = if Path::new(uri).is_absolute() {
        // Already absolute path
        PathBuf::from(uri)
    } else {
        // Relative path - search in path directories and importer directory
        let mut search_paths = path.to_vec();
        
        // Add importer directory if available
        if let Some(imp) = importer {
            if let Some(parent) = Path::new(&imp.pos.abspath).parent() {
                search_paths.push(parent.to_string_lossy().to_string());
            }
        } else {
            // Add current working directory
            if let Ok(cwd) = std::env::current_dir() {
                search_paths.push(cwd.to_string_lossy().to_string());
            }
        }
        
        // Find first existing file in search paths
        let mut found_path = None;
        for search_dir in search_paths.iter().rev() {
            let candidate = Path::new(search_dir).join(uri);
            if candidate.is_file() {
                found_path = Some(candidate);
                break;
            }
        }
        
        match found_path {
            Some(path) => path,
            None => {
                return Err(WdlError::import_error(
                    SourcePosition::new(
                        uri.to_string(),
                        uri.to_string(),
                        1, 1, 1, 1
                    ),
                    uri.to_string(),
                    Some("File not found".to_string()),
                ));
            }
        }
    };
    
    // Verify file exists
    if !candidate_path.is_file() && !uri.starts_with("/dev/fd/") {
        return Err(WdlError::import_error(
            SourcePosition::new(
                uri.to_string(),
                candidate_path.to_string_lossy().to_string(),
                1, 1, 1, 1
            ),
            uri.to_string(),
            Some("File not found".to_string()),
        ));
    }
    
    Ok(candidate_path.to_string_lossy().to_string())
}

/// Read source file content
pub async fn read_source_default(
    uri: &str,
    _path: &[String],
    _importer: Option<&Document>,
) -> Result<ReadSourceResult, WdlError> {
    let abspath = resolve_file_import(uri, _path, _importer).await?;
    
    let source_text = fs::read_to_string(&abspath).map_err(|e| {
        WdlError::import_error(
            SourcePosition::new(
                uri.to_string(),
                abspath.clone(),
                1, 1, 1, 1
            ),
            uri.to_string(),
            Some(format!("Failed to read file: {}", e)),
        )
    })?;
    
    Ok(ReadSourceResult {
        source_text,
        abspath,
    })
}

#[cfg(test)]
mod import_tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn test_resolve_file_import() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("test.wdl");
        fs::write(&file_path, "version 1.0\ntask hello {}").unwrap();
        
        let result = futures::executor::block_on(resolve_file_import(
            "test.wdl",
            &[temp_dir.path().to_string_lossy().to_string()],
            None,
        ));
        
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), file_path.to_string_lossy().to_string());
    }

    #[test]
    fn test_read_source_default() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("test.wdl");
        let content = "version 1.0\ntask hello {}";
        fs::write(&file_path, content).unwrap();
        
        let result = futures::executor::block_on(read_source_default(
            &file_path.to_string_lossy(),
            &[],
            None,
        ));
        
        assert!(result.is_ok());
        let read_result = result.unwrap();
        assert_eq!(read_result.source_text, content);
    }

    #[test]
    fn test_load_simple_document() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("simple.wdl");
        let content = r#"version 1.0
task hello {
    command {
        echo "Hello World"
    }
}"#;
        fs::write(&file_path, content).unwrap();
        
        let result = load(
            &file_path.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            false, // Don't type check for now
            10,
        );
        
        if let Err(ref e) = result {
            println!("Load failed: {}", e);
        }
        assert!(result.is_ok());
        let doc = result.unwrap();
        assert_eq!(doc.tasks.len(), 1);
        assert_eq!(doc.tasks[0].name, "hello");
        assert!(doc.imports.is_empty());
    }

    #[test]
    fn test_load_document_with_imports() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create imported file
        let imported_file = temp_dir.path().join("lib.wdl");
        let imported_content = r#"version 1.0
task greet {
    command {
        echo "Greetings"
    }
}"#;
        fs::write(&imported_file, imported_content).unwrap();
        
        // Create main file
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0

import "lib.wdl" as mylib

task hello {
    command {
        echo "Hello World"
    }
}"#;
        fs::write(&main_file, main_content).unwrap();
        
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            false, // Don't type check for now
            10,
        );
        
        if let Err(ref e) = result {
            println!("Load failed: {}", e);
        }
        assert!(result.is_ok());
        
        let doc = result.unwrap();
        assert_eq!(doc.tasks.len(), 1); // Main document task
        assert_eq!(doc.tasks[0].name, "hello");
        assert_eq!(doc.imports.len(), 1);
        
        // Check import
        let import = &doc.imports[0];
        assert_eq!(import.uri, "lib.wdl");
        assert_eq!(import.namespace, "mylib");
        
        // Check imported document is loaded
        assert!(import.doc.is_some());
        let imported_doc = import.doc.as_ref().unwrap();
        assert_eq!(imported_doc.tasks.len(), 1);
        assert_eq!(imported_doc.tasks[0].name, "greet");
    }

    #[test]
    fn test_call_resolution_with_imports() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create imported file with task
        let imported_file = temp_dir.path().join("utils.wdl");
        let imported_content = r#"version 1.0
task process_data {
    command {
        echo "Processing data"
    }
}"#;
        fs::write(&imported_file, imported_content).unwrap();
        
        // Create main file with workflow that calls imported task
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0

import "utils.wdl" as utils

workflow process_workflow {
    call utils.process_data
}"#;
        fs::write(&main_file, main_content).unwrap();
        
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            true, // Enable type checking to test call resolution
            10,
        );
        
        if let Err(ref e) = result {
            println!("Load failed: {}", e);
        }
        assert!(result.is_ok());
        
        let doc = result.unwrap();
        assert_eq!(doc.imports.len(), 1);
        
        // Check workflow exists and has been processed
        assert!(doc.workflow.is_some());
        let workflow = doc.workflow.as_ref().unwrap();
        assert_eq!(workflow.name, "process_workflow");
        assert_eq!(workflow.body.len(), 1);
        
        // Check call was resolved
        if let tree::WorkflowElement::Call(call) = &workflow.body[0] {
            assert_eq!(call.task, "utils.process_data");
            // TODO: Once call resolution is fully working, check that callee is resolved
            // assert!(call.callee.is_some());
        } else {
            panic!("Expected call in workflow body");
        }
    }

    #[test]
    fn test_struct_imports() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create imported file with struct
        let imported_file = temp_dir.path().join("types.wdl");
        let imported_content = r#"version 1.0
struct Person {
    String name
    Int age
}"#;
        fs::write(&imported_file, imported_content).unwrap();
        
        // Create main file that imports the struct
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0

import "types.wdl" as types

task use_person {
    command {
        echo "Hello World"
    }
}"#;
        fs::write(&main_file, main_content).unwrap();
        
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            true, // Enable type checking to test struct imports
            10,
        );
        
        if let Err(ref e) = result {
            println!("Load failed: {}", e);
        }
        assert!(result.is_ok());
        
        let doc = result.unwrap();
        assert_eq!(doc.imports.len(), 1);
        
        // Check that imported struct is available
        // The struct should be imported during type checking
        let imported_doc = doc.imports[0].doc.as_ref().unwrap();
        assert_eq!(imported_doc.struct_typedefs.len(), 1);
        assert_eq!(imported_doc.struct_typedefs[0].name, "Person");
        
        // After type checking, the main document should have imported structs
        // This would be populated by the import_structs method during typecheck
        // TODO: Verify struct imports are working once fully implemented
    }

    #[test]
    fn test_circular_import_detection() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create file A that imports B
        let file_a = temp_dir.path().join("a.wdl");
        let content_a = r#"version 1.0
import "b.wdl" as b
task task_a {
    command { echo "A" }
}"#;
        fs::write(&file_a, content_a).unwrap();
        
        // Create file B that imports A (circular)
        let file_b = temp_dir.path().join("b.wdl");
        let content_b = r#"version 1.0
import "a.wdl" as a
task task_b {
    command { echo "B" }
}"#;
        fs::write(&file_b, content_b).unwrap();
        
        let result = load(
            &file_a.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            false,
            3, // Low depth to trigger circular import detection
        );
        
        assert!(result.is_err());
        if let Err(e) = result {
            assert!(e.to_string().contains("exceeded import_max_depth"));
        }
    }

    #[test]
    fn test_missing_import_file() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create main file that imports non-existent file
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0
import "missing.wdl" as missing
task hello {
    command { echo "Hello" }
}"#;
        fs::write(&main_file, main_content).unwrap();
        
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            false,
            10,
        );
        
        assert!(result.is_err());
        if let Err(e) = result {
            assert!(e.to_string().contains("File not found"));
        }
    }

    #[test]
    fn test_multiple_imports_same_namespace() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create two files to import
        let file_a = temp_dir.path().join("a.wdl");
        let content_a = r#"version 1.0
task task_a { command { echo "A" } }"#;
        fs::write(&file_a, content_a).unwrap();
        
        let file_b = temp_dir.path().join("b.wdl");
        let content_b = r#"version 1.0
task task_b { command { echo "B" } }"#;
        fs::write(&file_b, content_b).unwrap();
        
        // Create main file with duplicate namespace imports
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0
import "a.wdl" as lib
import "b.wdl" as lib
task hello {
    command { echo "Hello" }
}"#;
        fs::write(&main_file, main_content).unwrap();
        
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            true, // Enable type checking to catch namespace collision
            10,
        );
        
        assert!(result.is_err());
        if let Err(e) = result {
            assert!(e.to_string().contains("Multiple imports with namespace"));
        }
    }

    #[test]
    fn test_nested_imports() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create level 3 file
        let file_c = temp_dir.path().join("c.wdl");
        let content_c = r#"version 1.0
task task_c { command { echo "C" } }"#;
        fs::write(&file_c, content_c).unwrap();
        
        // Create level 2 file that imports C
        let file_b = temp_dir.path().join("b.wdl");
        let content_b = r#"version 1.0
import "c.wdl" as c
task task_b { command { echo "B" } }"#;
        fs::write(&file_b, content_b).unwrap();
        
        // Create level 1 file that imports B
        let file_a = temp_dir.path().join("a.wdl");
        let content_a = r#"version 1.0
import "b.wdl" as b
task task_a { command { echo "A" } }"#;
        fs::write(&file_a, content_a).unwrap();
        
        let result = load(
            &file_a.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            false,
            10,
        );
        
        assert!(result.is_ok());
        let doc = result.unwrap();
        assert_eq!(doc.imports.len(), 1);
        
        // Check nested import structure
        let imported_b = doc.imports[0].doc.as_ref().unwrap();
        assert_eq!(imported_b.imports.len(), 1);
        
        let imported_c = imported_b.imports[0].doc.as_ref().unwrap();
        assert_eq!(imported_c.tasks.len(), 1);
        assert_eq!(imported_c.tasks[0].name, "task_c");
    }

    #[test]
    fn test_namespace_inference() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create file with complex name
        let imported_file = temp_dir.path().join("complex-name.wdl");
        let imported_content = r#"version 1.0
task hello { command { echo "Hello" } }"#;
        fs::write(&imported_file, imported_content).unwrap();
        
        // Import without explicit namespace
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0
import "complex-name.wdl"
task test { command { echo "Test" } }"#;
        fs::write(&main_file, main_content).unwrap();
        
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            false,
            10,
        );
        
        assert!(result.is_ok());
        let doc = result.unwrap();
        assert_eq!(doc.imports.len(), 1);
        // Namespace should be inferred as "complex-name" (filename without extension)
        assert_eq!(doc.imports[0].namespace, "complex-name");
    }

    #[test]
    fn test_runtime_import_execution() {
        let temp_dir = TempDir::new().unwrap();
        
        // Create imported task file
        let imported_file = temp_dir.path().join("lib.wdl");
        let imported_content = r#"version 1.0

task greet {
    input {
        String name
    }
    
    command {
        echo "Hello, ~{name}!"
    }
    
    output {
        String greeting = stdout()
    }
}"#;
        fs::write(&imported_file, imported_content).unwrap();
        
        // Create main workflow file that calls imported task
        let main_file = temp_dir.path().join("main.wdl");
        let main_content = r#"version 1.0

import "lib.wdl" as lib

workflow test_workflow {
    input {
        String person = "World"
    }
    
    call lib.greet { input: name = person }
    
    output {
        String result = greet.greeting
    }
}"#;
        fs::write(&main_file, main_content).unwrap();
        
        // Load and parse the document
        let result = load(
            &main_file.to_string_lossy(),
            Some(&[temp_dir.path().to_string_lossy().as_ref()]),
            true, // Enable type checking to resolve calls
            10,
        );
        
        assert!(result.is_ok(), "Failed to load document: {:?}", result.err());
        let doc = result.unwrap();
        
        // Verify import was loaded
        assert_eq!(doc.imports.len(), 1);
        assert_eq!(doc.imports[0].namespace, "lib");
        assert!(doc.imports[0].doc.is_some());
        
        // Verify workflow exists
        assert!(doc.workflow.is_some());
        let workflow = doc.workflow.as_ref().unwrap();
        
        // Verify call was resolved
        assert!(!workflow.body.is_empty());
        if let crate::tree::WorkflowElement::Call(call) = &workflow.body[0] {
            assert_eq!(call.task, "lib.greet");
            // Verify the call was resolved to point to the imported task
            assert!(call.callee.is_some(), "Call should have been resolved");
            if let Some(crate::tree::CalleeRef::Task(task)) = &call.callee {
                assert_eq!(task.name, "greet");
                assert_eq!(task.outputs.len(), 1);
                assert_eq!(task.outputs[0].name, "greeting");
            } else {
                panic!("Callee should be a Task");
            }
        } else {
            panic!("First workflow element should be a Call");
        }
        
        // Test would require runtime execution engine to actually execute
        // For now we verify the structure is correct for runtime execution
        println!("✅ Runtime import test passed - call was properly resolved for execution");
    }
}
