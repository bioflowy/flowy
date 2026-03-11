# Claude Instructions for flowy

## Language

Write all documentation, comments, commit messages, and planning files (CONTEXT.md, PLAN.md, RESEARCH.md, etc.) in **English**.

Conversation with the user may be in Japanese. Code comments and docs must be English.

## Technical Debt Tracking

When you apply a temporary or workaround fix just to make tests pass, but a more fundamental/proper fix is needed, you **must** create a file documenting the situation. The file should be placed under `docs/technical-debt/` with a descriptive name (e.g., `docs/technical-debt/type-coercion-workaround.md`) and include:

1. **Problem**: What the underlying issue is
2. **Temporary fix applied**: What was done to pass the tests
3. **Why it's not sufficient**: What scenarios or edge cases the temporary fix doesn't cover
4. **Proposed proper fix**: How to fundamentally resolve the issue
5. **Affected files**: List of files involved in the temporary fix
