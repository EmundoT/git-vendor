---
paths:
  - "internal/**/*.go"
  - "main.go"
---

# Go Cross-File Edit Safety

gopls (Go language server) runs on file save and may revert files when the package fails to compile. Interface signature changes that span multiple files MUST be applied atomically.

## Rules

1. **Identify all dependents before editing.** Before changing an interface method signature, grep for all callers, implementations, and test stubs. Plan the full edit set.

2. **Edit in dependency order within one tool-call batch.** Interface definition -> implementations -> callers -> tests. Use parallel Edit calls so all files update before gopls re-checks.

3. **Isolate new standalone code in separate files.** New types or functions with no cross-file dependencies SHOULD go in their own file (e.g., `local_path.go` not appended to `git_operations.go`). Standalone files survive the linter independently.

4. **On "File has been modified since read" errors**, re-read only the failed file and retry. gopls likely touched it after a sibling edit changed compilation state.

5. **MUST NOT leave the package in a broken state between tool calls.** If you cannot update all dependent files in one pass, do not start the edit.
