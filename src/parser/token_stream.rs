//! Token stream for parsing WDL

use super::tokens::{Token, LocatedToken};
use super::lexer::{Span, normal_token};
use crate::error::{SourcePosition, WdlError};
use nom::multi::many0;

/// A stream of tokens with lookahead capability
#[derive(Debug, Clone)]
pub struct TokenStream {
    tokens: Vec<LocatedToken>,
    position: usize,
    version: String,
}

impl TokenStream {
    /// Create a new token stream from source text
    pub fn new(source: &str, version: &str) -> Result<Self, WdlError> {
        let tokens = tokenize(source, version)?;
        Ok(TokenStream {
            tokens,
            position: 0,
            version: version.to_string(),
        })
    }
    
    /// Peek at the current token without consuming it
    pub fn peek(&self) -> Option<&LocatedToken> {
        self.tokens.get(self.position)
    }
    
    /// Peek at the nth token ahead without consuming
    pub fn peek_ahead(&self, n: usize) -> Option<&LocatedToken> {
        self.tokens.get(self.position + n)
    }
    
    /// Get the current token type
    pub fn peek_token(&self) -> Option<&Token> {
        self.peek().map(|t| &t.token)
    }
    
    /// Consume and return the current token
    pub fn next(&mut self) -> Option<LocatedToken> {
        if self.position < self.tokens.len() {
            let token = self.tokens[self.position].clone();
            self.position += 1;
            Some(token)
        } else {
            None
        }
    }
    
    /// Check if we're at the end of the stream
    pub fn is_eof(&self) -> bool {
        self.position >= self.tokens.len()
    }
    
    /// Get the current position in the token stream
    pub fn position(&self) -> usize {
        self.position
    }
    
    /// Set the position in the token stream (for backtracking)
    pub fn set_position(&mut self, pos: usize) {
        self.position = pos.min(self.tokens.len());
    }
    
    /// Consume a specific token type, returning an error if it doesn't match
    pub fn expect(&mut self, expected: Token) -> Result<LocatedToken, WdlError> {
        match self.peek_token() {
            Some(token) if *token == expected => {
                Ok(self.next().unwrap())
            }
            Some(token) => {
                let pos = self.peek().unwrap().pos.clone();
                Err(WdlError::syntax_error(
                    pos,
                    format!("Expected {:?}, found {:?}", expected, token),
                    self.version.clone(),
                    None,
                ))
            }
            None => {
                let pos = if let Some(last) = self.tokens.last() {
                    last.pos.clone()
                } else {
                    SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1)
                };
                Err(WdlError::syntax_error(
                    pos,
                    format!("Expected {:?}, found EOF", expected),
                    self.version.clone(),
                    None,
                ))
            }
        }
    }
    
    /// Try to consume a specific token type
    pub fn try_consume(&mut self, expected: &Token) -> Option<LocatedToken> {
        match self.peek_token() {
            Some(token) if token == expected => self.next(),
            _ => None,
        }
    }
    
    /// Consume tokens while a predicate is true
    pub fn consume_while<F>(&mut self, mut predicate: F) -> Vec<LocatedToken>
    where
        F: FnMut(&Token) -> bool,
    {
        let mut tokens = Vec::new();
        while let Some(token) = self.peek_token() {
            if predicate(token) {
                tokens.push(self.next().unwrap());
            } else {
                break;
            }
        }
        tokens
    }
    
    /// Skip tokens until a predicate is true
    pub fn skip_until<F>(&mut self, mut predicate: F)
    where
        F: FnMut(&Token) -> bool,
    {
        while let Some(token) = self.peek_token() {
            if predicate(token) {
                break;
            }
            self.next();
        }
    }
    
    /// Get the source position of the current token
    pub fn current_position(&self) -> SourcePosition {
        if let Some(token) = self.peek() {
            token.pos.clone()
        } else if let Some(last) = self.tokens.last() {
            last.pos.clone()
        } else {
            SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1)
        }
    }
}

