# PROMPT: CQ-003 Context Propagation

## Task
Add `context.Context` as the first parameter to all long-running operations for proper cancellation and timeout handling.

## Priority
HIGH - Enables proper cancellation, timeout handling, and graceful shutdown.

## Problem
Some long-running operations don't accept `context.Context`, preventing proper cancellation and timeout handling. The vulnerability scanner correctly uses `context.WithTimeout`, but other services don't propagate context.

## Affected Files
- `internal/core/vendor_syncer.go` (Sync, SyncWithOptions, etc.)
- `internal/core/sync_service.go` (Sync, SyncVendor)
- `internal/core/update_service.go` (UpdateAll, UpdateAllWithOptions)
- `internal/core/sbom_generator.go` (Generate)
- `internal/core/git_operations.go` (Clone, Fetch, ListTree - add runWithContext)
- `internal/core/github_client.go` (CheckLicense - already uses context internally)
- `main.go` (create contexts with timeouts at CLI entry points)

## Implementation Steps

### 1. Add runWithContext to GitClient
```go
// In git_operations.go
func (g *SystemGitClient) runWithContext(ctx context.Context, dir string, args ...string) error {
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = dir
    // ... rest of implementation
}
```

### 2. Update GitClient Interface
```go
type GitClient interface {
    Clone(ctx context.Context, url, dest string, opts types.CloneOptions) error
    Fetch(ctx context.Context, dir string, depth int, ref string) error
    // ... all methods get ctx as first param
}
```

### 3. Update VendorSyncer Public Methods
```go
func (s *VendorSyncer) Sync(ctx context.Context) error
func (s *VendorSyncer) SyncWithOptions(ctx context.Context, vendorName string, force, noCache bool) error
```

### 4. Update Call Sites in main.go
```go
// Create context with timeout for sync operations
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

if err := manager.Sync(ctx); err != nil {
    // Handle error
}
```

### 5. Update Mock Implementations
All mock interfaces in test files need updated signatures.

## Mandatory Tracking Updates
1. Update `ideas/code_quality.md` - change CQ-003 status to "completed"
2. Add completion notes under "## Completed Issue Details"

## Acceptance Criteria
- [ ] All GitClient methods accept context.Context as first parameter
- [ ] All VendorSyncer public methods accept context.Context
- [ ] Git commands use exec.CommandContext for cancellation
- [ ] HTTP clients use request.WithContext for API calls
- [ ] main.go creates contexts with appropriate timeouts
- [ ] All mock implementations updated to match new signatures
- [ ] `go build ./...` - no compilation errors
- [ ] `go test ./...` - all tests pass

## GIT WORKFLOW (MANDATORY)
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   ```
   git fetch origin main
   git merge origin/main
   ```
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   ```
   git push -u origin <your-branch-name>
   ```
