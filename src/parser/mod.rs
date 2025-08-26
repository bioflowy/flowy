//! WDL Parser implementation using nom

pub mod command_parser;
pub mod command_preprocessor;
pub mod declarations;
pub mod document;
pub mod expressions;
pub mod keywords;
pub mod lexer;
pub mod literals;
pub mod parser_utils;
pub mod statements;
pub mod tasks;
pub mod token_stream;
pub mod tokens;
pub mod types;

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
