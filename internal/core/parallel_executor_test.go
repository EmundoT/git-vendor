package core

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

// TestParallelExecutor_SyncMultipleVendors tests parallel sync with multiple vendors
func TestParallelExecutor_SyncMultipleVendors(t *testing.T) {
	// Create 5 test vendors
	vendors := []types.VendorSpec{
		{Name: "vendor-1", URL: "https://github.com/test/repo1", License: "MIT"},
		{Name: "vendor-2", URL: "https://github.com/test/repo2", License: "MIT"},
		{Name: "vendor-3", URL: "https://github.com/test/repo3", License: "MIT"},
		{Name: "vendor-4", URL: "https://github.com/test/repo4", License: "MIT"},
		{Name: "vendor-5", URL: "https://github.com/test/repo5", License: "MIT"},
	}

	lockMap := map[string]map[string]string{
		"vendor-1": {"main": "hash1"},
		"vendor-2": {"main": "hash2"},
		"vendor-3": {"main": "hash3"},
		"vendor-4": {"main": "hash4"},
		"vendor-5": {"main": "hash5"},
	}

	// Mock sync function that simulates work
	syncFunc := func(_ types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return map[string]RefMetadata{"main": {CommitHash: "newhash"}}, CopyStats{FileCount: 10, ByteCount: 1000}, nil
	}

	// Create executor with 2 workers
	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})

	// Execute parallel sync
	results, err := executor.ExecuteParallelSync(vendors, lockMap, SyncOptions{}, syncFunc)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}

	// Verify all vendors were processed
	vendorNames := make(map[string]bool)
	for _, result := range results {
		vendorNames[result.Vendor.Name] = true
		if result.Error != nil {
			t.Errorf("Vendor %s had error: %v", result.Vendor.Name, result.Error)
		}
		if result.Stats.FileCount != 10 {
			t.Errorf("Expected FileCount 10, got %d", result.Stats.FileCount)
		}
	}

	if len(vendorNames) != 5 {
		t.Errorf("Expected 5 unique vendors, got %d", len(vendorNames))
	}
}

// TestParallelExecutor_WorkerCountLimit tests that worker count is capped at maximum
func TestParallelExecutor_WorkerCountLimit(t *testing.T) {
	// Create executor requesting 100 workers
	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 100}, &SilentUICallback{})

	// Verify workers are capped at 8
	if executor.maxWorkers != 8 {
		t.Errorf("Expected maxWorkers to be capped at 8, got %d", executor.maxWorkers)
	}

	// Create executor with 0 workers (should default to NumCPU)
	executor2 := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 0}, &SilentUICallback{})

	expectedWorkers := runtime.NumCPU()
	if expectedWorkers > 8 {
		expectedWorkers = 8
	}

	if executor2.maxWorkers != expectedWorkers {
		t.Errorf("Expected maxWorkers to be %d (NumCPU capped at 8), got %d", expectedWorkers, executor2.maxWorkers)
	}

	// Create executor with 3 workers (should be respected)
	executor3 := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 3}, &SilentUICallback{})

	if executor3.maxWorkers != 3 {
		t.Errorf("Expected maxWorkers to be 3, got %d", executor3.maxWorkers)
	}
}

// TestParallelExecutor_FailFast tests that errors are returned when a vendor fails
func TestParallelExecutor_FailFast(t *testing.T) {
	vendors := []types.VendorSpec{
		{Name: "vendor-1", URL: "https://github.com/test/repo1", License: "MIT"},
		{Name: "vendor-2", URL: "https://github.com/test/repo2", License: "MIT"},
		{Name: "vendor-3", URL: "https://github.com/test/repo3", License: "MIT"},
		{Name: "vendor-4", URL: "https://github.com/test/repo4", License: "MIT"},
		{Name: "vendor-5", URL: "https://github.com/test/repo5", License: "MIT"},
	}

	lockMap := map[string]map[string]string{
		"vendor-1": {"main": "hash1"},
		"vendor-2": {"main": "hash2"},
		"vendor-3": {"main": "hash3"},
		"vendor-4": {"main": "hash4"},
		"vendor-5": {"main": "hash5"},
	}

	// Mock sync function that fails for vendor-2
	syncFunc := func(v types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		time.Sleep(10 * time.Millisecond)
		if v.Name == "vendor-2" {
			return nil, CopyStats{}, errors.New("simulated sync failure")
		}
		return map[string]RefMetadata{"main": {CommitHash: "newhash"}}, CopyStats{FileCount: 5, ByteCount: 500}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})

	// Execute parallel sync
	results, err := executor.ExecuteParallelSync(vendors, lockMap, SyncOptions{}, syncFunc)

	// Should return error
	if err == nil {
		t.Fatal("Expected error when vendor fails, got nil")
	}

	// Error should mention the failing vendor
	if err.Error() != "vendor-2: simulated sync failure" {
		t.Errorf("Expected error to mention vendor-2, got: %v", err)
	}

	// Results should still be returned (all vendors processed)
	if len(results) != 5 {
		t.Errorf("Expected 5 results even with error, got %d", len(results))
	}

	// Verify the specific vendor had an error
	var vendor2Result *VendorResult
	for i := range results {
		if results[i].Vendor.Name == "vendor-2" {
			vendor2Result = &results[i]
			break
		}
	}

	if vendor2Result == nil {
		t.Fatal("vendor-2 result not found")
	}

	if vendor2Result.Error == nil {
		t.Error("Expected vendor-2 to have an error")
	}
}

