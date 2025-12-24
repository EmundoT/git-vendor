# Phase 8: Advanced Features & Optimizations

**Prerequisites:** Phases 5-7 complete (CI/CD, multi-platform, enhanced testing)
**Goal:** Add power-user features and performance optimizations
**Priority:** OPTIONAL - Nice-to-have features for advanced use cases
**Estimated Effort:** 12-16 hours

---

## Current State

**What Works:**
- ✅ Basic vendor management (add, sync, update, remove)
- ✅ Deterministic lockfile-based versioning
- ✅ Multi-platform support (GitHub, GitLab, Bitbucket)
- ✅ CI/CD automation
- ✅ Comprehensive testing

**Feature Gaps:**
- ❌ No notification when dependencies are outdated
- ❌ Sequential processing (slow with many vendors)
- ❌ Always full re-sync (inefficient for large vendored files)
- ❌ No vendor groups for batch operations
- ❌ No custom hooks for automation
- ❌ No progress indicators during long operations

---

## Goals

1. **Dependency Update Checker** - Notify when vendor commits are stale
2. **Parallel Vendor Processing** - Speed up multi-vendor operations
3. **Incremental Sync** - Skip re-download if already up-to-date
4. **Vendor Groups** - Batch operations on related vendors
5. **Custom Hooks** - Pre/post sync automation
6. **Progress Indicators** - Real-time operation feedback
7. **Advanced CLI Features** - Quality-of-life improvements

---

## Implementation Steps

### 1. Dependency Update Checker

Create `internal/core/update_checker.go`:

```go
package core

import (
	"fmt"
	"time"

	"git-vendor/internal/types"
)

// CheckForUpdates compares lockfile commit hashes with latest available
func (s *VendorSyncer) CheckForUpdates() ([]types.UpdateInfo, error) {
	config, err := s.configStore.Load()
	if err != nil {
		return nil, err
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		return nil, err
	}

	var updates []types.UpdateInfo

	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			lockedHash := s.GetLockHash(vendor.Name, spec.Ref)
			if lockedHash == "" {
				continue // Not synced yet
			}

			// Fetch latest commit on ref
			tempDir, err := s.fs.CreateTemp("", "update-check-*")
			if err != nil {
				continue
			}
			defer s.fs.RemoveAll(tempDir)

			if err := s.gitClient.Init(tempDir); err != nil {
				continue
			}

			if err := s.gitClient.AddRemote(tempDir, "origin", vendor.URL); err != nil {
				continue
			}

			// Fetch just the ref we need
			if err := s.gitClient.Fetch(tempDir, 1, spec.Ref); err != nil {
				continue
			}

			latestHash, err := s.gitClient.GetHeadHash(tempDir)
			if err != nil {
				continue
			}

			if latestHash != lockedHash {
				// Get lock details for timestamp
				lockEntry := s.getLockEntry(lock, vendor.Name, spec.Ref)

				updates = append(updates, types.UpdateInfo{
					VendorName:   vendor.Name,
					Ref:          spec.Ref,
					CurrentHash:  lockedHash,
					LatestHash:   latestHash,
					LastUpdated:  lockEntry.Updated,
					BehindBy:     "unknown", // Could add commit count if needed
				})
			}
		}
	}

	return updates, nil
}

func (s *VendorSyncer) getLockEntry(lock types.VendorLock, name, ref string) types.LockDetails {
	for _, entry := range lock.Vendors {
		if entry.Name == name && entry.Ref == ref {
			return entry
		}
	}
	return types.LockDetails{}
}
```

Add to `internal/types/types.go`:

```go
// UpdateInfo represents available update for a vendor
type UpdateInfo struct {
	VendorName  string
	Ref         string
	CurrentHash string
	LatestHash  string
	LastUpdated string
	BehindBy    string
}
```

Add CLI command in `main.go`:

```go
case "check-updates":
	updates, err := manager.CheckForUpdates()
	if err != nil {
		tui.PrintError("Error", err.Error())
		return
	}

	if len(updates) == 0 {
		fmt.Println("✓ All vendors are up-to-date")
		return
	}

	fmt.Printf("Found %d update(s) available:\n\n", len(updates))
	for _, u := range updates {
		fmt.Printf("  %s @ %s\n", u.VendorName, u.Ref)
		fmt.Printf("    Current: %s\n", u.CurrentHash[:7])
		fmt.Printf("    Latest:  %s\n", u.LatestHash[:7])
		fmt.Printf("    Updated: %s\n\n", u.LastUpdated)
	}

	fmt.Println("Run 'git-vendor update' to fetch latest versions")
```

### 2. Parallel Vendor Processing

Update `internal/core/update_service.go`:

