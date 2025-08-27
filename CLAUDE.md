# miniwdl Rust Port Guidelines

## Overview
This document outlined the process for porting miniwdl from Python to Rust. The systematic approach of porting modules one at a time, starting with foundational modules and building up the dependency chain, has been successfully completed.

**Status**: The miniwdl Rust port is now feature complete with 21,174+ lines of code across 56 Rust files, including a fully functional CLI executable.

## Porting Process Rules

### 1. One Module at a Time
- Complete each module fully before starting the next
- Follow the dependency order: Error → Env → Type → Value → Expr → Tree
- Get user approval before moving to the next module

### 2. Test-First Approach  
- Write comprehensive tests for each module
- All tests must pass before module is considered complete
- Create both unit tests and integration tests where applicable

### 3. Error Handling Protocol
- **CRITICAL**: When tests fail, ALWAYS ask user for confirmation on the cause and fix before making any changes
- Do not assume the cause of test failures
- Present the error, proposed diagnosis, and suggested fix to the user for approval
- **NOTE**: Compilation errors can be fixed directly without user confirmation - only test failures require confirmation

### 4. Testing Commands
- Never assume specific test frameworks or commands
- Check README or search codebase to determine the correct testing approach  
- Ask user for test commands if not found in documentation

### 5. Module Completion Criteria
Each module must meet these requirements before proceeding:
- ✅ All tests pass
- ✅ Documentation complete
- ✅ API matches Python equivalent functionality
- ✅ User approval received

## Module Porting Order (Completed)

1. **Error & SourcePosition** ✅ (Foundation - no dependencies)
2. **Environment (Env)** ✅ (Variable bindings - depends on Error)
3. **Type** ✅ (Type system - depends on Error) 
4. **Value** ✅ (Runtime values - depends on Type, Error)
5. **Expression (Expr)** ✅ (Expressions - depends on all above)
6. **AST Tree** ✅ (Document/workflow AST - depends on all above)
7. **Parser** ✅ (nom-based parser replacing Python Lark parser)
8. **Runtime** ✅ (Task and workflow execution engine)
9. **Standard Library** ✅ (Built-in functions and operators)
10. **CLI Executable** ✅ (Complete command-line interface)

## Quality Standards
- Use idiomatic Rust patterns
- Provide comprehensive error messages
- Include documentation comments for public APIs
- Follow Rust naming conventions
- Use appropriate error handling (Result types)

## Future Development Areas

With the core porting complete, potential areas for continued development include:

- **Performance optimization** - Profile and optimize hot paths in parser and runtime
- **Extended WDL support** - Additional WDL specification features
- **Integration testing** - More comprehensive end-to-end workflow tests  
- **Documentation** - API documentation and user guides
- **Ecosystem integration** - IDE plugins, package managers, etc.

## Completed Modules ✅

1. **Error & SourcePosition** (638 lines) - Error handling and source position tracking
2. **Environment (Env)** (528 lines) - Variable bindings and namespaces  
3. **Type** (937 lines) - WDL type system with coercion and unification
4. **Value** (780 lines) - Runtime values and JSON conversion
5. **Expression (Expr)** (2,701 lines → 7 files) - Expression AST, evaluation, and type inference
   - Successfully refactored into modular structure for better maintainability

6. **Tree (AST)** (2,193 lines → 8 files) - WDL document AST with tasks, workflows, and control flow
   - Modular design with visitor pattern and trait-based architecture

7. **Parser** (5,509 lines → 15 files) - Complete WDL parser implementation using nom combinators
   - Lexer with location tracking and mode-based tokenization
   - Expression parsing with precedence handling
   - Statement and declaration parsing
   - Task and workflow parsing with command preprocessing
   - Comprehensive test coverage

8. **Runtime** (5,726 lines → 9 files) - Task and workflow execution engine
   - Task execution with Docker integration
   - Workflow orchestration with scatter/conditional support
   - File system utilities and configuration management
   - Comprehensive integration tests

9. **Standard Library** (1,733 lines → 8 files) - WDL built-in functions and operators
   - Array operations, math functions, string manipulation
   - I/O operations and type utilities
   - Task output handling

## Current Status: Feature Complete Implementation 🎉
The miniwdl Rust port now includes all core functionality with a working CLI executable.

## Module Structure Documentation

### Parser Module Structure (15 files, 5,509 lines)

1. **parser/mod.rs** - Main parser entry point and module exports
2. **parser/lexer.rs** - Tokenization with location tracking and mode support
3. **parser/tokens.rs** - Token type definitions and classification
4. **parser/token_stream.rs** - Token stream management with backtracking
5. **parser/parser_utils.rs** - Common parsing utilities and combinators
6. **parser/keywords.rs** - WDL keyword management by version
7. **parser/literals.rs** - Literal value parsing (strings, numbers, arrays)
8. **parser/types.rs** - Type specification parsing (primitives, collections)
9. **parser/expressions.rs** - Expression parsing with precedence
10. **parser/declarations.rs** - Variable and parameter declarations
11. **parser/statements.rs** - Control flow statements (scatter, conditional, calls)
12. **parser/tasks.rs** - Task and workflow definition parsing
13. **parser/document.rs** - Document structure and import parsing
14. **parser/command_preprocessor.rs** - Command template preprocessing
15. **parser/command_parser.rs** - Command section parsing

### Runtime Module Structure (9 files, 5,726 lines)

1. **runtime/mod.rs** - Runtime module entry point
2. **runtime/error.rs** - Runtime-specific error types
3. **runtime/config.rs** - Execution configuration management
4. **runtime/task_context.rs** - Task execution context
5. **runtime/task.rs** - Task execution engine
6. **runtime/workflow.rs** - Workflow orchestration engine
7. **runtime/fs_utils.rs** - File system utilities and operations
8. **runtime/task_tests.rs** - Task execution tests
9. **runtime/workflow_tests.rs** - Comprehensive workflow integration tests

### Standard Library Structure (8 files, 1,733 lines)

1. **stdlib/mod.rs** - Standard library module entry point
2. **stdlib/arrays.rs** - Array manipulation functions
3. **stdlib/math.rs** - Mathematical functions and operations  
4. **stdlib/strings.rs** - String manipulation utilities
5. **stdlib/io.rs** - Input/output operations
6. **stdlib/operators.rs** - Arithmetic and logical operators
7. **stdlib/types.rs** - Type conversion and validation utilities
8. **stdlib/task_output.rs** - Task output handling functions

## CLI Executable Functionality

The project includes a fully functional CLI executable (`src/main.rs`) with:

- **WDL file parsing and validation**
- **JSON input/output handling** 
- **Workflow execution with configurable options**
- **Task execution with Docker support**
- **Comprehensive error reporting**
- **Integration with all core modules**