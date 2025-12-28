package core

import (
	"fmt"
	"runtime"
	"sync"

	"git-vendor/internal/types"
)

// VendorResult represents the result of processing a single vendor
type VendorResult struct {
	Vendor      types.VendorSpec
	UpdatedRefs map[string]string // ref -> commit hash
	Stats       CopyStats
	Error       error
}

// ParallelExecutor handles concurrent processing of multiple vendors
type ParallelExecutor struct {
	maxWorkers int
	ui         UICallback
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(opts types.ParallelOptions, ui UICallback) *ParallelExecutor {
	workers := opts.MaxWorkers
	if workers == 0 {
		workers = runtime.NumCPU()
	}
	// Limit to a reasonable maximum to avoid overwhelming the system
	if workers > 8 {
		workers = 8
	}

	return &ParallelExecutor{
		maxWorkers: workers,
		ui:         ui,
	}
}

// SyncVendorFunc is a function type that syncs a single vendor
type SyncVendorFunc func(v types.VendorSpec, lockedRefs map[string]string, opts SyncOptions) (map[string]string, CopyStats, error)

// ExecuteParallelSync processes vendors in parallel using a worker pool
func (p *ParallelExecutor) ExecuteParallelSync(
	vendors []types.VendorSpec,
	lockMap map[string]map[string]string,
	opts SyncOptions,
	syncFunc SyncVendorFunc,
) ([]VendorResult, error) {
	if len(vendors) == 0 {
		return nil, nil
	}

	// Determine worker count - don't use more workers than vendors
	workerCount := p.maxWorkers
	if workerCount > len(vendors) {
		workerCount = len(vendors)
	}

	// Create channels for work distribution
	jobs := make(chan types.VendorSpec, len(vendors))
	results := make(chan VendorResult, len(vendors))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go p.syncWorker(&wg, jobs, results, lockMap, opts, syncFunc)
	}

	// Send all vendors to the jobs channel
	for _, vendor := range vendors {
		jobs <- vendor
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []VendorResult
	var errors []error

	for result := range results {
		allResults = append(allResults, result)
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.Vendor.Name, result.Error))
		}
	}

	// If any errors occurred, return them
	if len(errors) > 0 {
		// Return first error for now (could be enhanced to return all errors)
		return allResults, errors[0]
	}

	return allResults, nil
}

// syncWorker is a worker goroutine that processes vendors from the jobs channel
func (p *ParallelExecutor) syncWorker(
	wg *sync.WaitGroup,
	jobs <-chan types.VendorSpec,
	results chan<- VendorResult,
	lockMap map[string]map[string]string,
	opts SyncOptions,
	syncFunc SyncVendorFunc,
) {
	defer wg.Done()

	for vendor := range jobs {
		// Get locked refs for this vendor (thread-safe read from lockMap)
		var lockedRefs map[string]string
		if lockMap != nil {
			lockedRefs = lockMap[vendor.Name]
		}

		// Apply force option
		if opts.Force {
			lockedRefs = nil
		}

		// Execute sync for this vendor
		updatedRefs, stats, err := syncFunc(vendor, lockedRefs, opts)

		// Send result
		results <- VendorResult{
			Vendor:      vendor,
			UpdatedRefs: updatedRefs,
			Stats:       stats,
			Error:       err,
		}
	}
}

// UpdateVendorFunc is a function type that updates a single vendor
type UpdateVendorFunc func(v types.VendorSpec, opts SyncOptions) (map[string]string, error)

// ExecuteParallelUpdate processes vendor updates in parallel
func (p *ParallelExecutor) ExecuteParallelUpdate(
	vendors []types.VendorSpec,
	updateFunc UpdateVendorFunc,
) ([]VendorResult, error) {
	if len(vendors) == 0 {
		return nil, nil
	}

	// Determine worker count
	workerCount := p.maxWorkers
	if workerCount > len(vendors) {
		workerCount = len(vendors)
	}

	// Create channels
	jobs := make(chan types.VendorSpec, len(vendors))
	results := make(chan VendorResult, len(vendors))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go p.updateWorker(&wg, jobs, results, updateFunc)
	}

	// Send jobs
	for _, vendor := range vendors {
		jobs <- vendor
	}
	close(jobs)

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []VendorResult
	var errors []error

	for result := range results {
		allResults = append(allResults, result)
		if result.Error != nil {
			errors = append(errors, fmt.Errorf("%s: %w", result.Vendor.Name, result.Error))
		}
	}

	if len(errors) > 0 {
		return allResults, errors[0]
	}

	return allResults, nil
}

// updateWorker processes vendor updates
func (p *ParallelExecutor) updateWorker(
	wg *sync.WaitGroup,
	jobs <-chan types.VendorSpec,
	results chan<- VendorResult,
	updateFunc UpdateVendorFunc,
) {
	defer wg.Done()

	for vendor := range jobs {
		// Update this vendor (force=true, no-cache=true for updates)
		updatedRefs, err := updateFunc(vendor, SyncOptions{Force: true, NoCache: true})

		results <- VendorResult{
			Vendor:      vendor,
			UpdatedRefs: updatedRefs,
			Error:       err,
		}
	}
}
