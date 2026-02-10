# Prompt: Set 3 — Context Propagation Completion (CQ-003 Phase 2)

## Concurrency
CONFLICTS with Sets 2 and 4 (all touch main.go, engine.go, vendor_syncer.go).
SAFE with Sets 1 and 5.

## Branch
Create and work on a branch named `claude/set3-context-propagation-<session-suffix>`.

## Context
`Scan()` was updated to accept `context.Context` for cancellation support (CQ-003/CQ-006). The same pattern needs to be applied to all other long-running operations so users can Ctrl+C any command.

Currently context-aware: `Scan(ctx, failOn)`
Missing context: `Sync()`, `SyncWithOptions()`, `SyncWithGroup()`, `SyncWithParallel()`, `UpdateAll()`, `UpdateAllWithParallel()`, `Drift()`, `Verify()`, `FetchRepoDir()`, `CheckUpdates()`

## Task
Thread `context.Context` through all long-running Manager/VendorSyncer/Service methods, following the exact pattern established by `Scan()`.

### Phase 1: Interface + Service signatures

Update these interfaces to accept `context.Context` as first parameter:

1. **`SyncServiceInterface`** in `sync_service.go`:
   - `Sync(ctx, cfg, lock, ...)` — all Sync variants
   - `SyncVendor(ctx, ...)`, `syncRef(ctx, ...)`

2. **`UpdateServiceInterface`** in `update_service.go`:
   - `UpdateAll(ctx, ...)`, `UpdateAllWithOptions(ctx, ...)`

3. **`VerifyServiceInterface`** in `verify_service.go`:
   - `Verify(ctx)` — verify is local-only today but context enables future timeout

4. **`DriftServiceInterface`** in `drift_service.go`:
   - `Drift(ctx, opts)`

5. **`UpdateCheckerInterface`** in `update_checker.go`:
   - `CheckUpdates(ctx)`

6. **`RemoteExplorerInterface`** in `remote_explorer.go`:
   - `FetchRepoDir(ctx, url, ref, subdir)` — already has internal 30s timeout, should derive from parent

### Phase 2: VendorSyncer delegation

Update `vendor_syncer.go` methods to accept and forward context:
- `Sync(ctx)`, `SyncDryRun(ctx)`, `SyncWithOptions(ctx, ...)`, `SyncWithGroup(ctx, ...)`, `SyncWithParallel(ctx, ...)`
- `UpdateAll(ctx)`, `UpdateAllWithParallel(ctx, ...)`
- `Verify(ctx)`, `Drift(ctx, opts)`, `CheckUpdates(ctx)`
- `FetchRepoDir(ctx, url, ref, subdir)`

### Phase 3: Manager facade (engine.go)

Update all corresponding `Manager` methods to accept and forward context.

### Phase 4: CLI entry points (main.go)

For each command that calls these methods, create a signal-aware context:

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

Commands needing this: `sync`, `update`, `verify`, `drift`, `status`, `check-updates`, `diff`, `watch`

The `scan` command already has this pattern — match it exactly.

### Phase 5: Test updates

Every test file that calls these methods needs `context.Background()` added:
- `sync_service_test.go` — all Sync() calls
- `update_service_test.go` — all UpdateAll() calls
- `verify_service_test.go` — all Verify() calls
- `drift_service_test.go` — all Drift() calls
- `update_checker_test.go` — all CheckUpdates() calls
- `vendor_syncer_test.go` — all delegation test stubs
- Any other test files calling these methods

For each stub/mock, update the Scan-style pattern:

    // Before
    func (s *stubSyncService) Sync(...) error {
    // After
    func (s *stubSyncService) Sync(ctx context.Context, ...) error {

### Key patterns
- The `Scan()` implementation is the reference: see `vuln_scanner.go:179` and `main.go` scan case
- Use `context.WithTimeout(ctx, ...)` when deriving sub-contexts (not `context.Background()`)
- The parallel executor's `ExecuteParallelSync/Update` should accept parent context and pass to workers
- `FetchRepoDir` already uses `context.WithTimeout(context.Background(), 30*time.Second)` internally — change to derive from parent ctx
- Short-circuit pattern: `if ctx.Err() != nil { return ctx.Err() }` at loop boundaries

### What NOT to change
- `Init()`, `SaveVendor()`, `RemoveVendor()`, `AddVendor()` — fast local operations, no context needed
- `GetConfig()`, `GetLock()`, `GetLockHash()` — pure reads
- Spec 072 config commands — local YAML operations
- `WatchConfig()` — already has its own loop; context could be added but is out of scope
- `GenerateSBOM()` — local generation, no I/O

### Definition of Done
1. `go build` succeeds
2. `go test ./...` passes (all 1013+ tests, including updated signatures)
3. `go vet ./...` clean
4. All long-running operations honor context cancellation
5. All `context.Background()` in services replaced with parent context derivation
6. Inline docs updated on changed signatures
7. CLAUDE.md updated: note context propagation in Key Operations
8. Commit and push
