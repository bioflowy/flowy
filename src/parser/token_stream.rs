//! Token stream for parsing WDL

use super::lexer::{normal_token, stateful_token, Lexer, LexerMode, Span};
use super::tokens::{LocatedToken, Token};
use crate::error::{SourcePosition, WdlError};
use nom::multi::many0;

/// A stream of tokens with lazy tokenization and lookahead capability
#[derive(Debug, Clone)]
pub struct TokenStream {
    source: String,
    source_position: usize, // Current byte position in source
    token_position: usize,  // Current token number (for backtracking)
    lexer: Lexer,
    version: String,
    current_token: Option<LocatedToken>, // Hold current token for peek()
    generated_tokens: Vec<LocatedToken>, // Store generated tokens for backtracking
}

impl TokenStream {
    /// Create a new token stream from source text
    pub fn new(source: &str, version: &str) -> Result<Self, WdlError> {
        let lexer = Lexer::new(version);
        Ok(TokenStream {
            source: source.to_string(),
            source_position: 0,
            token_position: 0,
            lexer,
            version: version.to_string(),
            current_token: None,
            generated_tokens: Vec::new(),
        })
    }

    /// Generate the next token from the source at the current position
    fn generate_next_token(&mut self) -> Result<Option<LocatedToken>, WdlError> {
        loop {
            if self.source_position >= self.source.len() {
                return Ok(None); // EOF
            }

            let remaining_source = &self.source[self.source_position..];
            let input = Span::new(remaining_source);
            let tokenizer = stateful_token(&self.lexer);

            match tokenizer(input) {
                Ok((remaining, token)) => {
                    // Update source position
                    let consumed_bytes = remaining_source.len() - remaining.fragment().len();
                    self.source_position += consumed_bytes;

                    // Filter out whitespace, newlines, and comments only in normal mode
                    // In command mode, preserve whitespace and newlines as they are significant
                    if self.lexer.current_mode() != crate::parser::lexer::LexerMode::Command
                        && matches!(
                            token.token,
                            Token::Whitespace(_) | Token::Newline | Token::Comment(_)
                        )
                    {
                        // Skip this token and continue the loop
                        continue;
                    }

                    return Ok(Some(token));
                }
                Err(e) => {
                    // Convert nom error to WdlError
                    let pos = SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1);
                    return Err(WdlError::syntax_error(
                        pos,
                        format!("Tokenization error: {:?}", e),
                        self.version.clone(),
                        None,
                    ));
                }
            }
        }
    }

    /// Switch to command mode for parsing command blocks
    pub fn enter_command_mode(&mut self) {
        self.lexer.push_mode(LexerMode::Command);
        self.current_token = None; // Reset current token so next generation uses new mode
    }

    /// Exit command mode back to normal mode
    pub fn exit_command_mode(&mut self) {
        self.lexer.pop_mode();
        self.current_token = None; // Reset current token so next generation uses new mode
    }

    /// Get current lexer mode
    pub fn current_mode(&self) -> LexerMode {
        self.lexer.current_mode()
    }

    /// Parse command content using command-mode tokenization
    /// This method allows re-tokenizing specific content with command-mode rules
    pub fn parse_command_content(&self, command_text: &str) -> Result<Vec<LocatedToken>, WdlError> {
        let mut command_lexer = Lexer::new(&self.version);
        command_lexer.push_mode(LexerMode::Command);

        let input = Span::new(command_text);
        let tokenizer = stateful_token(&command_lexer);

        // Parse all tokens in command mode
        let result = many0(tokenizer)(input);

        match result {
            Ok((_remaining, tokens)) => {
                // It's OK if we don't consume all input in command mode
                // since command content can contain arbitrary shell syntax
                Ok(tokens)
            }
            Err(e) => {
                let pos = SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1);
                Err(WdlError::syntax_error(
                    pos,
                    format!("Command tokenization error: {:?}", e),
                    self.version.clone(),
                    None,
                ))
            }
        }
    }

    /// Peek at the current token without consuming it
    pub fn peek(&mut self) -> Option<&LocatedToken> {
        if self.current_token.is_none() {
            // First check if we can get token from generated_tokens (backtracking case)
            if let Some(token) = self.generated_tokens.get(self.token_position) {
                self.current_token = Some(token.clone());
            } else {
                // Generate new token
                match self.generate_next_token() {
                    Ok(Some(token)) => {
                        // Store the token in generated_tokens immediately for backtracking
                        if self.generated_tokens.len() <= self.token_position {
                            self.generated_tokens.push(token.clone());
                        }
                        self.current_token = Some(token);
                    }
                    Ok(None) => return None, // EOF
                    Err(_) => return None,   // Error in tokenization
                }
            }
        }
        self.current_token.as_ref()
    }

    /// Peek at the nth token ahead without consuming
    pub fn peek_ahead(&mut self, n: usize) -> Option<&LocatedToken> {
        if n == 0 {
            return self.peek();
        }

        // Ensure we have generated enough tokens
        while self.generated_tokens.len() <= self.token_position + n {
            match self.generate_next_token() {
                Ok(Some(token)) => {
                    self.generated_tokens.push(token);
                }
                Ok(None) => break, // EOF
                Err(_) => break,   // Error
            }
        }

        self.generated_tokens.get(self.token_position + n)
    }

    /// Get the current token type
    pub fn peek_token(&mut self) -> Option<Token> {
        self.peek().map(|t| t.token.clone())
    }

    /// Consume and return the current token
    pub fn next(&mut self) -> Option<LocatedToken> {
        // If we have a current token, return it and advance
        if let Some(token) = self.current_token.take() {
            // Token should already be in generated_tokens from peek()
            self.token_position += 1;
            return Some(token);
        }

        // Otherwise, check if we can get from generated tokens (backtracking case)
        if let Some(token) = self.generated_tokens.get(self.token_position) {
            let token = token.clone();
            self.token_position += 1;
            return Some(token);
        }

        // Generate new token (this should be rare since peek() usually handles it)
        match self.generate_next_token() {
            Ok(Some(token)) => {
                if self.generated_tokens.len() <= self.token_position {
                    self.generated_tokens.push(token.clone());
                }
                self.token_position += 1;
                Some(token)
            }
            Ok(None) => None, // EOF
            Err(_) => None,   // Error
        }
    }

    /// Check if we're at the end of the stream
    pub fn is_eof(&mut self) -> bool {
        // We're at EOF if we can't peek at the next token
        self.peek().is_none()
    }

    /// Get the current position in the token stream
    pub fn position(&self) -> usize {
        self.token_position
    }

    /// Set the position in the token stream (for backtracking)
    pub fn set_position(&mut self, pos: usize) {
        if pos <= self.generated_tokens.len() {
            self.token_position = pos;
            self.current_token = None; // Clear current token
        }
        // Note: We don't update source_position here as it's only used for forward tokenization
        // Backtracking within generated tokens works without source position updates
    }

    /// Consume a specific token type, returning an error if it doesn't match
    pub fn expect(&mut self, expected: Token) -> Result<LocatedToken, WdlError> {
        // Get current token info for error messages
        let current_token = self.peek_token();
        let current_pos = self.peek().map(|t| t.pos.clone());

        match current_token {
            Some(token) if token == expected => Ok(self.next().unwrap()),
            Some(token) => {
                let pos = current_pos.unwrap_or_else(|| {
                    SourcePosition::new("".to_string(), "".to_string(), 1, 1, 1, 1)
                });
                Err(WdlError::syntax_error(
                    pos,
                    format!("Expected {:?}, found {:?}", expected, token),
                    self.version.clone(),
                    None,
                ))
            }
            None => {
                let pos = if let Some(last) = self.generated_tokens.last() {
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
            Some(token) if token == *expected => self.next(),
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
            if predicate(&token) {
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
            if predicate(&token) {
                break;
            }
            self.next();
        }
    }

    /// Get the source position of the current token
    pub fn current_position(&mut self) -> SourcePosition {
        if let Some(token) = self.peek() {
            token.pos.clone()
        } else if let Some(last) = self.generated_tokens.last() {
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
                .filter(|t| {
                    !matches!(
                        t.token,
                        Token::Whitespace(_) | Token::Newline | Token::Comment(_)
                    )
                })
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
pub fn tokenize_with_whitespace(
    source: &str,
    version: &str,
) -> Result<Vec<LocatedToken>, WdlError> {
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

    #[test]
    fn test_lazy_tokenization() {
        let source = "x + y";
        let mut stream = TokenStream::new(source, "1.0").unwrap();

        // Verify tokens are generated on demand
        assert_eq!(stream.generated_tokens.len(), 0);

        // Peek should generate first token
        let token = stream.peek().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "x"));

        // Next should advance position
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "x"));
        assert_eq!(stream.position(), 1);

        // Continue with remaining tokens
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Plus));

        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "y"));

        // Should be at EOF
        assert!(stream.is_eof());
    }

    #[test]
    fn test_mode_change_resets_current_token() {
        let source = "test";
        let mut stream = TokenStream::new(source, "1.0").unwrap();

        // Peek to generate current token
        let token = stream.peek().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "test"));

        // Change mode should reset current token
        stream.enter_command_mode();
        assert!(stream.current_token.is_none());

        // Exit mode should also reset current token
        stream.exit_command_mode();
        assert!(stream.current_token.is_none());
    }

    #[test]
    fn test_backtracking() {
        let source = "a + b * c";
        let mut stream = TokenStream::new(source, "1.0").unwrap();

        // Read some tokens
        let _token1 = stream.next().unwrap(); // a
        let _token2 = stream.next().unwrap(); // +
        let _token3 = stream.next().unwrap(); // b
        assert_eq!(stream.position(), 3);

        // Backtrack to position 1
        stream.set_position(1);
        assert_eq!(stream.position(), 1);

        // Should be able to re-read from position 1
        let token = stream.next().unwrap(); // +
        assert!(matches!(token.token, Token::Plus));
        assert_eq!(stream.position(), 2);
    }

    #[test]
    fn test_try_parse_scenario() {
        let source = "foo bar";
        let mut stream = TokenStream::new(source, "1.0").unwrap();

        // First, successful parse of "foo"
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "foo"));
        assert_eq!(stream.position(), 1);

        // Now we're at "bar" - this should work
        let token = stream.peek().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "bar"));

        // Save position for backtracking test
        let pos = stream.position();

        // Try to parse a plus (should fail)
        let result = stream.expect(Token::Plus);
        assert!(result.is_err());

        // Set position back
        stream.set_position(pos);
        assert_eq!(stream.position(), pos);

        // Now we should be able to parse "bar"
        let token = stream.next().unwrap();
        assert!(matches!(token.token, Token::Identifier(ref s) if s == "bar"));
    }
}
