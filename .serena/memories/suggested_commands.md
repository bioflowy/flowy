# Essential Development Commands

## Testing
- `cargo test` - Run all tests
- `cargo test --lib` - Run library tests only  
- `cargo test <module>` - Run specific module tests
- `cargo test -- --nocapture` - Show print output during tests
- `RUST_BACKTRACE=1 cargo test` - Show backtraces on test failures

## Development
- `cargo build` - Build project
- `cargo build --release` - Release build
- `cargo check` - Fast compilation check without generating binaries
- `cargo run` - Build and run main binary

## Code Quality  
- `cargo clippy` - Linter (check for clippy warnings in CI)
- `cargo fmt` - Code formatter
- `cargo fmt --check` - Check formatting without applying

## Project Management
- `cargo clean` - Clean build artifacts
- `cargo update` - Update dependencies

## Git Operations (Linux)
- Standard git commands: `git status`, `git add`, `git commit`, `git push`
- `ls` - List files
- `find` - Find files (though prefer Rust tools when available)
- `grep` - Search in files (though prefer ripgrep `rg` when available)