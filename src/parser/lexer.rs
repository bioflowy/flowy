//! Stateful lexer for WDL parsing

use crate::error::SourcePosition;
use super::tokens::{Token, LocatedToken};
use super::keywords::is_keyword;
use nom::{
    IResult,
    branch::alt,
    bytes::complete::{tag, take_while, take_while1, take_until},
    character::complete::{char, digit1, alpha1, alphanumeric1, line_ending},
    combinator::{map, opt, recognize, value},
    multi::many0,
    sequence::{pair, preceded, tuple, delimited},
};
use nom_locate::LocatedSpan;

pub type Span<'a> = LocatedSpan<&'a str>;

/// Lexer mode for context-aware tokenization
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LexerMode {
    /// Normal WDL code
    Normal,
    /// Inside command{} or <<<>>>
    Command,
    /// Inside string literal
    StringLiteral,
    /// Inside placeholder ${} or ~{}
    Placeholder,
}

/// Stateful lexer for WDL
#[derive(Debug, Clone)]
pub struct Lexer {
    mode_stack: Vec<LexerMode>,
    #[allow(dead_code)]
    version: String,
}

impl Lexer {
    /// Create a new lexer for the specified WDL version
    pub fn new(version: &str) -> Self {
        Self {
            mode_stack: vec![LexerMode::Normal],
            version: version.to_string(),
        }
    }
    
    /// Get the current lexer mode
    pub fn current_mode(&self) -> LexerMode {
        *self.mode_stack.last().unwrap_or(&LexerMode::Normal)
    }
    
    /// Push a new mode onto the stack
    pub fn push_mode(&mut self, mode: LexerMode) {
        self.mode_stack.push(mode);
    }
    
    /// Pop the current mode from the stack
    pub fn pop_mode(&mut self) -> Option<LexerMode> {
        if self.mode_stack.len() > 1 {
            self.mode_stack.pop()
        } else {
            None
        }
    }
    
    /// Check if whitespace should be preserved in the current mode
    pub fn preserve_whitespace(&self) -> bool {
        match self.current_mode() {
            LexerMode::Command | LexerMode::StringLiteral => true,
            _ => false,
        }
    }
}

/// Convert a span to a source position
pub fn span_to_position(span: Span) -> SourcePosition {
    SourcePosition::new(
        "".to_string(),
        "".to_string(),
        span.location_line(),
        span.get_utf8_column() as u32,
        span.location_line(),
        (span.get_utf8_column() + span.fragment().len()) as u32,
    )
}

// Basic token parsers

/// Parse whitespace
pub fn whitespace(input: Span) -> IResult<Span, Token> {
    map(
        take_while1(|c: char| c == ' ' || c == '\t'),
        |s: Span| Token::Whitespace(s.fragment().to_string())
    )(input)
}

/// Parse newline
pub fn newline(input: Span) -> IResult<Span, Token> {
    map(
        line_ending,
        |_| Token::Newline
    )(input)
}

/// Parse a comment
pub fn comment(input: Span) -> IResult<Span, Token> {
    map(
        preceded(
            char('#'),
            take_while(|c: char| c != '\n' && c != '\r')
        ),
        |s: Span| Token::Comment(s.fragment().to_string())
    )(input)
}

/// Parse an integer literal
pub fn int_literal(input: Span) -> IResult<Span, Token> {
    map(
        recognize(pair(
            opt(char('-')),
            digit1
        )),
        |s: Span| {
            let num = s.fragment().parse::<i64>().unwrap_or(0);
            Token::IntLiteral(num)
        }
    )(input)
}

/// Parse a float literal
pub fn float_literal(input: Span) -> IResult<Span, Token> {
    map(
        recognize(tuple((
            opt(char('-')),
            digit1,
            char('.'),
            digit1,
            opt(tuple((
                alt((char('e'), char('E'))),
                opt(alt((char('+'), char('-')))),
                digit1
            )))
        ))),
        |s: Span| {
            let num = s.fragment().parse::<f64>().unwrap_or(0.0);
            Token::FloatLiteral(num)
        }
    )(input)
}

/// Parse a boolean literal
pub fn bool_literal(input: Span) -> IResult<Span, Token> {
    alt((
        value(Token::BoolLiteral(true), tag("true")),
        value(Token::BoolLiteral(false), tag("false"))
    ))(input)
}

/// Parse a string literal (single or double quoted)
pub fn string_literal(input: Span) -> IResult<Span, Token> {
    alt((
        map(
            delimited(
                char('\''),
                take_until("'"),
                char('\'')
            ),
            |s: Span| Token::StringLiteral(s.fragment().to_string())
        ),
        map(
            delimited(
                char('"'),
                take_until("\""),
                char('"')
            ),
            |s: Span| Token::StringLiteral(s.fragment().to_string())
        )
    ))(input)
}

