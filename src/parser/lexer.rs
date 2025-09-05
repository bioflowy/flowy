//! Stateful lexer for WDL parsing

use super::keywords::is_keyword;
use super::tokens::{LocatedToken, Token};
use crate::error::SourcePosition;
use nom::{
    branch::alt,
    bytes::complete::{tag, take_until, take_while, take_while1},
    character::complete::{alpha1, alphanumeric1, char, digit1, line_ending},
    combinator::{map, opt, peek, recognize, value},
    multi::many0,
    sequence::{delimited, pair, preceded, tuple},
    IResult,
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
    /// Inside string literal with the opening quote character
    StringLiteral(char),
}

/// Stateful lexer for WDL
#[derive(Debug, Clone)]
pub struct Lexer {
    mode_stack: Vec<LexerMode>,
    #[allow(dead_code)]
    version: String,
    filename: String,
}

impl Lexer {
    /// Create a new lexer for the specified WDL version
    pub fn new(version: &str) -> Self {
        Self {
            mode_stack: vec![LexerMode::Normal],
            version: version.to_string(),
            filename: String::new(),
        }
    }

    /// Set the filename for better error reporting
    pub fn set_filename(&mut self, filename: &str) {
        self.filename = filename.to_string();
    }

    /// Get the filename
    pub fn get_filename(&self) -> &str {
        &self.filename
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
        matches!(
            self.current_mode(),
            LexerMode::Command | LexerMode::StringLiteral(_)
        )
    }
}

/// Convert a span to a source position with filename and offset
pub fn span_to_position_with_offset(
    span: Span,
    filename: &str,
    source_offset: usize,
) -> SourcePosition {
    // Calculate actual line and column considering the source offset
    let line = span.location_line() as usize;
    let column = span.get_utf8_column();

    // For now, we'll calculate line/column from the beginning, but this could be improved
    // to use the source_offset to calculate the actual position in the original file
    SourcePosition::new(
        filename.to_string(),
        filename.to_string(),
        line as u32,
        column as u32,
        line as u32,
        (column + span.fragment().len()) as u32,
    )
}

/// Convert a span to a source position with filename (legacy version)
pub fn span_to_position(span: Span, filename: &str) -> SourcePosition {
    span_to_position_with_offset(span, filename, 0)
}

// Basic token parsers

/// Parse whitespace
pub fn whitespace(input: Span) -> IResult<Span, Token> {
    map(take_while1(|c: char| c == ' ' || c == '\t'), |s: Span| {
        Token::Whitespace(s.fragment().to_string())
    })(input)
}

/// Parse newline
pub fn newline(input: Span) -> IResult<Span, Token> {
    map(line_ending, |_| Token::Newline)(input)
}

/// Parse a comment
pub fn comment(input: Span) -> IResult<Span, Token> {
    map(
        preceded(char('#'), take_while(|c: char| c != '\n' && c != '\r')),
        |s: Span| Token::Comment(s.fragment().to_string()),
    )(input)
}

/// Parse an integer literal (positive only, minus handled as separate token)
pub fn int_literal(input: Span) -> IResult<Span, Token> {
    map(recognize(digit1), |s: Span| {
        let num = s.fragment().parse::<i64>().unwrap_or(0);
        Token::IntLiteral(num)
    })(input)
}

/// Parse a float literal (positive only, minus handled as separate token)
pub fn float_literal(input: Span) -> IResult<Span, Token> {
    alt((
        // Standard format: digits.digits[exponent]
        map(
            recognize(tuple((
                digit1,
                char('.'),
                digit1,
                opt(tuple((
                    alt((char('e'), char('E'))),
                    opt(alt((char('+'), char('-')))),
                    digit1,
                ))),
            ))),
            |s: Span| {
                let num = s.fragment().parse::<f64>().unwrap_or(0.0);
                Token::FloatLiteral(num)
            },
        ),
        // Decimal-only format: .digits[exponent]
        map(
            recognize(tuple((
                char('.'),
                digit1,
                opt(tuple((
                    alt((char('e'), char('E'))),
                    opt(alt((char('+'), char('-')))),
                    digit1,
                ))),
            ))),
            |s: Span| {
                let num = s.fragment().parse::<f64>().unwrap_or(0.0);
                Token::FloatLiteral(num)
            },
        ),
    ))(input)
}

