//! Pre-processor for extracting command blocks before tokenization
//!
//! This solves the fundamental issue where shell syntax in command blocks
//! breaks the normal WDL tokenizer. We extract commands first, then
//! tokenize the rest normally.

use crate::error::WdlError;
use std::collections::HashMap;

#[derive(Debug, Clone)]
pub struct CommandBlock {
    pub content: String,
    pub is_heredoc: bool, // true for <<<>>>, false for {}
    pub start_pos: usize,
    pub end_pos: usize,
}

#[derive(Debug, Clone)]
pub struct PreprocessorResult {
    pub processed_source: String,
    pub command_blocks: HashMap<String, CommandBlock>, // placeholder_id -> CommandBlock
}

/// Pre-process source to extract command blocks
pub fn preprocess_commands(source: &str) -> Result<PreprocessorResult, WdlError> {
    let mut processed_source = String::new();
    let mut command_blocks = HashMap::new();
    let mut command_counter = 0;

    // Simple regex-based approach to find command blocks
    let mut current_pos = 0;

    while current_pos < source.len() {
        // Look for "command" keyword
        if let Some(cmd_start) = source[current_pos..].find("command") {
            let abs_cmd_start = current_pos + cmd_start;

            // Check if this is actually the keyword (not part of another identifier)
            let before_ok = abs_cmd_start == 0
                || !source
                    .chars()
                    .nth(abs_cmd_start - 1)
                    .unwrap_or(' ')
                    .is_alphanumeric();
            let after_idx = abs_cmd_start + 7; // "command".len()
            let after_ok = after_idx >= source.len()
                || !source
                    .chars()
                    .nth(after_idx)
                    .unwrap_or(' ')
                    .is_alphanumeric();

            if before_ok && after_ok {
                // Add everything up to "command"
                processed_source.push_str(&source[current_pos..abs_cmd_start]);
                processed_source.push_str("command");

                // Skip whitespace after "command"
                let mut scan_pos = after_idx;
                while scan_pos < source.len()
                    && source.chars().nth(scan_pos).unwrap().is_whitespace()
                {
                    processed_source.push(source.chars().nth(scan_pos).unwrap());
                    scan_pos += 1;
                }

                // Check for { or <<<
                if scan_pos < source.len() {
                    let remaining = &source[scan_pos..];
                    if remaining.starts_with('{') {
                        // Extract brace command block
                        if let Some(end_pos) = find_matching_brace(remaining) {
                            let content = &remaining[1..end_pos]; // Remove braces
                            let block = CommandBlock {
                                content: content.to_string(),
                                is_heredoc: false,
                                start_pos: scan_pos,
                                end_pos: scan_pos + end_pos + 1,
                            };

                            let placeholder_id = format!("__COMMAND_BLOCK_{}__", command_counter);
                            command_counter += 1;

                            processed_source.push_str(&placeholder_id);
                            command_blocks.insert(placeholder_id, block);

                            current_pos = scan_pos + end_pos + 1; // Skip past }
                        } else {
                            return Err(WdlError::RuntimeError {
                                message: "Unclosed command block".to_string(),
                            });
                        }
                    } else if remaining.starts_with("<<<") {
                        // Extract heredoc command block
                        if let Some(end_pos) = remaining.find(">>>") {
                            let content = &remaining[3..end_pos]; // Remove <<<
                            let block = CommandBlock {
                                content: content.to_string(),
                                is_heredoc: true,
                                start_pos: scan_pos,
                                end_pos: scan_pos + end_pos + 3,
                            };

                            let placeholder_id = format!("__COMMAND_BLOCK_{}__", command_counter);
                            command_counter += 1;

                            processed_source.push_str(&placeholder_id);
                            command_blocks.insert(placeholder_id, block);

                            current_pos = scan_pos + end_pos + 3; // Skip past >>>
                        } else {
                            return Err(WdlError::RuntimeError {
                                message: "Unclosed heredoc command block".to_string(),
                            });
                        }
                    } else {
                        // Not a command block, just continue
                        processed_source.push(source.chars().nth(scan_pos).unwrap());
                        current_pos = scan_pos + 1;
                    }
                } else {
                    // End of input
                    break;
                }
            } else {
                // Not the command keyword, add the character and continue
                processed_source.push_str(&source[current_pos..abs_cmd_start + 1]);
                current_pos = abs_cmd_start + 1;
            }
        } else {
            // No more "command" keywords, add the rest
            processed_source.push_str(&source[current_pos..]);
            break;
        }
    }

    Ok(PreprocessorResult {
        processed_source,
        command_blocks,
    })
}

/// Find the matching closing brace, handling nesting
fn find_matching_brace(source: &str) -> Option<usize> {
    let mut depth = 0;

    for (i, ch) in source.char_indices() {
        match ch {
            '{' => depth += 1,
            '}' => {
                depth -= 1;
                if depth == 0 {
                    return Some(i);
                }
            }
            _ => {}
        }
    }

    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_brace_command_extraction() {
        let source = r#"
        task test {
            command {
                echo $(( ~{x} + ~{y} ))
            }
        }
        "#;

        let result = preprocess_commands(source).unwrap();
        assert!(!result.command_blocks.is_empty());

        let block = result.command_blocks.values().next().unwrap();
        assert!(!block.is_heredoc);
        assert!(block.content.contains("echo $(( ~{x} + ~{y} ))"));
    }

    #[test]
    fn test_heredoc_command_extraction() {
        let source = r#"
        task test {
            command <<<
                echo $(( ~{x} + ~{y} ))
            >>>
        }
        "#;

        let result = preprocess_commands(source).unwrap();
        assert!(!result.command_blocks.is_empty());

        let block = result.command_blocks.values().next().unwrap();
        assert!(block.is_heredoc);
        assert!(block.content.contains("echo $(( ~{x} + ~{y} ))"));
    }
}
