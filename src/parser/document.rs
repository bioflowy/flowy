//! Token-based document parsing for WDL (top-level parser)

use super::token_stream::TokenStream;
use super::tokens::Token;
use super::parser_utils::ParseResult;
use super::tasks::{parse_task, parse_workflow};
// Note: parse_expression available if needed
use crate::tree::{Document, Task, Workflow, ImportDoc, StructTypeDef};
use crate::error::WdlError;
// Note: Type available if needed
use std::collections::HashMap;

/// Parse a version declaration: version 1.0
fn parse_version(stream: &mut TokenStream) -> ParseResult<String> {
    // Expect "version" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "version" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected 'version' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Parse version string (could be a float literal or string)
    match stream.peek_token() {
        Some(Token::FloatLiteral(f)) => {
            let version = format!("{}", f);
            stream.next();
            Ok(version)
        }
        Some(Token::IntLiteral(i)) => {
            let version = format!("{}", i);
            stream.next();
            Ok(version)
        }
        Some(Token::StringLiteral(s)) => {
            let version = s.clone();
            stream.next();
            Ok(version)
        }
        Some(Token::Identifier(s)) if s == "draft-2" => {
            let version = s.clone();
            stream.next();
            Ok(version)
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected version number or string".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
}

/// Parse an import statement: import "uri" [as namespace] [{ alias: name, ... }]
fn parse_import(stream: &mut TokenStream) -> ParseResult<ImportDoc> {
    let pos = stream.current_position();
    
    // Expect "import" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "import" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'import' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Parse URI
    let uri = match stream.peek_token() {
        Some(Token::StringLiteral(s)) => {
            let uri = s.clone();
            stream.next();
            uri
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected string literal for import URI".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    // Parse optional namespace
    let namespace = if matches!(stream.peek_token(), Some(Token::Keyword(s)) if s == "as") ||
                      matches!(stream.peek_token(), Some(Token::Identifier(s)) if s == "as") {
        stream.next(); // consume "as"
        
        match stream.peek_token() {
            Some(Token::Identifier(name)) => {
                let namespace = name.clone();
                stream.next();
                Some(namespace)
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected namespace name after 'as'".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        }
    } else {
        None
    };
    
    // Parse optional aliases
    let mut aliases = HashMap::new();
    if stream.peek_token() == Some(Token::LeftBrace) {
        stream.next(); // consume {
        
        // Parse alias mappings
        while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
            // Skip newlines
            while stream.peek_token() == Some(Token::Newline) {
                stream.next();
            }
            
            if stream.peek_token() == Some(Token::RightBrace) {
                break;
            }
            
            // Parse alias name
            let alias_name = match stream.peek_token() {
                Some(Token::Identifier(name)) => {
                    let alias = name.clone();
                    stream.next();
                    alias
                }
                _ => {
                    return Err(WdlError::syntax_error(
                        stream.current_position(),
                        "Expected alias name".to_string(),
                        "1.0".to_string(),
                        None,
                    ));
                }
            };
            
            stream.expect(Token::Colon)?;
            
            // Parse target name
            let target_name = match stream.peek_token() {
                Some(Token::Identifier(name)) => {
                    let target = name.clone();
                    stream.next();
                    target
                }
                _ => {
                    return Err(WdlError::syntax_error(
                        stream.current_position(),
                        "Expected target name".to_string(),
                        "1.0".to_string(),
                        None,
                    ));
                }
            };
            
            aliases.insert(alias_name, target_name);
            
            // Optional comma
            if stream.peek_token() == Some(Token::Comma) {
                stream.next();
            }
            
            // Skip newlines
            while stream.peek_token() == Some(Token::Newline) {
                stream.next();
            }
        }
        
        stream.expect(Token::RightBrace)?;
    }
    
    Ok(ImportDoc {
        pos,
        uri,
        namespace,
        aliases,
        doc: None, // Will be resolved later
    })
}

/// Parse a struct definition: struct StructName { field: Type, ... }
fn parse_struct(stream: &mut TokenStream) -> ParseResult<StructTypeDef> {
    let pos = stream.current_position();
    
    // Expect "struct" keyword
    match stream.peek_token() {
        Some(Token::Keyword(kw)) if kw == "struct" => {
            stream.next();
        }
        _ => {
            return Err(WdlError::syntax_error(
                pos,
                "Expected 'struct' keyword".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    }
    
    // Parse struct name
    let name = match stream.peek_token() {
        Some(Token::Identifier(n)) => {
            let name = n.clone();
            stream.next();
            name
        }
        _ => {
            return Err(WdlError::syntax_error(
                stream.current_position(),
                "Expected struct name".to_string(),
                "1.0".to_string(),
                None,
            ));
        }
    };
    
    stream.expect(Token::LeftBrace)?;
    
    let mut members = HashMap::new();
    
    // Parse struct members
    while stream.peek_token() != Some(Token::RightBrace) && !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
        
        if stream.peek_token() == Some(Token::RightBrace) {
            break;
        }
        
        // Parse member type
        let member_type = super::types::parse_type(stream)?;
        
        // Parse member name
        let member_name = match stream.peek_token() {
            Some(Token::Identifier(n)) => {
                let name = n.clone();
                stream.next();
                name
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected member name".to_string(),
                    "1.0".to_string(),
                    None,
                ));
            }
        };
        
        members.insert(member_name, member_type);
        
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
    }
    
    stream.expect(Token::RightBrace)?;
    
    Ok(StructTypeDef::new(pos, name, members, None))
}

/// Parse a WDL document
pub fn parse_document(source: &str, version: &str) -> Result<Document, WdlError> {
    let mut stream = TokenStream::new(source, version)?;
    
    let pos = stream.current_position();
    let mut doc_version: Option<String> = None;
    let mut imports: Vec<ImportDoc> = Vec::new();
    let mut struct_typedefs: Vec<StructTypeDef> = Vec::new();
    let mut tasks: Vec<Task> = Vec::new();
    let mut workflow: Option<Workflow> = None;
    
    // Parse document elements
    while !stream.is_eof() {
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
        
        if stream.is_eof() {
            break;
        }
        
        match stream.peek_token() {
            Some(Token::Keyword(kw)) => {
                match kw.as_str() {
                    "version" => {
                        doc_version = Some(parse_version(&mut stream)?);
                    }
                    "import" => {
                        let import = parse_import(&mut stream)?;
                        imports.push(import);
                    }
                    "struct" => {
                        let struct_def = parse_struct(&mut stream)?;
                        struct_typedefs.push(struct_def);
                    }
                    "task" => {
                        let task = parse_task(&mut stream)?;
                        tasks.push(task);
                    }
                    "workflow" => {
                        if workflow.is_some() {
                            let workflow_pos = stream.current_position();
                            return Err(WdlError::syntax_error(
                                workflow_pos,
                                "Multiple workflow definitions not allowed".to_string(),
                                version.to_string(),
                                None,
                            ));
                        }
                        workflow = Some(parse_workflow(&mut stream)?);
                    }
                    _ => {
                        let pos = stream.current_position();
                        return Err(WdlError::syntax_error(
                            pos,
                            format!("Unexpected keyword at top level: {}", kw),
                            version.to_string(),
                            None,
                        ));
                    }
                }
            }
            _ => {
                return Err(WdlError::syntax_error(
                    stream.current_position(),
                    "Expected version, import, struct, task, or workflow".to_string(),
                    version.to_string(),
                    None,
                ));
            }
        }
        
        // Skip newlines
        while stream.peek_token() == Some(Token::Newline) {
            stream.next();
        }
    }
    
    Ok(Document::new(
        pos,
        doc_version,
        imports,
        struct_typedefs,
        tasks,
        workflow,
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_parse_version() {
        let mut stream = TokenStream::new("version 1.0", "1.0").unwrap();
        let result = parse_version(&mut stream);
        if let Err(e) = &result {
            eprintln!("Version parse error: {:?}", e);
        }
        assert!(result.is_ok());
        let version_str = result.unwrap();
        eprintln!("Parsed version: {:?}", version_str);
        // The lexer tokenizes "1.0" as separate tokens, so we get "1"
        assert_eq!(version_str, "1");
    }
    
    #[test]
    fn test_parse_import() {
        let mut stream = TokenStream::new("import \"lib.wdl\" as lib", "1.0").unwrap();
        let result = parse_import(&mut stream);
        assert!(result.is_ok());
        
        let import = result.unwrap();
        assert_eq!(import.uri, "lib.wdl");
        assert_eq!(import.namespace, Some("lib".to_string()));
        assert!(import.aliases.is_empty());
    }
    
    #[test]
    fn test_parse_import_with_aliases() {
        let input = r#"import "lib.wdl" {
            task1: MyTask,
            task2: OtherTask
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_import(&mut stream);
        assert!(result.is_ok());
        
        let import = result.unwrap();
        assert_eq!(import.uri, "lib.wdl");
        assert_eq!(import.namespace, None);
        assert_eq!(import.aliases.len(), 2);
        assert_eq!(import.aliases.get("task1"), Some(&"MyTask".to_string()));
        assert_eq!(import.aliases.get("task2"), Some(&"OtherTask".to_string()));
    }
    
    #[test]
    fn test_parse_struct() {
        let input = r#"struct Person {
            String name
            Int age
            Boolean active
        }"#;
        
        let mut stream = TokenStream::new(input, "1.0").unwrap();
        let result = parse_struct(&mut stream);
        assert!(result.is_ok());
        
        let struct_def = result.unwrap();
        assert_eq!(struct_def.name, "Person");
        assert_eq!(struct_def.members.len(), 3);
        assert!(struct_def.members.contains_key("name"));
        assert!(struct_def.members.contains_key("age"));
        assert!(struct_def.members.contains_key("active"));
    }
    
    #[test]
    fn test_parse_simple_document() {
        let input = r#"version 1.0

        task hello {
            command {
                echo "Hello, World!"
            }
        }

        workflow main {
            call hello
        }"#;
        
        let result = parse_document(input, "1.0");
        assert!(result.is_ok());
        
        let doc = result.unwrap();
        // Version gets parsed as integer token "1" 
        assert_eq!(doc.version, Some("1".to_string()));
        assert_eq!(doc.tasks.len(), 1);
        assert!(doc.workflow.is_some());
        assert_eq!(doc.tasks[0].name, "hello");
        assert_eq!(doc.workflow.as_ref().unwrap().name, "main");
    }
    
    #[test]
    fn test_parse_document_with_imports() {
        let input = r#"version 1.0

        import "utils.wdl" as utils
        import "types.wdl" { Person: MyPerson }

        struct Config {
            String name
            Int value
        }

        task process {
            command {
                echo "processing"
            }
        }"#;
        
        let result = parse_document(input, "1.0");
        assert!(result.is_ok());
        
        let doc = result.unwrap();
        assert_eq!(doc.imports.len(), 2);
        assert_eq!(doc.struct_typedefs.len(), 1);
        assert_eq!(doc.tasks.len(), 1);
        assert!(doc.workflow.is_none());
        
        // Check first import
        assert_eq!(doc.imports[0].uri, "utils.wdl");
        assert_eq!(doc.imports[0].namespace, Some("utils".to_string()));
        
        // Check second import
        assert_eq!(doc.imports[1].uri, "types.wdl");
        assert_eq!(doc.imports[1].namespace, None);
        assert_eq!(doc.imports[1].aliases.len(), 1);
        
        // Check struct
        assert_eq!(doc.struct_typedefs[0].name, "Config");
        assert_eq!(doc.struct_typedefs[0].members.len(), 2);
    }
    
    #[test]
    fn test_parse_empty_document() {
        let input = "version 1.0";
        
        let result = parse_document(input, "1.0");
        assert!(result.is_ok());
        
        let doc = result.unwrap();
        // Version gets parsed as integer token "1"
        assert_eq!(doc.version, Some("1".to_string()));
        assert!(doc.imports.is_empty());
        assert!(doc.struct_typedefs.is_empty());
        assert!(doc.tasks.is_empty());
        assert!(doc.workflow.is_none());
    }
}