// TestParallelExecutor_ThreadSafety tests concurrent access safety
func TestParallelExecutor_ThreadSafety(t *testing.T) {
	// Create many vendors to stress test concurrency
	var vendors []types.VendorSpec
	lockMap := make(map[string]map[string]string)

	for i := 0; i < 20; i++ {
		vendorName := "vendor-" + string(rune('a'+i))
		vendors = append(vendors, types.VendorSpec{
			Name:    vendorName,
			URL:     "https://github.com/test/" + vendorName,
			License: "MIT",
		})
		lockMap[vendorName] = map[string]string{"main": "hash"}
	}

	// Shared counter to detect race conditions
	var counter int
	var mu sync.Mutex

	// Mock sync function that accesses shared state safely
	syncFunc := func(_ types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		// Simulate work
		time.Sleep(5 * time.Millisecond)

		// Access shared state (protected by mutex)
		mu.Lock()
		counter++
		mu.Unlock()

		return map[string]RefMetadata{"main": {CommitHash: "newhash"}}, CopyStats{FileCount: 1, ByteCount: 100}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 4}, &SilentUICallback{})

	// Execute parallel sync
	results, err := executor.ExecuteParallelSync(vendors, lockMap, SyncOptions{}, syncFunc)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(results) != 20 {
		t.Errorf("Expected 20 results, got %d", len(results))
	}

	// Verify counter matches vendor count (all vendors processed)
	if counter != 20 {
		t.Errorf("Expected counter to be 20, got %d", counter)
	}

	// Note: Run this test with -race flag to detect race conditions:
	// go test -race -run TestParallelExecutor_ThreadSafety
}

// TestParallelExecutor_ExecuteParallelUpdate tests parallel update functionality
func TestParallelExecutor_ExecuteParallelUpdate(t *testing.T) {
	vendors := []types.VendorSpec{
		{Name: "vendor-1", URL: "https://github.com/test/repo1", License: "MIT"},
		{Name: "vendor-2", URL: "https://github.com/test/repo2", License: "MIT"},
		{Name: "vendor-3", URL: "https://github.com/test/repo3", License: "MIT"},
	}

	// Mock update function
	updateFunc := func(_ types.VendorSpec, _ SyncOptions) (map[string]RefMetadata, error) {
		time.Sleep(10 * time.Millisecond)
		return map[string]RefMetadata{"main": {CommitHash: "updated-hash"}}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})

	// Execute parallel update
	results, err := executor.ExecuteParallelUpdate(vendors, updateFunc)

	// Verify
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Verify all vendors were updated
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Vendor %s had error: %v", result.Vendor.Name, result.Error)
		}
		if result.UpdatedRefs["main"].CommitHash != "updated-hash" {
			t.Errorf("Expected updated-hash, got %s", result.UpdatedRefs["main"].CommitHash)
		}
	}
}

// TestParallelExecutor_EmptyVendorList tests handling of empty vendor list
func TestParallelExecutor_EmptyVendorList(t *testing.T) {
	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})

	syncFunc := func(_ types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		return nil, CopyStats{}, nil
	}

	// Execute with empty vendor list
	results, err := executor.ExecuteParallelSync([]types.VendorSpec{}, nil, SyncOptions{}, syncFunc)

	// Should return nil without error
	if err != nil {
		t.Errorf("Expected no error for empty vendor list, got: %v", err)
	}

	if results != nil {
		t.Errorf("Expected nil results for empty vendor list, got %d results", len(results))
	}
}

