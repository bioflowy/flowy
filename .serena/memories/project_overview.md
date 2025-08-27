# Miniwdl Rust Port Project Overview

## Project Purpose
This project is a Rust port of miniwdl, a Workflow Description Language (WDL) parser and runtime. It's being systematically ported from Python to Rust following a modular approach.

## Tech Stack
- **Language**: Rust (edition 2021, MSRV 1.78.0)
- **Parser**: Using nom parser combinator library (transitioning from Python's Lark)
- **Dependencies**: 
  - thiserror: Error handling
  - serde/serde_json: Serialization
  - once_cell: Lazy static initialization
  - nom/nom_locate: Parser combinators with location tracking
  - regex: Regular expressions
  - chrono: Date/time handling
  - tempfile (dev): Testing utilities

## Current Status
The project is in the middle of porting from Python miniwdl. Completed modules:
- ✅ Error & SourcePosition (Foundation)
- ✅ Environment (Env) 
- ✅ Type system
- ✅ Value system
- ✅ Expression (Expr) - 7 files modular structure
- ✅ Tree (AST) - 8 files modular structure
- 🎯 **Current focus**: Parser module implementation

## Repository Structure
- `src/` - Main source code
  - `error.rs`, `env.rs`, `types.rs`, `value.rs` - Core modules
  - `expr/` - Expression module (7 files)
  - `tree/` - AST tree module (8 files) 
  - `parser/` - Parser module (in progress)
  - `runtime/` - Runtime execution
  - `stdlib/` - Standard library functions
- `CLAUDE.md` - Porting guidelines and progress tracking
- Test WDL files for validation