```go
func (s *VendorSyncer) UpdateAllParallel() error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	if len(config.Vendors) == 0 {
		return fmt.Errorf("no vendors configured")
	}

	// Use worker pool pattern
	type result struct {
		vendor types.VendorSpec
		refs   map[string]string
		err    error
	}

	numWorkers := min(len(config.Vendors), 5) // Max 5 concurrent syncs
	jobs := make(chan types.VendorSpec, len(config.Vendors))
	results := make(chan result, len(config.Vendors))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vendor := range jobs {
				updatedRefs, err := s.syncVendor(vendor, nil)
				results <- result{
					vendor: vendor,
					refs:   updatedRefs,
					err:    err,
				}
			}
		}()
	}

	// Send jobs
	for _, vendor := range config.Vendors {
		jobs <- vendor
	}
	close(jobs)

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var newLock types.VendorLock
	var errors []error

	for res := range results {
		if res.err != nil {
			errors = append(errors, res.err)
			continue
		}

		for ref, hash := range res.refs {
			newLock.Vendors = append(newLock.Vendors, types.LockDetails{
				Name:       res.vendor.Name,
				Ref:        ref,
				CommitHash: hash,
				Updated:    time.Now().Format(time.RFC3339),
			})
		}
	}

	// Save lockfile
	if err := s.lockStore.Save(newLock); err != nil {
		return fmt.Errorf("failed to save lockfile: %w", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to sync %d vendor(s)", len(errors))
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

Add flag to use parallel mode:

```bash
git-vendor update --parallel
```

### 3. Incremental Sync

Update `internal/core/sync_service.go`:

```go
func (s *VendorSyncer) SyncIncremental(vendorName string) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		// No lockfile, do full sync
		return s.SyncWithOptions(vendorName, false)
	}

	vendor := findVendor(config, vendorName)
	if vendor == nil {
		return fmt.Errorf("vendor '%s' not found", vendorName)
	}

	for _, spec := range vendor.Specs {
		lockedHash := s.GetLockHash(vendor.Name, spec.Ref)
		if lockedHash == "" {
			// Not synced yet, do full sync
			continue
		}

		// Check if files need updating
		needsUpdate, err := s.checkIfNeedsUpdate(*vendor, spec, lockedHash)
		if err != nil {
			return err
		}

		if !needsUpdate {
			s.ui.Notify(fmt.Sprintf("✓ %s @ %s is up-to-date, skipping", vendor.Name, spec.Ref))
			continue
		}

		// Sync only if needed
		if _, err := s.syncVendor(*vendor, map[string]string{spec.Ref: lockedHash}); err != nil {
			return err
		}
	}

	return nil
}

func (s *VendorSyncer) checkIfNeedsUpdate(vendor types.VendorSpec, spec types.BranchSpec, lockedHash string) (bool, error) {
	// For each mapping, check if destination file exists and has correct content
	for _, mapping := range spec.Mapping {
		destPath := mapping.To
		if destPath == "" {
			destPath = filepath.Join(s.rootDir, filepath.Base(mapping.From))
		} else {
			destPath = filepath.Join(s.rootDir, destPath)
		}

		// If destination doesn't exist, needs update
		if _, err := s.fs.Stat(destPath); os.IsNotExist(err) {
			return true, nil
		}

		// Could add content hash checking here for more robust detection
	}

	return false, nil
}
```

### 4. Vendor Groups

Add to `internal/types/types.go`:

```go
// VendorGroup allows batching operations on related vendors
type VendorGroup struct {
	Name    string   `yaml:"name"`
	Vendors []string `yaml:"vendors"`
}

type VendorConfig struct {
	Vendors []VendorSpec  `yaml:"vendors"`
	Groups  []VendorGroup `yaml:"groups,omitempty"`
}
```

Add group operations in `internal/core/vendor_groups.go`:

```go
package core

func (s *VendorSyncer) SyncGroup(groupName string) error {
	config, err := s.configStore.Load()
	if err != nil {
		return err
	}

	var group *types.VendorGroup
	for _, g := range config.Groups {
		if g.Name == groupName {
			group = &g
			break
		}
	}

	if group == nil {
		return fmt.Errorf("group '%s' not found", groupName)
	}

	for _, vendorName := range group.Vendors {
		if err := s.SyncWithOptions(vendorName, false); err != nil {
			return fmt.Errorf("failed to sync %s: %w", vendorName, err)
		}
	}

	return nil
}
```

Usage in vendor.yml:

```yaml
vendors:
  - name: utils
    url: https://github.com/owner/utils
    # ...
  - name: helpers
    url: https://github.com/owner/helpers
    # ...

groups:
  - name: core-libs
    vendors:
      - utils
      - helpers
```

Command:

```bash
git-vendor sync --group core-libs
```

### 5. Custom Hooks

Add to `internal/types/types.go`:

```go
type VendorSpec struct {
	Name    string       `yaml:"name"`
	URL     string       `yaml:"url"`
	License string       `yaml:"license"`
	Specs   []BranchSpec `yaml:"specs"`
	Hooks   *Hooks       `yaml:"hooks,omitempty"`
}

type Hooks struct {
	PreSync  string `yaml:"pre-sync,omitempty"`
	PostSync string `yaml:"post-sync,omitempty"`
}
```

Implement hook execution in `internal/core/hooks.go`:

```go
package core

