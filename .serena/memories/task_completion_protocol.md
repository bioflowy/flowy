# Task Completion Protocol

## Required Steps When Task is Complete

### 1. Run Tests  
- `cargo test` - Ensure all tests pass
- If tests fail: **CRITICAL - Ask user for confirmation before fixing**
- Only proceed after all tests pass

### 2. Code Quality Checks
- `cargo clippy` - Run linter, fix any warnings
- `cargo fmt` - Format code 
- Both must pass without warnings

### 3. Build Verification
- `cargo build` - Ensure clean build
- `cargo check` - Fast compilation check

### 4. Module Completion Criteria (for porting tasks)
Before proceeding to next module:
- ✅ All tests pass
- ✅ Documentation complete  
- ✅ API matches Python equivalent functionality
- ✅ User approval received

## Error Handling During Completion
- **Test failures**: Always get user confirmation on cause and fix
- **Compilation errors**: Can fix directly
- **Clippy warnings**: Fix automatically following Rust best practices

## Git Workflow
- Do NOT commit changes unless user explicitly requests it
- Keep changes focused and atomic
- Follow existing commit message patterns in the repository