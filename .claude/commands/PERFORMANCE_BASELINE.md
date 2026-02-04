# Performance Baseline Workflow

**Role:** You are a performance analyst working in a concurrent multi-agent Git environment. Your goal is to establish and monitor Go benchmark baselines, identify regressions, generate optimization prompts for other Claude instances, and verify fixes through iterative benchmark cycles.

**Branch Structure:**
- `main` - Parent branch where completed work lands
- `{your-current-branch}` - Your performance branch (already created for you)

**Key Principle:** Performance is measured, not guessed. Use Go's built-in benchmarking and benchstat for statistical significance. A 5% variance is noise; a 50% regression is signal.

## Phase 1: Sync & Baseline Assessment

- **Sync:** Pull the latest changes from `main`:
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Check Existing Benchmarks:**
  ```bash
  # Find all benchmark functions
  grep -rn "^func Benchmark" internal/

  # List benchmark files
  find . -name "*_test.go" -exec grep -l "Benchmark" {} \;
  ```

- **Run Benchmark Suite:**
  ```bash
  # Run all benchmarks with memory stats
  go test -bench=. -benchmem ./...

  # Run benchmarks with multiple iterations for statistical significance
  go test -bench=. -benchmem -count=10 ./... > benchmark-current.txt

  # Run specific package benchmarks
  go test -bench=. -benchmem ./internal/core
  ```

- **Establish Baseline (if none exists):**
  ```bash
  # Run on main branch first
  git stash
  git checkout main
  go test -bench=. -benchmem -count=10 ./... > benchmark-baseline.txt
  git checkout -
  git stash pop
  ```

- **Compare to Baseline:**
  ```bash
  # Install benchstat if needed
  go install golang.org/x/perf/cmd/benchstat@latest

  # Compare results
  benchstat benchmark-baseline.txt benchmark-current.txt
  ```

## Phase 2: Statistical Analysis

### 2.1 Benchstat Output Interpretation

```
name                   old time/op    new time/op    delta
SyncVendor-8           12.5ms ± 3%    15.2ms ± 4%   +21.6%  (p=0.001 n=10+10)
ParseSmartURL-8        245ns ± 2%     248ns ± 1%     ~       (p=0.095 n=10+10)
```

Key fields:
- **delta**: Percentage change (positive = slower)
- **p-value**: Statistical significance (p<0.05 = significant)
- **n**: Sample count
- **~**: No significant change

### 2.2 Regression Detection Thresholds

| Change | Classification | Action |
|--------|---------------|--------|
| ±5% with p>0.05 | Noise | Ignore |
| ±10% with p<0.05 | Minor regression | Investigate |
| ±25% with p<0.05 | Significant regression | Fix this sprint |
| ±50% with p<0.05 | Critical regression | Fix immediately |

### 2.3 Memory Analysis

```bash
# Run with memory profiling
go test -bench=BenchmarkSyncVendor -benchmem -memprofile=mem.prof ./internal/core

# Analyze memory profile
go tool pprof mem.prof
```

Key memory metrics:
- **B/op**: Bytes allocated per operation
- **allocs/op**: Number of allocations per operation

### 2.4 CPU Profiling

```bash
# Run with CPU profiling
go test -bench=BenchmarkSyncVendor -cpuprofile=cpu.prof ./internal/core

# Analyze CPU profile
go tool pprof cpu.prof

# View in browser
go tool pprof -http=:8080 cpu.prof
```

## Phase 3: Categorize Performance Issues

| Severity | Criteria | Action |
|----------|----------|--------|
| **CRITICAL** | >50% regression, p<0.01 | Immediate fix, block release |
| **HIGH** | 25-50% regression, p<0.05 | Fix this sprint |
| **MEDIUM** | 10-25% regression, p<0.05 | Investigate, plan fix |
| **LOW** | <10% regression | Monitor, may be noise |

### Root Cause Categories

| Category | Indicators | Common Causes |
|----------|------------|---------------|
| **Algorithm** | Scales with input size | O(n) became O(n²), unnecessary loops |
| **Memory** | High allocs/op | String concatenation, unnecessary copies |
| **I/O** | High wall time, low CPU | Excessive file/network operations |
| **Concurrency** | Scales inversely with parallelism | Lock contention, goroutine overhead |