/// Parse a boolean literal
pub fn bool_literal(input: Span) -> IResult<Span, Token> {
    alt((
        value(Token::BoolLiteral(true), tag("true")),
        value(Token::BoolLiteral(false), tag("false")),
    ))(input)
}

/// Parse a string literal (single or double quoted)
pub fn string_literal(input: Span) -> IResult<Span, Token> {
    alt((
        map(
            delimited(char('\''), take_until("'"), char('\'')),
            |s: Span| Token::StringLiteral(s.fragment().to_string()),
        ),
        map(
            delimited(char('"'), take_until("\""), char('"')),
            |s: Span| Token::StringLiteral(s.fragment().to_string()),
        ),
    ))(input)
}

/// Parse command placeholder tokens (from preprocessing)
pub fn command_placeholder(input: Span) -> IResult<Span, Token> {
    map(
        recognize(tuple((tag("__COMMAND_BLOCK_"), digit1, tag("__")))),
        |s: Span| Token::CommandPlaceholder(s.fragment().to_string()),
    )(input)
}

/// Parse an identifier or keyword
pub fn identifier_or_keyword(version: &str) -> impl Fn(Span) -> IResult<Span, Token> + '_ {
    move |input: Span| {
        let (input, start) = recognize(pair(alpha1, many0(alt((alphanumeric1, tag("_"))))))(input)?;

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
    ))(input)
}

/// Parse text content in command mode (allows shell syntax like $( but recognizes ${)
pub fn command_mode_text(input: Span) -> IResult<Span, Token> {
    // This follows miniwdl's pattern:
    // COMMAND1_CHAR: /[^~$}]/ | /\$(?=[^{])/ | /~(?=[^{])/

    use nom::bytes::complete::take_while1;
    use nom::combinator::recognize;

    // Take characters that are safe in command mode
    let (input, text) = recognize(take_while1(|c: char| {
        match c {
            // Never consume these - they have special meaning
            '}' => false,

            // Don't consume ~ or $ here - they're handled separately
            '~' | '$' => false,

            // Don't consume > if it might be part of >>>
            '>' => false,

            // Allow everything else
            _ => true,
        }
    }))(input)?;

    Ok((input, Token::CommandText(text.fragment().to_string())))
}

/// Parse $ or ~ or > with lookahead in command mode
pub fn command_mode_special_chars(input: Span) -> IResult<Span, Token> {
    alt((
        // ~{  -> TildeBrace token
        value(Token::TildeBrace, tag("~{")),
        // $ -> part of shell syntax, treat as text (including ${})
        map(char('$'), |_| Token::CommandText("$".to_string())),
        // ~ not followed by { -> regular text
        map(
            recognize(tuple((
                char('~'),
                nom::combinator::not(char('{')), // negative lookahead
                nom::combinator::peek(nom::character::complete::anychar), // must have next char
            ))),
            |s: Span| Token::CommandText(s.fragment().chars().take(1).collect()), // just the ~
        ),
        // > not part of >>> -> regular text (shell redirection)
        map(
            recognize(tuple((
                char('>'),
                nom::combinator::not(tag(">>")), // negative lookahead for >>>
            ))),
            |s: Span| Token::CommandText(s.fragment().chars().take(1).collect()), // just the >
        ),
    ))(input)
}