// TestParallelExecutor_SingleVendor tests that single vendor doesn't create excessive workers
func TestParallelExecutor_SingleVendor(t *testing.T) {
	vendors := []types.VendorSpec{
		{Name: "single-vendor", URL: "https://github.com/test/repo", License: "MIT"},
	}

	lockMap := map[string]map[string]string{
		"single-vendor": {"main": "hash1"},
	}

	// Track how many workers actually ran
	var workerCount int
	var mu sync.Mutex

	syncFunc := func(_ types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		mu.Lock()
		workerCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return map[string]RefMetadata{"main": {CommitHash: "newhash"}}, CopyStats{FileCount: 1, ByteCount: 100}, nil
	}

	// Request 4 workers but only have 1 vendor
	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 4}, &SilentUICallback{})

	results, err := executor.ExecuteParallelSync(vendors, lockMap, SyncOptions{}, syncFunc)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Only 1 worker should have actually processed work
	if workerCount != 1 {
		t.Errorf("Expected 1 worker to process work, got %d", workerCount)
	}
}

// TestParallelExecutor_ForceOption tests that force option is applied
func TestParallelExecutor_ForceOption(t *testing.T) {
	vendors := []types.VendorSpec{
		{Name: "vendor-1", URL: "https://github.com/test/repo1", License: "MIT"},
	}

	lockMap := map[string]map[string]string{
		"vendor-1": {"main": "locked-hash"},
	}

	// Track what lockedRefs were passed to sync function
	var receivedLockedRefs map[string]string

	syncFunc := func(_ types.VendorSpec, lockedRefs map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		receivedLockedRefs = lockedRefs
		return map[string]RefMetadata{"main": {CommitHash: "new-hash"}}, CopyStats{}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 1}, &SilentUICallback{})

	// Execute with Force option
	_, err := executor.ExecuteParallelSync(vendors, lockMap, SyncOptions{Force: true}, syncFunc)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// When Force is true, lockedRefs should be nil (forces re-download)
	if receivedLockedRefs != nil {
		t.Errorf("Expected nil lockedRefs with Force option, got %v", receivedLockedRefs)
	}
}

// ============================================================================
// Parallel Executor Edge Cases
// ============================================================================

func TestParallelExecutor_MultipleVendorsFailing(t *testing.T) {
	// When multiple vendors fail, only the first error (by collection order) is returned,
	// but all results should still be present.
	vendors := []types.VendorSpec{
		{Name: "fail-1", URL: "https://github.com/test/f1", License: "MIT"},
		{Name: "fail-2", URL: "https://github.com/test/f2", License: "MIT"},
		{Name: "ok-3", URL: "https://github.com/test/ok3", License: "MIT"},
		{Name: "fail-4", URL: "https://github.com/test/f4", License: "MIT"},
	}

	syncFunc := func(v types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		time.Sleep(5 * time.Millisecond)
		if v.Name != "ok-3" {
			return nil, CopyStats{}, errors.New("sync error for " + v.Name)
		}
		return map[string]RefMetadata{"main": {CommitHash: "ok"}}, CopyStats{FileCount: 1}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})
	results, err := executor.ExecuteParallelSync(vendors, nil, SyncOptions{}, syncFunc)

	if err == nil {
		t.Fatal("Expected error when multiple vendors fail, got nil")
	}

	// All 4 results should be present even with errors
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// Count errors
	errorCount := 0
	for _, r := range results {
		if r.Error != nil {
			errorCount++
		}
	}
	if errorCount != 3 {
		t.Errorf("Expected 3 vendor errors, got %d", errorCount)
	}
}

func TestParallelExecutor_WorkerCountEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		maxWorkers  int
		wantWorkers int
	}{
		{name: "worker count 1", maxWorkers: 1, wantWorkers: 1},
		{name: "worker count 8 (cap)", maxWorkers: 8, wantWorkers: 8},
		{name: "worker count 9 (capped to 8)", maxWorkers: 9, wantWorkers: 8},
		{name: "worker count 1000 (capped to 8)", maxWorkers: 1000, wantWorkers: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: tt.maxWorkers}, &SilentUICallback{})
			if executor.maxWorkers != tt.wantWorkers {
				t.Errorf("maxWorkers = %d, want %d", executor.maxWorkers, tt.wantWorkers)
			}
		})
	}
}