/// Parse command placeholder tokens (from preprocessing)
pub fn command_placeholder(input: Span) -> IResult<Span, Token> {
    map(
        recognize(tuple((
            tag("__COMMAND_BLOCK_"),
            digit1,
            tag("__")
        ))),
        |s: Span| Token::CommandPlaceholder(s.fragment().to_string())
    )(input)
}

/// Parse an identifier or keyword
pub fn identifier_or_keyword(version: &str) -> impl Fn(Span) -> IResult<Span, Token> + '_ {
    move |input: Span| {
        let (input, start) = recognize(pair(
            alpha1,
            many0(alt((alphanumeric1, tag("_"))))
        ))(input)?;
        
        let word = start.fragment();
        
        if is_keyword(word, version) {
            Ok((input, Token::Keyword(word.to_string())))
        } else {
            Ok((input, Token::Identifier(word.to_string())))
        }
    }
}

/// Parse special command/heredoc tokens in normal mode
pub fn command_tokens(input: Span) -> IResult<Span, Token> {
    alt((
        value(Token::HeredocStart, tag("<<<")),
        value(Token::HeredocEnd, tag(">>>")),
        value(Token::TildeBrace, tag("~{")),
        value(Token::DollarBrace, tag("${")),
        // Note: ${ must come after ~{ due to ordering
    ))(input)
}

/// Parse text content in command mode (allows shell syntax like $( but recognizes ${)
pub fn command_mode_text(input: Span) -> IResult<Span, Token> {
    // This follows miniwdl's pattern:
    // COMMAND1_CHAR: /[^~$}]/ | /\$(?=[^{])/ | /~(?=[^{])/
    
    use nom::bytes::complete::take_while1;
    use nom::combinator::recognize;
    
    // Take characters that are safe in command mode
    let (input, text) = recognize(
        take_while1(|c: char| {
            match c {
                // Never consume these - they have special meaning
                '}' => false,
                
                // Don't consume ~ or $ here - they're handled separately
                '~' | '$' => false,
                
                // Allow everything else
                _ => true,
            }
        })
    )(input)?;
    
    Ok((input, Token::CommandText(text.fragment().to_string())))
}

/// Parse $ or ~ with lookahead in command mode
pub fn command_mode_special_chars(input: Span) -> IResult<Span, Token> {
    alt((
        // ${  -> DollarBrace token
        value(Token::DollarBrace, tag("${")),
        
        // ~{  -> TildeBrace token
        value(Token::TildeBrace, tag("~{")),
        
        // $ not followed by { -> part of shell syntax, treat as text
        map(
            recognize(tuple((
                char('$'),
                nom::combinator::not(char('{')), // negative lookahead
                nom::combinator::peek(nom::character::complete::anychar), // must have next char
            ))),
            |s: Span| Token::CommandText(s.fragment().chars().take(1).collect()) // just the $
        ),
        
        // ~ not followed by { -> regular text
        map(
            recognize(tuple((
                char('~'),
                nom::combinator::not(char('{')), // negative lookahead
                nom::combinator::peek(nom::character::complete::anychar), // must have next char
            ))),
            |s: Span| Token::CommandText(s.fragment().chars().take(1).collect()) // just the ~
        ),
    ))(input)
}

/// Parse a single token in command mode
pub fn command_mode_token(_version: &str) -> impl Fn(Span) -> IResult<Span, LocatedToken> + '_ {
    move |input: Span| {
        let pos = span_to_position(input);
        let (input, token) = alt((
            // Check for closing command delimiters first
            value(Token::RightBrace, char('}')),
            value(Token::HeredocEnd, tag(">>>")),
            
            // Handle special characters with lookahead
            command_mode_special_chars,
            
            // Handle regular text
            command_mode_text,
            
            // Handle whitespace and newlines (preserve in command mode)
            whitespace,
            newline,
            comment,
        ))(input)?;
        
        Ok((input, LocatedToken::new(token, pos)))
    }
}

/// Parse operators
pub fn operator(input: Span) -> IResult<Span, Token> {
    alt((
        // Two-character operators
        value(Token::Equal, tag("==")),
        value(Token::NotEqual, tag("!=")),
        value(Token::LessEqual, tag("<=")),
        value(Token::GreaterEqual, tag(">=")),
        value(Token::And, tag("&&")),
        value(Token::Or, tag("||")),
        value(Token::PlusQuestion, tag("+?")),
        
        // Single-character operators
        value(Token::Plus, char('+')),
        value(Token::Minus, char('-')),
        value(Token::Star, char('*')),
        value(Token::Slash, char('/')),
        value(Token::Percent, char('%')),
        value(Token::Less, char('<')),
        value(Token::Greater, char('>')),
        value(Token::Not, char('!')),
        value(Token::Assign, char('=')),
    ))(input)
}

