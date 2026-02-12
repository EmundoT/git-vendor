---
paths:
  - "internal/core/position_extract.go"
  - "internal/core/file_copy_service.go"
  - "internal/types/position.go"
---

# Position Extraction (Spec 071)

Fine-grained file vendoring -- extract specific line/column ranges from source files and place at specific positions in destination files.

## Syntax (appended to path with :)

```text
file.go:L5          # Single line 5
file.go:L5-L20      # Lines 5 through 20
file.go:L5-EOF      # Line 5 to end of file
file.go:L5C10:L10C30  # Line 5 col 10 through line 10 col 30 (1-indexed inclusive bytes)
```

## Pipeline

1. ParsePathPosition() splits path:Lspec into file path + PositionSpec
2. ExtractPosition() reads file, normalizes CRLF->LF, extracts content, returns content + SHA-256 hash (prefixed "sha256:<hex>")
3. PlaceContent() normalizes existing CRLF->LF, writes extracted content at specified position
4. CopyStats.Positions carries positionRecord (From, To, SourceHash) back to caller
5. toPositionLocks() converts to PositionLock for lockfile persistence

## Column Semantics (CRITICAL)

EndCol is 1-indexed INCLUSIVE byte offset. L1C5:L1C10 extracts bytes 5-10 (6 bytes).
Go slice: `line[StartCol-1 : EndCol]` (Go's exclusive upper bound equals 1-indexed inclusive).

Columns are BYTE offsets, NOT Unicode rune offsets. Multi-byte characters: emoji=4 bytes, CJK=3, accented=2. Extracting partial multi-byte character produces invalid UTF-8.

## CRLF Normalization

extractFromContent and placeInContent normalize \r\n -> \n before processing. Extracted content always uses LF. Files with CRLF will have line endings changed to LF after PlaceContent.

Standalone \r (classic Mac) is NOT normalized.

## Edge Cases

- Empty file has 1 line (splits to [""]). L1 extracts empty string. L2+ errors
- Trailing newline creates phantom empty line: "a\nb\n" = 3 lines ("a", "b", "")
- L1-EOF hash equals whole-file hash (after CRLF normalization)
- Sequential PlaceContent calls operate on MODIFIED content. Second call sees file changed by first. Line count changes shift target positions

## Sync-time Behavior

checkLocalModifications() warns (not errors) if destination differs before overwrite:
"Warning: <path> has local modifications at target position that will be overwritten"

## Verify-time Behavior

verifyPositions() reads destination locally, extracts target range, hashes, compares to stored SourceHash. No network access -- purely local. Position entries produce SEPARATE verification results from whole-file entries.

## Binary Detection

ExtractPosition and PlaceContent (with position) reject binary files via IsBinaryContent() (null-byte scan, first 8000 bytes). Whole-file replacement (nil pos) bypasses check.

## Windows Path Safety

Position parser uses first `:L<digit>` occurrence to split, avoiding false matches on Windows drive letters like C:\path.