/// Tokenize a string into a vector of tokens, filtering out whitespace and comments
pub fn tokenize(source: &str, version: &str) -> Result<Vec<LocatedToken>, WdlError> {
    let input = Span::new(source);
    let tokenizer = normal_token(version);
    
    // Parse all tokens
    let result = many0(tokenizer)(input);
    
    match result {
        Ok((remaining, tokens)) => {
            // Check if we consumed all input
            if !remaining.fragment().is_empty() {
                let pos = super::lexer::span_to_position(remaining);
                return Err(WdlError::syntax_error(
                    pos,
                    format!("Unexpected input: {}", remaining.fragment()),
                    version.to_string(),
                    None,
                ));
            }
            
            // Filter out whitespace, newlines, and comments
            let filtered_tokens: Vec<LocatedToken> = tokens
                .into_iter()
                .filter(|t| !matches!(
                    t.token,
                    Token::Whitespace(_) | Token::Newline | Token::Comment(_)
                ))
                .collect();
            
            Ok(filtered_tokens)
        }
        Err(e) => {
            let pos = SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1);
            Err(WdlError::syntax_error(
                pos,
                format!("Tokenization error: {:?}", e),
                version.to_string(),
                None,
            ))
        }
    }
}

/// Tokenize with whitespace preserved (for command blocks, etc.)
pub fn tokenize_with_whitespace(source: &str, version: &str) -> Result<Vec<LocatedToken>, WdlError> {
    let input = Span::new(source);
    let tokenizer = normal_token(version);
    
    // Parse all tokens
    let result = many0(tokenizer)(input);
    
    match result {
        Ok((remaining, tokens)) => {
            // Check if we consumed all input
            if !remaining.fragment().is_empty() {
                let pos = super::lexer::span_to_position(remaining);
                return Err(WdlError::syntax_error(
                    pos,
                    format!("Unexpected input: {}", remaining.fragment()),
                    version.to_string(),
                    None,
                ));
            }
            
            Ok(tokens)
        }
        Err(e) => {
            let pos = SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1);
            Err(WdlError::syntax_error(
                pos,
                format!("Tokenization error: {:?}", e),
                version.to_string(),
                None,
            ))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_tokenize_simple() {
        let source = "1 + 2";
        let tokens = tokenize(source, "1.0").unwrap();
        assert_eq!(tokens.len(), 3); // 1, +, 2 (whitespace filtered)
        
        assert!(matches!(tokens[0].token, Token::IntLiteral(1)));
        assert!(matches!(tokens[1].token, Token::Plus));
        assert!(matches!(tokens[2].token, Token::IntLiteral(2)));
    }
    
    #[test]
    fn test_tokenize_with_comments() {
        let source = "x = 42 # comment\ny = 3";
        let tokens = tokenize(source, "1.0").unwrap();
        // Should have: x, =, 42, y, =, 3 (comment and newlines filtered)
        assert_eq!(tokens.len(), 6);
    }
    
    #[test]
    fn test_token_stream() {
        let source = "foo + bar";
        let mut stream = TokenStream::new(source, "1.0").unwrap();
        
        assert!(!stream.is_eof());
        
        // Peek doesn't consume
        let peeked = stream.peek_token().unwrap();
        assert!(matches!(peeked, Token::Identifier(_)));
        
        // Next consumes
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "foo"));
        
        // Now at +
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Plus));
        
        // Now at bar
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "bar"));
        
        // Now at EOF
        assert!(stream.is_eof());
        assert!(stream.next().is_none());
    }
    
    #[test]
    fn test_expect() {
        let source = "( 42 )";
        let mut stream = TokenStream::new(source, "1.0").unwrap();
        
        // Expect left paren
        let result = stream.expect(Token::LeftParen);
        assert!(result.is_ok());
        
        // Expect number (will fail)
        let result = stream.expect(Token::Plus);
        assert!(result.is_err());
        
        // Still at 42
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::IntLiteral(42)));
    }
}