import (
	"fmt"
	"os/exec"
)

func (s *VendorSyncer) executeHook(hook, vendorName string) error {
	if hook == "" {
		return nil
	}

	s.ui.Notify(fmt.Sprintf("Running hook: %s", hook))

	cmd := exec.Command("sh", "-c", hook)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("VENDOR_NAME=%s", vendorName),
		fmt.Sprintf("VENDOR_DIR=%s", s.rootDir),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook failed: %w\n%s", err, output)
	}

	return nil
}
```

Usage in vendor.yml:

```yaml
vendors:
  - name: frontend-lib
    url: https://github.com/owner/lib
    license: MIT
    hooks:
      pre-sync: "echo 'About to sync frontend-lib'"
      post-sync: "npm run build-vendor"
    specs:
      - ref: main
        mapping:
          - from: src/
            to: vendor/frontend-lib/
```

### 6. Progress Indicators

Install bubbletea for progress UI:

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
```

Create `internal/tui/progress.go`:

```go
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type progressModel struct {
	spinner  spinner.Model
	progress progress.Model
	vendors  []string
	current  int
	done     bool
}

func NewProgressIndicator(vendors []string) *tea.Program {
	s := spinner.New()
	s.Spinner = spinner.Dot

	p := progress.New(progress.WithDefaultGradient())

	model := progressModel{
		spinner:  s,
		progress: p,
		vendors:  vendors,
		current:  0,
		done:     false,
	}

	return tea.NewProgram(model)
}

func (m progressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case vendorCompleteMsg:
		m.current++
		if m.current >= len(m.vendors) {
			m.done = true
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

func (m progressModel) View() string {
	if m.done {
		return "✓ All vendors synced\n"
	}

	percent := float64(m.current) / float64(len(m.vendors))
	progressBar := m.progress.ViewAs(percent)

	currentVendor := ""
	if m.current < len(m.vendors) {
		currentVendor = m.vendors[m.current]
	}

	return fmt.Sprintf(
		"%s Syncing vendors... (%d/%d)\n\n%s\n\n%s %s\n",
		m.spinner.View(),
		m.current,
		len(m.vendors),
		progressBar,
		m.spinner.View(),
		currentVendor,
	)
}

type vendorCompleteMsg struct{}
```

Use in sync operations:

```go
if !quietMode {
	prog := tui.NewProgressIndicator(vendorNames)
	go prog.Start()
	defer prog.Quit()
}
```

### 7. Advanced CLI Features

Add to `main.go`:

```go
// Auto-complete support
case "completion":
	shell := "bash"
	if len(os.Args) > 2 {
		shell = os.Args[2]
	}
	generateCompletion(shell)

// Diff command
case "diff":
	if len(os.Args) < 3 {
		fmt.Println("Usage: git-vendor diff <vendor>")
		return
	}
	showVendorDiff(os.Args[2])

// Watch for updates
case "watch":
	interval := 1 * time.Hour
	if len(os.Args) > 2 {
		// Parse interval flag
	}
	watchForUpdates(manager, interval)
```

Implement features:

```go
func generateCompletion(shell string) {
	// Generate shell completion script
	// For bash, zsh, fish, powershell
}

func showVendorDiff(vendorName string) {
	// Show diff between current files and locked version
	// Use git diff with vendored files
}

func watchForUpdates(manager *core.Manager, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			updates, _ := manager.CheckForUpdates()
			if len(updates) > 0 {
				// Notify user (desktop notification, email, etc.)
			}
		}
	}
}
```

---

## Verification Checklist

After implementing Phase 8, verify:

- [ ] `git-vendor check-updates` shows available updates
- [ ] `git-vendor update --parallel` speeds up multi-vendor sync
- [ ] Incremental sync skips unchanged files
- [ ] Vendor groups work: `git-vendor sync --group <name>`
- [ ] Pre/post sync hooks execute correctly
- [ ] Progress indicators show during sync
- [ ] Diff command works
- [ ] Shell completion generated
- [ ] All features documented in README

---

## Expected Outcomes

**After Phase 8:**
- ✅ Dependency staleness detection
- ✅ 3-5x faster multi-vendor operations (parallel)
- ✅ Reduced redundant downloads (incremental)
- ✅ Batch operations via groups
- ✅ Automation via hooks
- ✅ Better UX with progress indicators
- ✅ Professional CLI experience

**Performance Metrics:**
- Update check: <30s for 10 vendors
- Parallel sync: 3-5x faster than sequential
- Incremental sync: 10x faster when up-to-date
- Hook execution: <100ms overhead

---

## Next Steps

After Phase 8 completion:
- **Project is feature-complete** for most use cases
- **Optional:** Add web UI for non-CLI users
- **Optional:** Plugin system for custom providers
- **Optional:** Vendor marketplace/registry

**Phase 8 represents the advanced feature set.** The tool is production-ready after Phase 5, with Phases 6-8 adding power-user features.
