//! Token definitions for WDL parser

use crate::error::SourcePosition;
use std::fmt;

/// Token type for WDL lexer
#[derive(Debug, Clone, PartialEq)]
pub enum Token {
    // Keywords (version-specific)
    Keyword(String),
    
    // Identifiers
    Identifier(String),
    
    // Literals
    IntLiteral(i64),
    FloatLiteral(f64),
    BoolLiteral(bool),
    StringLiteral(String), // Raw string for now, will be parsed later
    
    // Operators - Arithmetic
    Plus,
    Minus,
    Star,
    Slash,
    Percent,
    
    // Operators - Comparison
    Equal,
    NotEqual,
    Less,
    LessEqual,
    Greater,
    GreaterEqual,
    
    // Operators - Logical
    And,
    Or,
    Not,
    
    // Operators - Assignment
    Assign,
    
    // Delimiters
    LeftParen,
    RightParen,
    LeftBracket,
    RightBracket,
    LeftBrace,
    RightBrace,
    
    // Punctuation
    Comma,
    Dot,
    Colon,
    Question,
    PlusQuestion, // +?
    
    // Special markers for commands and strings
    CommandStart,      // { or <<<
    CommandEnd,        // } or >>>
    PlaceholderStart,  // ${ or ~{
    PlaceholderEnd,    // }
    
    // String quotes
    SingleQuote,
    DoubleQuote,
    
    // Whitespace (preserved in certain contexts)
    Whitespace(String),
    Newline,
    
    // Comments (preserved for documentation)
    Comment(String),
    
    // End of file
    Eof,
}

impl Token {
    /// Check if this token is a keyword
    pub fn is_keyword(&self) -> bool {
        matches!(self, Token::Keyword(_))
    }
    
    /// Check if this token is an identifier
    pub fn is_identifier(&self) -> bool {
        matches!(self, Token::Identifier(_))
    }
    
    /// Check if this token is a literal
    pub fn is_literal(&self) -> bool {
        matches!(
            self,
            Token::IntLiteral(_)
                | Token::FloatLiteral(_)
                | Token::BoolLiteral(_)
                | Token::StringLiteral(_)
        )
    }
    
    /// Check if this token is an operator
    pub fn is_operator(&self) -> bool {
        matches!(
            self,
            Token::Plus
                | Token::Minus
                | Token::Star
                | Token::Slash
                | Token::Percent
                | Token::Equal
                | Token::NotEqual
                | Token::Less
                | Token::LessEqual
                | Token::Greater
                | Token::GreaterEqual
                | Token::And
                | Token::Or
                | Token::Not
                | Token::Assign
        )
    }
}

impl fmt::Display for Token {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Token::Keyword(s) => write!(f, "{}", s),
            Token::Identifier(s) => write!(f, "{}", s),
            Token::IntLiteral(n) => write!(f, "{}", n),
            Token::FloatLiteral(n) => write!(f, "{}", n),
            Token::BoolLiteral(b) => write!(f, "{}", b),
            Token::StringLiteral(s) => write!(f, "\"{}\"", s),
            
            Token::Plus => write!(f, "+"),
            Token::Minus => write!(f, "-"),
            Token::Star => write!(f, "*"),
            Token::Slash => write!(f, "/"),
            Token::Percent => write!(f, "%"),
            
            Token::Equal => write!(f, "=="),
            Token::NotEqual => write!(f, "!="),
            Token::Less => write!(f, "<"),
            Token::LessEqual => write!(f, "<="),
            Token::Greater => write!(f, ">"),
            Token::GreaterEqual => write!(f, ">="),
            
            Token::And => write!(f, "&&"),
            Token::Or => write!(f, "||"),
            Token::Not => write!(f, "!"),
            
            Token::Assign => write!(f, "="),
            
            Token::LeftParen => write!(f, "("),
            Token::RightParen => write!(f, ")"),
            Token::LeftBracket => write!(f, "["),
            Token::RightBracket => write!(f, "]"),
            Token::LeftBrace => write!(f, "{{"),
            Token::RightBrace => write!(f, "}}"),
            
            Token::Comma => write!(f, ","),
            Token::Dot => write!(f, "."),
            Token::Colon => write!(f, ":"),
            Token::Question => write!(f, "?"),
            Token::PlusQuestion => write!(f, "+?"),
            
            Token::CommandStart => write!(f, "command{{"),
            Token::CommandEnd => write!(f, "}}"),
            Token::PlaceholderStart => write!(f, "${{/~{{"),
            Token::PlaceholderEnd => write!(f, "}}"),
            
            Token::SingleQuote => write!(f, "'"),
            Token::DoubleQuote => write!(f, "\""),
            
            Token::Whitespace(s) => write!(f, "{}", s),
            Token::Newline => write!(f, "\\n"),
            Token::Comment(s) => write!(f, "#{}", s),
            
            Token::Eof => write!(f, "EOF"),
        }
    }
}

/// A token with its source position
#[derive(Debug, Clone, PartialEq)]
pub struct LocatedToken {
    pub token: Token,
    pub pos: SourcePosition,
}

impl LocatedToken {
    pub fn new(token: Token, pos: SourcePosition) -> Self {
        Self { token, pos }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_token_display() {
        assert_eq!(Token::Plus.to_string(), "+");
        assert_eq!(Token::And.to_string(), "&&");
        assert_eq!(Token::IntLiteral(42).to_string(), "42");
        assert_eq!(Token::Keyword("task".to_string()).to_string(), "task");
    }
    
    #[test]
    fn test_token_classification() {
        assert!(Token::Keyword("task".to_string()).is_keyword());
        assert!(Token::Identifier("foo".to_string()).is_identifier());
        assert!(Token::IntLiteral(42).is_literal());
        assert!(Token::Plus.is_operator());
        
        assert!(!Token::Plus.is_keyword());
        assert!(!Token::Comma.is_operator());
    }
}