/// Parse a single token in command mode
pub fn command_mode_token(_version: &str) -> impl Fn(Span) -> IResult<Span, LocatedToken> + '_ {
    move |input: Span| {
        let pos = span_to_position(input, "");
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

/// Parse a single token in command mode with filename information
pub fn command_mode_token_with_filename<'a>(
    _version: &'a str,
    filename: &'a str,
) -> impl Fn(Span) -> IResult<Span, LocatedToken> + 'a {
    move |input: Span| {
        let pos = span_to_position(input, filename);
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

/// Parse a single token in string literal mode with filename information
pub fn string_literal_mode_token_with_filename<'a>(
    _version: &'a str,
    filename: &'a str,
    quote_char: char,
) -> impl Fn(Span) -> IResult<Span, LocatedToken> + 'a {
    move |input: Span| {
        let pos = span_to_position(input, filename);
        let (input, token) = alt((
            // Check for placeholder start sequences
            value(Token::TildeBrace, tag("~{")),
            value(Token::DollarBrace, tag("${")),
            // Check for string end quote - only the matching quote character
            map(char(quote_char), Token::StringEnd),
            // Handle string text content with quote-aware parsing
            move |input| string_literal_text_with_quote(input, quote_char),
        ))(input)?;

        Ok((input, LocatedToken::new(token, pos)))
    }
}

/// Parse string literal text content (legacy function)
pub fn string_literal_text(input: Span) -> IResult<Span, Token> {
    string_literal_text_with_quote(input, '"') // Default to double quote
}

