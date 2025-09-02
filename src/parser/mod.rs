//! WDL Parser implementation using nom

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
    // Parse source directly using stateful lexer (no preprocessing needed)
    document::parse_document(source, version)
}

/// Parse a WDL document from source text with filename for better error reporting
pub fn parse_document_with_filename(
    source: &str,
    version: &str,
    filename: &str,
) -> Result<Document, WdlError> {
    // Parse source directly using stateful lexer (no preprocessing needed)
    document::parse_document_with_filename(source, version, filename)
}