func TestParallelExecutor_WorkersLimitedByVendorCount(t *testing.T) {
	// When vendor count < worker count, only vendor-count goroutines should process work.
	vendors := []types.VendorSpec{
		{Name: "only-one", URL: "https://github.com/test/repo", License: "MIT"},
	}

	var processedCount int
	var mu sync.Mutex

	syncFunc := func(_ types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		mu.Lock()
		processedCount++
		mu.Unlock()
		return map[string]RefMetadata{"main": {CommitHash: "h"}}, CopyStats{}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 8}, &SilentUICallback{})
	results, err := executor.ExecuteParallelSync(vendors, nil, SyncOptions{}, syncFunc)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if processedCount != 1 {
		t.Errorf("Expected 1 processed, got %d", processedCount)
	}
}

func TestParallelExecutor_NilLockMap(t *testing.T) {
	// Passing nil lockMap should not panic; lockedRefs should be nil for all vendors.
	vendors := []types.VendorSpec{
		{Name: "v1", URL: "https://github.com/test/v1", License: "MIT"},
	}

	var receivedLockedRefs map[string]string

	syncFunc := func(_ types.VendorSpec, lockedRefs map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		receivedLockedRefs = lockedRefs
		return map[string]RefMetadata{"main": {CommitHash: "h"}}, CopyStats{}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 1}, &SilentUICallback{})
	_, err := executor.ExecuteParallelSync(vendors, nil, SyncOptions{}, syncFunc)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedLockedRefs != nil {
		t.Errorf("Expected nil lockedRefs with nil lockMap, got %v", receivedLockedRefs)
	}
}

func TestParallelExecutor_UpdateMultipleFailures(t *testing.T) {
	// Test that ExecuteParallelUpdate handles multiple failures correctly.
	vendors := []types.VendorSpec{
		{Name: "upd-1", URL: "https://github.com/test/u1", License: "MIT"},
		{Name: "upd-2", URL: "https://github.com/test/u2", License: "MIT"},
		{Name: "upd-3", URL: "https://github.com/test/u3", License: "MIT"},
	}

	updateFunc := func(v types.VendorSpec, _ SyncOptions) (map[string]RefMetadata, error) {
		time.Sleep(5 * time.Millisecond)
		if v.Name == "upd-1" || v.Name == "upd-3" {
			return nil, errors.New("update failed: " + v.Name)
		}
		return map[string]RefMetadata{"main": {CommitHash: "ok"}}, nil
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})
	results, err := executor.ExecuteParallelUpdate(vendors, updateFunc)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// All results should be present
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Count errors
	errorCount := 0
	successCount := 0
	for _, r := range results {
		if r.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}
	if errorCount != 2 {
		t.Errorf("Expected 2 errors, got %d", errorCount)
	}
	if successCount != 1 {
		t.Errorf("Expected 1 success, got %d", successCount)
	}
}

func TestParallelExecutor_UpdateEmptyVendors(t *testing.T) {
	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})

	updateFunc := func(_ types.VendorSpec, _ SyncOptions) (map[string]RefMetadata, error) {
		t.Fatal("update function should not be called for empty vendor list")
		return nil, nil
	}

	results, err := executor.ExecuteParallelUpdate([]types.VendorSpec{}, updateFunc)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results, got %d results", len(results))
	}
}

func TestParallelExecutor_AllVendorsFail(t *testing.T) {
	// When every vendor fails, the returned error should wrap the first one collected.
	vendors := []types.VendorSpec{
		{Name: "bad-1", URL: "https://github.com/test/b1", License: "MIT"},
		{Name: "bad-2", URL: "https://github.com/test/b2", License: "MIT"},
	}

	syncFunc := func(v types.VendorSpec, _ map[string]string, _ SyncOptions) (map[string]RefMetadata, CopyStats, error) {
		return nil, CopyStats{}, errors.New("total failure: " + v.Name)
	}

	executor := NewParallelExecutor(types.ParallelOptions{MaxWorkers: 2}, &SilentUICallback{})
	results, err := executor.ExecuteParallelSync(vendors, nil, SyncOptions{}, syncFunc)

	if err == nil {
		t.Fatal("Expected error when all vendors fail")
	}

	// All results should still be returned
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Both should have errors
	for _, r := range results {
		if r.Error == nil {
			t.Errorf("Expected error for vendor %s", r.Vendor.Name)
		}
	}
}