/// Parse delimiters
pub fn delimiter(input: Span) -> IResult<Span, Token> {
    alt((
        value(Token::LeftParen, char('(')),
        value(Token::RightParen, char(')')),
        value(Token::LeftBracket, char('[')),
        value(Token::RightBracket, char(']')),
        value(Token::LeftBrace, char('{')),
        value(Token::RightBrace, char('}')),
    ))(input)
}

/// Parse punctuation
pub fn punctuation(input: Span) -> IResult<Span, Token> {
    alt((
        value(Token::Comma, char(',')),
        value(Token::Dot, char('.')),
        value(Token::Colon, char(':')),
        value(Token::Question, char('?')),
    ))(input)
}

/// Parse a single token based on lexer mode (stateful)
pub fn stateful_token(lexer: &Lexer) -> impl Fn(Span) -> IResult<Span, LocatedToken> + '_ {
    move |input: Span| {
        match lexer.current_mode() {
            LexerMode::Normal => normal_token(&lexer.version)(input),
            LexerMode::Command => command_mode_token(&lexer.version)(input),
            LexerMode::StringLiteral => {
                // TODO: Implement string literal mode if needed
                normal_token(&lexer.version)(input)
            }
            LexerMode::Placeholder => {
                // TODO: Implement placeholder mode if needed
                normal_token(&lexer.version)(input)
            }
        }
    }
}

/// Parse a single token in normal mode
pub fn normal_token(version: &str) -> impl Fn(Span) -> IResult<Span, LocatedToken> + '_ {
    move |input: Span| {
        let pos = span_to_position(input);
        let (input, token) = alt((
            command_placeholder,  // Must come before identifiers (has underscores)
            command_tokens,  // Must come before operators due to overlaps
            string_literal,  // Must come before other literals
            float_literal,  // Must come before int_literal
            int_literal,
            bool_literal,
            identifier_or_keyword(version),
            operator,
            delimiter,
            punctuation,
            whitespace,
            newline,
            comment,
        ))(input)?;
        
        Ok((input, LocatedToken::new(token, pos)))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_lexer_modes() {
        let mut lexer = Lexer::new("1.0");
        assert_eq!(lexer.current_mode(), LexerMode::Normal);
        
        lexer.push_mode(LexerMode::Command);
        assert_eq!(lexer.current_mode(), LexerMode::Command);
        assert!(lexer.preserve_whitespace());
        
        lexer.push_mode(LexerMode::Placeholder);
        assert_eq!(lexer.current_mode(), LexerMode::Placeholder);
        
        lexer.pop_mode();
        assert_eq!(lexer.current_mode(), LexerMode::Command);
        
        lexer.pop_mode();
        assert_eq!(lexer.current_mode(), LexerMode::Normal);
        assert!(!lexer.preserve_whitespace());
    }
    
    #[test]
    fn test_int_literal_parsing() {
        let input = Span::new("42");
        let result = int_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::IntLiteral(42));
        
        let input = Span::new("-123");
        let result = int_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::IntLiteral(-123));
    }
    
    #[test]
    fn test_float_literal_parsing() {
        let input = Span::new("3.14");
        let result = float_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::FloatLiteral(3.14));
        
        let input = Span::new("-2.5e10");
        let result = float_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::FloatLiteral(-2.5e10));
    }
    
    #[test]
    fn test_bool_literal_parsing() {
        let input = Span::new("true");
        let result = bool_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::BoolLiteral(true));
        
        let input = Span::new("false");
        let result = bool_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::BoolLiteral(false));
    }
    
    #[test]
    fn test_identifier_or_keyword() {
        let parser = identifier_or_keyword("1.0");
        
        let input = Span::new("task");
        let result = parser(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::Keyword("task".to_string()));
        
        let input = Span::new("my_variable");
        let result = parser(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::Identifier("my_variable".to_string()));
    }
    
    #[test]
    fn test_operators() {
        let test_cases = vec![
            ("==", Token::Equal),
            ("!=", Token::NotEqual),
            ("<=", Token::LessEqual),
            (">=", Token::GreaterEqual),
            ("&&", Token::And),
            ("||", Token::Or),
            ("+", Token::Plus),
            ("-", Token::Minus),
            ("*", Token::Star),
            ("/", Token::Slash),
            ("%", Token::Percent),
        ];
        
        for (input_str, expected) in test_cases {
            let input = Span::new(input_str);
            let result = operator(input);
            assert!(result.is_ok(), "Failed to parse operator: {}", input_str);
            let (_, token) = result.unwrap();
            assert_eq!(token, expected);
        }
    }
}