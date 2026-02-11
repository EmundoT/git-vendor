---
paths:
  - "internal/**/*.go"
---

# Legacy Traps and Gotchas

## Rejected Approaches

### os.IsNotExist() for wrapped errors
MUST NOT use `os.IsNotExist(err)` when the error may have been wrapped with `fmt.Errorf("%w")`. MUST use `errors.Is(err, os.ErrNotExist)`. Go's `os.IsNotExist` does not unwrap.

### net/http.DetectContentType for binary detection
Rejected in favor of git's null-byte heuristic (scan first 8000 bytes for \x00). DetectContentType only inspects 512 bytes and can misclassify source code as application/octet-stream.

### Bare hex vs sha256: prefix in position verify
ComputeFileChecksum returns bare hex, but ExtractPosition returns "sha256:<hex>". When comparing in verifyPositions for whole-file destinations (destPos == nil), MUST normalize ComputeFileChecksum output to "sha256:" prefix. Mixing formats causes false drift detection.

### file:// URLs in vendor.yml
MUST NOT allow file:// scheme. ValidateVendorURL() rejects file://, ftp://, javascript:, data:. Only https://, http://, ssh://, git://, git+ssh://, and SCP-style (git@host:path) accepted (SEC-011).

### Logging URLs with credentials
MUST NOT include raw URLs in error messages when URL may contain embedded credentials. Use SanitizeURL() to strip userinfo (SEC-013).

### Unbounded YAML file reads
MUST NOT read vendor.yml/vendor.lock without checking file size. YAMLStore.Load() enforces 1 MB limit (maxYAMLFileSize) (SEC-020).

### One-commit-per-vendor
Original design created N commits for N vendors. Replaced with single-commit + multi-valued trailers for atomic semantics. MUST NOT regress.

## Platform Gotchas

- Hook execution: sh -c (Unix), cmd /c (Windows)
- Position parser uses first `:L<digit>` to split, avoiding false match on Windows drive letters (C:\path)
- CRLF normalized to LF in position extraction. Standalone \r (classic Mac) NOT normalized
- CopyDir does not follow directory symlinks (filepath.Walk uses os.Lstat). File symlinks ARE dereferenced

## Position Extraction Gotchas

- EndCol is 1-indexed inclusive byte offset: L1C5:L1C10 = bytes 5-10 (6 bytes). Go slice: `line[StartCol-1 : EndCol]`
- Column semantics are byte-offset, not rune-offset. Multi-byte chars: emoji=4, CJK=3, accented=2 bytes. Partial extraction = invalid UTF-8
- Trailing newline creates phantom empty line: "a\nb\n" = 3 lines ("a", "b", "")
- Empty file has 1 line. L1 = empty string. L2+ errors
- Sequential PlaceContent calls operate on modified content. Line count changes shift positions
- L1-EOF hash equals whole-file hash (after CRLF normalization)

## Error Handling Gotchas

- tui.PrintError takes string, not error. Call `.Error()` on sentinel errors
- HookError wraps with vendor name, phase, command. 5-min timeout. Env var values sanitized (newlines/null bytes stripped). Timeout tests MUST use `exec sleep` not bare `sleep` to prevent orphaned processes
- canSkipSync() logs warning and forces re-sync on cache corruption
- OSVAPIError wraps HTTP errors. Response bodies limited to 10 MB via io.LimitReader. Rate limit (429), server error (5xx), client error (4xx) produce distinct messages

## Verify Gotchas

- Verify produces separate position-level and whole-file results. Both types for same file = two results; position can fail independently
- IsBinaryContent is exported for use in both position extraction and whole-file copy warnings (SEC-023)

## Config / Data Gotchas

- Empty PathMapping.To uses auto-naming based on source basename
- Policy file detection uses os.Stat(PolicyFile), not heuristics. Malformed = error (no silent fallback)
- Incremental sync cache: .git-vendor/.cache/, auto-invalidates on commit hash change, 1000 file limit per vendor
- Watch mode: 1-second debounce, watches vendor.yml only
- types.Trailer and git.Trailer are distinct Go types with identical fields. Explicit conversion required in SystemGitClient.Commit()
