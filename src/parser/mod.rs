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


use crate::error::WdlError;
use crate::tree::Document;
use nom_locate::LocatedSpan;

pub type Span<'a> = LocatedSpan<&'a str>;

/// Parse a WDL document from source text
pub fn parse_document(source: &str, version: &str) -> Result<Document, WdlError> {
    // Use the document parser
    document::parse_document(source, version)
}

