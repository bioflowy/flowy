# Code Style and Conventions

## Rust Style Guidelines
- Follow standard Rust naming conventions (snake_case for functions/variables, PascalCase for types)
- Use idiomatic Rust patterns (Result types, Option, pattern matching)
- Comprehensive error handling with `thiserror` crate
- Use `serde` for serialization/deserialization

## Code Organization
- Modular structure: Large modules split into multiple files in subdirectories
- Each module has its own `mod.rs` file as entry point
- Tests are co-located with code or in separate test files
- Clear separation of concerns between modules

## Documentation
- Include documentation comments for public APIs
- Use `///` for doc comments on public items
- Provide comprehensive error messages

## Error Handling Protocol (CRITICAL)
- When tests fail, ALWAYS ask user for confirmation before making changes
- Compilation errors can be fixed directly without confirmation
- Present error, diagnosis, and proposed fix for user approval on test failures

## Dependencies Management
- Minimal dependencies approach
- Prefer standard library when possible
- Use well-established crates (thiserror, serde, nom)