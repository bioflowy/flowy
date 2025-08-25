# miniwdl Rust Port Guidelines

## Overview
This document outlines the process for porting miniwdl from Python to Rust. The approach is to systematically port modules one at a time, starting with foundational modules and building up the dependency chain.

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

## Module Porting Order

1. **Error & SourcePosition** (Foundation - no dependencies)
2. **Environment (Env)** (Variable bindings - depends on Error)
3. **Type** (Type system - depends on Error) 
4. **Value** (Runtime values - depends on Type, Error)
5. **Expression (Expr)** (Expressions - depends on all above)
6. **AST Tree** (Document/workflow AST - depends on all above)
7. **Parser** (Convert from Lark to Rust parser)

## Quality Standards
- Use idiomatic Rust patterns
- Provide comprehensive error messages
- Include documentation comments for public APIs
- Follow Rust naming conventions
- Use appropriate error handling (Result types)

## Progress Tracking
Use TodoWrite tool to track progress on each module and maintain visibility into the porting process.

## Completed Modules ✅

1. **Error & SourcePosition** (593 lines) - Error handling and source position tracking
2. **Environment (Env)** (525 lines) - Variable bindings and namespaces  
3. **Type** (810 lines) - WDL type system with coercion and unification
4. **Value** (750 lines) - Runtime values and JSON conversion
5. **Expression (Expr)** (1,246 lines → 7 files) - Expression AST, evaluation, and type inference
   - Successfully refactored into modular structure for better maintainability

6. **Tree (AST)** (1,107 lines → 8 files) - WDL document AST with tasks, workflows, and control flow
   - Modular design with visitor pattern and trait-based architecture

## Current Phase: Parser Module 🎯  
Next target: WDL Parser implementation - Convert from Python Lark-based parser to Rust.

### Tree Module Structure Analysis

**Main AST Node Classes:**
- `StructTypeDef` - Struct type definitions
- `WorkflowNode` (abstract) - Base class for workflow nodes
- `Decl` - Value declarations  
- `Task` - Task definitions
- `Call` - Task/workflow calls
- `Gather` - Array gathering operations (scatter/conditional)
- `WorkflowSection` (abstract) - Base for scatter/conditional sections  
- `Scatter` - Scatter sections for parallel execution
- `Conditional` - Conditional sections
- `Workflow` - Workflow definitions
- `Document` - Complete WDL document

### Tree Module Porting Strategy

Due to the size and complexity (2,122 lines), the Tree module will follow the same modular approach used for Expression:

1. **tree/mod.rs** - Core AST definitions and traits
2. **tree/document.rs** - Document and top-level structures
3. **tree/workflow.rs** - Workflow definitions and nodes  
4. **tree/task.rs** - Task definitions
5. **tree/declarations.rs** - Declarations and bindings
6. **tree/control_flow.rs** - Scatter and conditional sections
7. **tree/validation.rs** - AST validation and type checking
8. **tree/traversal.rs** - AST traversal utilities