## Phase 4: Optimization Prompt Generation

### Prompt Template for Regression Fix

```
TASK: Fix performance regression in [Function]

REGRESSION DETECTED:
- Baseline: [X] ns/op, [Y] allocs/op
- Current: [A] ns/op, [B] allocs/op
- Regression: [Z]% slower
- p-value: [N] (statistically significant)

BENCHMARK:
```go
func Benchmark[Name](b *testing.B) {
    for i := 0; i < b.N; i++ {
        // benchmark code
    }
}
```

WHEN REGRESSION STARTED:
[Date/commit if identifiable from git bisect]

INVESTIGATION CHECKLIST:
1. Check recent code changes to this function
2. Run CPU profile to find hotspots
3. Run memory profile to find allocation sites
4. Check for algorithm complexity changes

LIKELY CAUSE: [Based on analysis]

OPTIMIZATION APPROACHES:

If Algorithm Issue:
- Review recent changes for complexity regressions
- Look for nested loops, repeated calculations
- Consider caching, memoization

If Memory Issue:
- Use sync.Pool for frequently allocated objects
- Pre-allocate slices with known capacity
- Avoid string concatenation in loops (use strings.Builder)
- Pass large structs by pointer

If I/O Issue:
- Batch operations where possible
- Add caching layer
- Use buffered I/O

If Concurrency Issue:
- Reduce lock contention
- Use RWMutex where appropriate
- Consider lock-free alternatives

REQUIRED:
1. Identify root cause via profiling
2. Implement fix
3. Re-run benchmarks to verify
4. Ensure no regression in other areas

VERIFICATION:
- [ ] `go test -bench=. -benchmem` shows improvement
- [ ] Performance within 10% of baseline
- [ ] No new regressions introduced
- [ ] All existing tests pass

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

### Prompt Template for New Benchmark

```
TASK: Add benchmark for [Function]

FUNCTION ANALYSIS:
- Location: internal/core/[file].go:[line]
- Signature: func [signature]
- Expected hot path: [Yes/No]
- Current coverage: No benchmark exists

BENCHMARK FILE: internal/core/[name]_test.go

BENCHMARK TEMPLATE:
```go
func Benchmark[FunctionName](b *testing.B) {
    // Setup (not timed)
    [setup code]

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        [function call]
    }
}

// For parallel benchmarks
func Benchmark[FunctionName]Parallel(b *testing.B) {
    [setup]
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            [function call]
        }
    })
}
```

CONSIDERATIONS:
- Test with representative input sizes
- Include memory-heavy operations
- Consider parallel benchmark if function is concurrent

VERIFICATION:
- [ ] Benchmark runs without error
- [ ] Benchmark produces consistent results
- [ ] Baseline recorded in benchmark-baseline.txt

Commit, pull main and merge it into your branch, then push to your branch when complete.
```

## Phase 5: Performance Report

```markdown
## Performance Baseline Report

### Executive Summary

| Metric | Value |
|--------|-------|
| Functions Benchmarked | X |
| Regressions Detected | Y |
| Critical Regressions | Z |
| Coverage of Hot Paths | N% |

### Benchmark Results

| Benchmark | Baseline | Current | Change | Status |
|-----------|----------|---------|--------|--------|
| BenchmarkSync | 12.5ms | 15.2ms | +21.6% | REGRESSION |
| BenchmarkParse | 245ns | 248ns | ~ | OK |

### Memory Analysis

| Benchmark | B/op | allocs/op | Status |
|-----------|------|-----------|--------|
| BenchmarkSync | 4096 | 12 | HIGH |
| BenchmarkParse | 128 | 2 | OK |

### Functions Without Benchmarks

| Function | File | Priority |
|----------|------|----------|
| syncVendor | vendor_syncer.go | HIGH |
| ParseSmartURL | git_operations.go | MEDIUM |

### Prompts Generated

---

## PROMPT 1: Fix regression in [Name]
[Full prompt]