/// Parse string literal text content with quote character awareness
/// This function handles content within string literals, including escape sequences
pub fn string_literal_text_with_quote(input: Span, quote_char: char) -> IResult<Span, Token> {
    let mut result = String::new();
    let mut remaining = input;

    while !remaining.fragment().is_empty() {
        let c = remaining.fragment().chars().next().unwrap();

        match c {
            // Stop at placeholder start sequences
            '~' | '$'
                if remaining.fragment().starts_with("~{")
                    || remaining.fragment().starts_with("${") =>
            {
                break;
            }
            // Stop only at the matching quote character
            c if c == quote_char => {
                break;
            }
            // Handle escape sequences
            '\\' if remaining.fragment().len() > 1 => {
                let next_char = remaining.fragment().chars().nth(1).unwrap();
                match next_char {
                    'n' => result.push('\n'),
                    't' => result.push('\t'),
                    'r' => result.push('\r'),
                    '\\' => result.push('\\'),
                    '\'' => result.push('\''),
                    '"' => result.push('"'),
                    _ => {
                        // Unknown escape sequence - keep as is
                        result.push('\\');
                        result.push(next_char);
                    }
                }
                // Skip both the backslash and the escaped character
                remaining = Span::new(&remaining.fragment()[2..]);
            }
            // Regular character
            _ => {
                result.push(c);
                remaining = Span::new(&remaining.fragment()[1..]);
            }
        }
    }

    if result.is_empty() && input == remaining {
        // No content was consumed
        Err(nom::Err::Error(nom::error::Error::new(
            input,
            nom::error::ErrorKind::TakeWhile1,
        )))
    } else {
        Ok((remaining, Token::StringText(result)))
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
    move |input: Span| match lexer.current_mode() {
        LexerMode::Normal => normal_token_with_filename(&lexer.version, &lexer.filename)(input),
        LexerMode::Command => {
            command_mode_token_with_filename(&lexer.version, &lexer.filename)(input)
        }
        LexerMode::StringLiteral(quote_char) => {
            string_literal_mode_token_with_filename(&lexer.version, &lexer.filename, quote_char)(
                input,
            )
        }
    }
}

/// Parse a single token in normal mode
pub fn normal_token(version: &str) -> impl Fn(Span) -> IResult<Span, LocatedToken> + '_ {
    move |input: Span| {
        let pos = span_to_position(input, "");
        let (input, token) = alt((
            command_placeholder, // Must come before identifiers (has underscores)
            command_tokens,      // Must come before operators due to overlaps
            // Handle string start quotes - removed old string_literal to use new approach
            value(Token::SingleQuote, char('\'')),
            value(Token::DoubleQuote, char('"')),
            float_literal, // Must come before int_literal
            int_literal,
            identifier_or_keyword(version), // Must come before bool_literal to handle identifiers starting with keywords
            bool_literal,                   // Moved after identifier_or_keyword
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

/// Parse a single token in normal mode with filename information
pub fn normal_token_with_filename<'a>(
    version: &'a str,
    filename: &'a str,
) -> impl Fn(Span) -> IResult<Span, LocatedToken> + 'a {
    move |input: Span| {
        let pos = span_to_position(input, filename);
        let (input, token) = alt((
            command_placeholder, // Must come before identifiers (has underscores)
            command_tokens,      // Must come before operators due to overlaps
            // Handle string start quotes - removed old string_literal to use new approach
            value(Token::SingleQuote, char('\'')),
            value(Token::DoubleQuote, char('"')),
            float_literal, // Must come before int_literal
            int_literal,
            identifier_or_keyword(version), // Must come before bool_literal to handle identifiers starting with keywords
            bool_literal,                   // Moved after identifier_or_keyword
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

        lexer.push_mode(LexerMode::StringLiteral('"'));
        assert_eq!(lexer.current_mode(), LexerMode::StringLiteral('"'));

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

        // Negative numbers should NOT be parsed by the lexer anymore
        // They are handled as Minus + IntLiteral by the expression parser
        let input = Span::new("-123");
        let result = int_literal(input);
        assert!(
            result.is_err(),
            "Lexer should not parse negative integers directly"
        );
    }

    #[test]
    fn test_float_literal_parsing() {
        let input = Span::new("3.11");
        let result = float_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::FloatLiteral(3.11));

        // Positive float with exponent
        let input = Span::new("2.5e10");
        let result = float_literal(input);
        assert!(result.is_ok());
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::FloatLiteral(2.5e10));

        // Float starting with decimal point (e.g., .14)
        let input = Span::new(".14");
        let result = float_literal(input);
        assert!(result.is_ok(), "Should parse .14 as float literal");
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::FloatLiteral(0.14));

        // Float starting with decimal point with exponent (e.g., .5e10)
        let input = Span::new(".5e10");
        let result = float_literal(input);
        assert!(result.is_ok(), "Should parse .5e10 as float literal");
        let (_, token) = result.unwrap();
        assert_eq!(token, Token::FloatLiteral(0.5e10));

        // Negative numbers should fail (handled by parser as unary minus)
        let input = Span::new("-2.5e10");
        let result = float_literal(input);
        assert!(result.is_err()); // This should fail as expected
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
    fn test_identifier_starting_with_keyword() {
        let parser = identifier_or_keyword("1.2");

        // Test identifier starting with keyword "true"
        let input = Span::new("true_false_ternary");
        let result = parser(input);
        assert!(result.is_ok());
        let (remaining, token) = result.unwrap();
        // This should parse "true_false_ternary" as a single identifier, not just "true"
        assert_eq!(token, Token::Identifier("true_false_ternary".to_string()));
        assert_eq!(remaining.fragment().to_owned(), "");

        // Test another identifier starting with keyword "if"
        let input = Span::new("if_condition");
        let result = parser(input);
        assert!(result.is_ok());
        let (remaining, token) = result.unwrap();
        assert_eq!(token, Token::Identifier("if_condition".to_string()));
        assert_eq!(remaining.fragment().to_owned(), "");
    }

    #[test]
    fn test_normal_token_with_keyword_prefix() {
        let parser = normal_token("1.2");

        // Test that "true_false_ternary" should be tokenized as a single identifier
        let input = Span::new("true_false_ternary");
        let result = parser(input);
        assert!(result.is_ok());
        let (remaining, located_token) = result.unwrap();
        // The problem: this currently returns Token::BoolLiteral(true) instead of Token::Identifier
        assert_eq!(
            located_token.token,
            Token::Identifier("true_false_ternary".to_string())
        );
        assert_eq!(remaining.fragment().to_owned(), "");
    }

    #[test]
    fn test_tokenization_of_task_with_true_prefix() {
        use super::super::token_stream::tokenize;

        // Test tokenizing "task true_false_ternary {"
        let input = "task true_false_ternary {";
        let result = tokenize(input, "1.2");
        assert!(result.is_ok());
        let tokens = result.unwrap();

        // Should have 3 tokens: task (keyword), true_false_ternary (identifier), { (left brace)
        assert_eq!(tokens.len(), 3);
        assert_eq!(tokens[0].token, Token::Keyword("task".to_string()));
        assert_eq!(
            tokens[1].token,
            Token::Identifier("true_false_ternary".to_string())
        );
        assert_eq!(tokens[2].token, Token::LeftBrace);
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
