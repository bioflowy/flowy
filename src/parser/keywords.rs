//! WDL version-specific keywords

use std::collections::HashSet;

/// Get the set of keywords for a specific WDL version
pub fn keywords_for_version(version: &str) -> HashSet<String> {
    let mut keywords = HashSet::new();

    // Draft-2 keywords (base set)
    let draft2_keywords = vec![
        "Array",
        "Boolean",
        "File",
        "Float",
        "Int",
        "Map",
        "None",
        "Object",
        "Pair",
        "String",
        "as",
        "call",
        "command",
        "else",
        "false",
        "if",
        "import",
        "input",
        "left",
        "meta",
        "object",
        "output",
        "parameter_meta",
        "right",
        "runtime",
        "scatter",
        "task",
        "then",
        "true",
        "version",
        "workflow",
    ];

    for kw in draft2_keywords {
        keywords.insert(kw.to_string());
    }

    // Additional keywords for WDL 1.0+
    if version == "1.0" || version.starts_with("1.") || version == "development" {
        keywords.insert("alias".to_string());
        keywords.insert("struct".to_string());
    }

    // Additional keywords for WDL 1.2+
    if version == "1.2" || version == "development" {
        keywords.insert("Directory".to_string());
        keywords.insert("env".to_string());
        keywords.insert("requirements".to_string());
        keywords.insert("hints".to_string());
    }

    keywords
}

/// Check if a string is a keyword in the given WDL version
pub fn is_keyword(word: &str, version: &str) -> bool {
    keywords_for_version(version).contains(word)
}

/// Check if a string is a valid identifier (not a keyword)
pub fn is_valid_identifier(word: &str, version: &str) -> bool {
    // Must start with a letter, followed by letters, digits, or underscores
    if word.is_empty() {
        return false;
    }

    let first_char = word.chars().next().unwrap();
    if !first_char.is_alphabetic() {
        return false;
    }

    if !word.chars().all(|c| c.is_alphanumeric() || c == '_') {
        return false;
    }

    // Must not be a keyword
    !is_keyword(word, version)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_draft2_keywords() {
        let keywords = keywords_for_version("draft-2");
        assert!(keywords.contains("task"));
        assert!(keywords.contains("workflow"));
        assert!(keywords.contains("String"));
        assert!(!keywords.contains("struct")); // Not in draft-2
    }

    #[test]
    fn test_wdl_1_0_keywords() {
        let keywords = keywords_for_version("1.0");
        assert!(keywords.contains("task"));
        assert!(keywords.contains("struct"));
        assert!(keywords.contains("alias"));
        assert!(!keywords.contains("Directory")); // Not in 1.0
    }

    #[test]
    fn test_wdl_1_2_keywords() {
        let keywords = keywords_for_version("1.2");
        assert!(keywords.contains("task"));
        assert!(keywords.contains("struct"));
        assert!(keywords.contains("Directory"));
        assert!(keywords.contains("env"));
        assert!(keywords.contains("requirements"));
        assert!(keywords.contains("hints"));
    }

    #[test]
    fn test_is_keyword() {
        assert!(is_keyword("task", "1.0"));
        assert!(is_keyword("struct", "1.0"));
        assert!(!is_keyword("struct", "draft-2"));
        assert!(!is_keyword("my_task", "1.0"));
    }

    #[test]
    fn test_is_valid_identifier() {
        assert!(is_valid_identifier("my_task", "1.0"));
        assert!(is_valid_identifier("foo123", "1.0"));
        assert!(is_valid_identifier("foo_bar", "1.0"));

        assert!(!is_valid_identifier("_private", "1.0")); // WDL doesn't allow leading underscore
        assert!(!is_valid_identifier("task", "1.0")); // Keyword
        assert!(!is_valid_identifier("123foo", "1.0")); // Starts with digit
        assert!(!is_valid_identifier("foo-bar", "1.0")); // Contains hyphen
        assert!(!is_valid_identifier("", "1.0")); // Empty
    }
}