---
```

## Phase 6: Verification Cycle

After optimization prompts are executed:

- **Sync:**
  ```bash
  git fetch origin main
  git merge origin/main
  ```

- **Re-Run Benchmarks:**
  ```bash
  go test -bench=. -benchmem -count=10 ./... > benchmark-after.txt
  ```

- **Compare to Baseline:**
  ```bash
  benchstat benchmark-baseline.txt benchmark-after.txt
  ```

- **Verify Regression Fixed:**
  - Delta should be neutral or positive
  - p-value should indicate no significant difference from baseline

- **Check for New Regressions:**
  ```bash
  # Compare all benchmarks
  benchstat benchmark-current.txt benchmark-after.txt
  ```

- **Grade Each Optimization:**

  | Grade | Criteria | Action |
  |-------|----------|--------|
  | **PASS** | Regression fixed, within baseline | Complete |
  | **IMPROVED** | Better than baseline | Update baseline, complete |
  | **PARTIAL** | Improved but still regressed | Follow-up for more optimization |
  | **NO-CHANGE** | No improvement | Different approach needed |
  | **REGRESSION** | Made it worse | Revert, investigate |

## Phase 7: Baseline Update & Sign-Off

When performance is healthy:

- **Update Baseline (if intentional change):**
  ```bash
  go test -bench=. -benchmem -count=10 ./... > benchmark-baseline.txt
  git add benchmark-baseline.txt
  git commit -m "perf: update benchmark baseline"
  ```

- **Push and PR:**
  ```bash
  git push -u origin {your-branch-name}
  ```

- **Performance Report:**

  ```markdown
  ## Performance Cycle Complete

  ### Regressions Resolved

  | Function | Was | Now | Fix Applied |
  |----------|-----|-----|-------------|
  | SyncVendor | +21% | ~0% | Reduced allocations |

  ### Baselines Updated

  | Benchmark | Old | New | Reason |
  |-----------|-----|-----|--------|
  | BenchmarkParse | 245ns | 180ns | Algorithm improvement |

  ### Performance Health
  - All benchmarks within baseline: Yes
  - New benchmarks added: 2
  - Monitoring recommendations: [List]
  ```

---

## Performance Analysis Quick Reference

### Quick Benchmark Run

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific benchmark
go test -bench=BenchmarkSyncVendor -benchmem ./internal/core

# Run with count for statistical analysis
go test -bench=. -count=10 ./... | tee results.txt

# Compare two result files
benchstat old.txt new.txt
```

### Profiling Commands

```bash
# CPU profile
go test -bench=BenchmarkX -cpuprofile=cpu.prof ./pkg
go tool pprof -http=:8080 cpu.prof

# Memory profile
go test -bench=BenchmarkX -memprofile=mem.prof ./pkg
go tool pprof mem.prof

# Trace
go test -bench=BenchmarkX -trace=trace.out ./pkg
go tool trace trace.out
```

### Benchmark Best Practices

```go
// Good: Reset timer after setup
func BenchmarkX(b *testing.B) {
    data := setupTestData() // Not timed
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        processData(data)
    }
}

// Good: Use b.ReportAllocs() for memory tracking
func BenchmarkX(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        allocatingFunction()
    }
}

// Good: Use sub-benchmarks for different inputs
func BenchmarkX(b *testing.B) {
    sizes := []int{10, 100, 1000}
    for _, size := range sizes {
        b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
            data := make([]byte, size)
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                processData(data)
            }
        })
    }
}
```

---

## git-vendor Performance Areas

### Critical Paths to Benchmark

| Function | Location | Importance |
|----------|----------|------------|
| syncVendor | vendor_syncer.go | HIGH - core operation |
| ParseSmartURL | git_operations.go | MEDIUM - URL parsing |
| CopyDir | filesystem.go | HIGH - file operations |
| Clone | git_operations.go | HIGH - git operations |
| ParallelExecutor | parallel_executor.go | HIGH - concurrency |

### Performance Considerations

- **Git operations**: Mostly I/O bound, limited optimization opportunity
- **File copying**: Can be optimized with buffering, parallel copying
- **URL parsing**: CPU bound, regex optimization possible
- **Parallel execution**: Goroutine overhead, channel contention

---

## Integration Points

- **internal/core/*_test.go** - Benchmark files
- **benchmark-baseline.txt** - Baseline measurements
- **Makefile** - `make bench` target
- **CLAUDE.md** - Performance documentation
