## Task: Position Extraction Test Hardening (Spec 071)

Spec 071 (position extraction) was recently implemented. Add edge case tests to
harden the new code paths.

### Constraints
- ONLY modify these files:
  - `internal/core/position_extract.go` (fix bugs found during testing only)
  - `internal/core/position_extract_test.go`
  - `internal/core/file_copy_service.go` (fix bugs found during testing only)
  - `internal/core/file_copy_service_test.go`
- DO NOT touch `git_operations.go`, `engine.go`, `go.mod`, `go.sum`
- DO NOT touch `verify_service.go`, `validation_service.go`, `filesystem.go`
- Generate mocks first: `make mocks`

### Test cases needed for ExtractPosition

**Line extraction edge cases:**
1. Extract single line from 1-line file
2. Extract L1-EOF from file produces entire file content
3. StartLine greater than file length produces clear error message
4. EndLine greater than file length produces clear error message (or clamp?)
5. Empty file with any position spec produces error
6. File with no trailing newline — verify extraction preserves content exactly
7. Lines with mixed line endings (CRLF vs LF) — verify consistent handling

**Column extraction edge cases:**
8. StartCol greater than line length produces error
9. EndCol greater than line length produces error or clamp?
10. Single character extraction: L1C5:L1C5 produces 1 char
11. Full line via columns: L1C1:L1C{len} produces entire line
12. Multi-line column extraction preserves intermediate lines fully
13. Unicode characters — column indexing is byte-based or rune-based?

**Hash verification:**
14. Same content extracted twice produces identical SHA-256 hash
15. Different content produces different hash
16. Hash is stable across extractions (deterministic)

### Test cases needed for PlaceContent

**Placement edge cases:**
17. Place into empty file creates file with content
18. Place at L1 of existing file replaces first line(s)
19. Place at line beyond file end produces error or extends?
20. Place with column spec into existing file — verify surrounding content preserved
21. Overwrite same range twice with same content is idempotent
22. Overwrite same range with different content fully replaces old content

### Test cases for CopyMappings position integration

23. Position mapping with nonexistent source file produces propagated error
24. Position mapping where destination dir does not exist — created automatically?
25. Multiple position mappings to same destination file at different ranges
26. Mix of whole-file and position mappings in same CopyMappings call

### Definition of Done
1. `go test -v ./internal/core/ -run TestExtract` all pass
2. `go test -v ./internal/core/ -run TestPlace` all pass
3. `go test -v ./internal/core/ -run TestCopy` all pass
4. `go test -cover ./internal/core/` shows improved coverage
5. `make lint` is green
6. Any bugs found are fixed with clear commit messages
7. Any intentional limitations discovered are documented as code comments
