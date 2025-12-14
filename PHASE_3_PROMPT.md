# Phase 3: Split Monolithic Test File

Continue implementing Phase 3 of the code quality refactor plan located at:
/home/emt/.claude/plans/transient-exploring-mist.md

Phase 2 (Decompose God Object) is complete. Now implement Phase 3: Split Monolithic Test File.

This phase involves breaking the 3,318-line engine_test.go into focused test files:

1. Create testhelpers.go (~120 lines) - Fluent test builder API
2. Split tests into 8 focused test files:
   - utils_test.go (~150 lines, 5 tests)
   - vendor_repository_test.go (~280 lines, 9 tests)
   - sync_service_test.go (~500 lines, 15 tests)
   - update_service_test.go (~350 lines, 10 tests)
   - license_service_test.go (~220 lines, 7 tests)
   - file_copy_service_test.go (~280 lines, 8 tests)
   - validation_service_test.go (~420 lines, 12 tests)
   - remote_explorer_test.go (~280 lines, 8 tests)
3. Delete engine_test.go after migration

Reference the plan file for detailed implementation guidance, including the fluent TestBuilder API pattern and test organization strategy.

## Goals

- Eliminate 40+ repetitive test setup blocks with TestBuilder
- ~700 lines saved through consolidated helpers
- Each test file mirrors its implementation file
- All tests must continue to pass
- Maintain or improve 62.7% coverage

## What Remains After Phase 3

### Phase 4: Reorganize Mock Infrastructure (Week 5 - LOW RISK)
**Goal:** Replace 522 lines of manual mocks with auto-generated mockgen mocks
- Create `Makefile` with mock generation targets
- Use `gomock/mockgen` (Go community standard)
- Delete `internal/core/mocks_test.go`
- Type-safe mock verification
- Auto-updates when interfaces change

### Phase 5: Decompose TUI Wizard (Week 6 - LOW RISK)
**Goal:** Split 498-line `wizard.go` into 8 focused files
- Extract `styles.go` (~50 lines)
- Extract `helpers.go` (~90 lines)
- Split wizards: `wizard_add.go` (~130 lines), `wizard_edit.go` (~160 lines)
- Extract browser interface: `browser.go`, `browser_remote.go`, `browser_local.go`
- Extract `conflict_display.go` (~70 lines)

### Phase 6: Remove Unnecessary Indirection (Week 7 - MEDIUM RISK)
**Goal:** Delete 220-line `engine.go` wrapper
- Rename VendorSyncer → VendorManager
- Make VendorManager the primary API
- Update `main.go` to use VendorManager directly
- Update all tests
- Eliminates pure delegation wrapper

### Phase 7: Document Code Patterns (Week 8 - LOW RISK)
**Goal:** Comprehensive documentation for contributors
- Create `docs/ARCHITECTURE.md` - Architecture overview & diagrams
- Create `docs/CONTRIBUTING.md` - Code quality standards & workflow
- Create `docs/PATTERNS.md` - Common patterns & best practices

## Progress Summary

| Phase | Status | Lines Saved | Key Achievement |
|-------|--------|-------------|-----------------|
| **Phase 1** | ✅ Complete | ~120 | Consolidated utilities & YAML stores with generics |
| **Phase 2** | ✅ Complete | ~400 | Decomposed God object into 7 services (67% reduction) |
| **Phase 3** | 🔜 Next | ~700 | Split 3,318-line test file with fluent helpers |
| **Phase 4** | ⏳ Pending | 522 | Auto-generate mocks with mockgen |
| **Phase 5** | ⏳ Pending | ~80 | Decompose wizard into 8 files |
| **Phase 6** | ⏳ Pending | 220 | Remove Manager wrapper |
| **Phase 7** | ⏳ Pending | 0 | Comprehensive documentation |

**Total Impact:** ~2,000+ lines eliminated through consolidation and cleanup
