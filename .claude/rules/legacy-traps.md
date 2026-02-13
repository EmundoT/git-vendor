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
ValidateVendorURL() (SEC-011) rejects file://, ftp://, javascript:, data: by default. Only https://, http://, ssh://, git://, git+ssh://, and SCP-style (git@host:path) accepted without flags. The `--local` flag on `sync` and `update` commands opts in to file:// and local filesystem paths. SyncVendor checks IsLocalPath() before git operations and resolves relative paths via ResolveLocalURL(). Without `--local`, local paths produce an error with a hint.

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

## Internal Vendor Traps (Spec 070)

### Transform pipeline for internal vendors
REJECTED: Transforms (extract-section, embed-json, template rendering) are out of scope for Spec 070. Internal vendors support exact file copy and position extraction/placement only. Transforms deferred to a separate spec.

### Parallel sync for internal vendors
REJECTED: Internal vendors sync sequentially even with `--parallel`. Internal mappings may share destination files; concurrent writes would race. External vendors can still parallelize independently.

### Full compliance enforcement modes (Spec 075)
DEFERRED: Spec 070 implements drift detection and propagation only. Strict/lenient/info enforcement levels, CI exit codes, and policy-based compliance gating are deferred to Spec 075.

### RefLocal is not a git ref
`RefLocal` ("local") is a sentinel value for internal vendors. MUST NOT pass to git checkout, fetch, or any git-plumbing operation. Internal vendors use `os.Stat`/`os.ReadFile` for all file access. Passing RefLocal to git operations will produce cryptic errors.

### Position auto-update only handles line-range specs
`updatePositionSpecs` adjusts `L5-L20` style line ranges when propagation changes file line count. ToEOF specs auto-expand (no update needed). Single-line specs are stable. Column specs (`L1C5:L10C30`) are NOT auto-updated — byte offsets shift unpredictably. Document this to users.

### Internal vendors sync before external in unified `git vendor sync`
Internal vendors have no network dependency and MUST complete before external sync begins. If internal sync fails, external sync still proceeds (no atomic abort). Results are collected separately and reported together.

## Config / Data Gotchas

- Empty PathMapping.To uses auto-naming based on source basename
- Policy file detection uses os.Stat(PolicyFile), not heuristics. Malformed = error (no silent fallback)
- Incremental sync cache: .git-vendor/.cache/, auto-invalidates on commit hash change, 1000 file limit per vendor
- Watch mode: 1-second debounce, watches vendor.yml only
- types.Trailer and git.Trailer are distinct Go types with identical fields. Explicit conversion required in SystemGitClient.Commit()
