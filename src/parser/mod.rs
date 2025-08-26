//! WDL Parser implementation using nom

pub mod tokens;
pub mod keywords;
pub mod lexer;
pub mod token_stream;
pub mod parser_utils;
pub mod literals;
pub mod types;
pub mod expressions;
pub mod declarations;
pub mod statements;
pub mod tasks;
pub mod document;
pub mod command_parser;
pub mod command_preprocessor;


use crate::error::WdlError;
use crate::tree::Document;
use nom_locate::LocatedSpan;

pub type Span<'a> = LocatedSpan<&'a str>;

/// Parse a WDL document from source text
pub fn parse_document(source: &str, version: &str) -> Result<Document, WdlError> {
    // First, preprocess to extract command blocks
    let preprocessed = command_preprocessor::preprocess_commands(source)?;
    
    // Parse the processed source (without problematic command content)
    let doc = document::parse_document(&preprocessed.processed_source, version)?;
    
    // TODO: Re-inject command blocks into the parsed AST
    // For now, we'll just parse with placeholders
    
    Ok(doc)
}

