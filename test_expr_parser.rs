use miniwdl_rust::parser::expressions::*;
use miniwdl_rust::parser::literals::*;
use miniwdl_rust::parser::lexer::Span;

fn main() {
    // Test parsing a simple literal
    let input = Span::new("1");
    println!("Parsing '1'...");
    let result = parse_literal(input);
    match result {
        Ok((rest, expr)) => {
            println!("  Success! Rest: '{}'", rest.fragment());
            println!("  Expression: {:?}", expr);
        }
        Err(e) => {
            println!("  Error: {:?}", e);
        }
    }
    
    // Test parsing an identifier
    let input = Span::new("x");
    println!("\nParsing 'x'...");
    let result = parse_identifier(input);
    match result {
        Ok((rest, expr)) => {
            println!("  Success! Rest: '{}'", rest.fragment());
            println!("  Expression: {:?}", expr);
        }
        Err(e) => {
            println!("  Error: {:?}", e);
        }
    }
    
    // Test parsing primary expression
    let input = Span::new("1");
    println!("\nParsing '1' as primary...");
    let result = parse_primary_expr(input);
    match result {
        Ok((rest, expr)) => {
            println!("  Success! Rest: '{}'", rest.fragment());
            println!("  Expression: {:?}", expr);
        }
        Err(e) => {
            println!("  Error: {:?}", e);
        }
    }
    
    // Test parsing "1 + 2"
    let input = Span::new("1 + 2");
    println!("\nParsing '1 + 2'...");
    let result = parse_expression(input);
    match result {
        Ok((rest, expr)) => {
            println!("  Success! Rest: '{}'", rest.fragment());
            println!("  Expression: {:?}", expr);
        }
        Err(e) => {
            println!("  Error: {:?}", e);
        }
    }
}