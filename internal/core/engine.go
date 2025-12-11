package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"encoding/json"
	"git-vendor/internal/tui"
	"git-vendor/internal/types"

	"gopkg.in/yaml.v3"
)

// Constants for the new "Clean Root" structure
const (
	VendorDir   = "vendor"
	ConfigName  = "vendor.yml"
	LockName    = "vendor.lock"
	LicenseDir  = "licenses"
)

// License constants
var AllowedLicenses = []string{
	"MIT",
	"Apache-2.0",
	"BSD-3-Clause",
	"BSD-2-Clause",
	"ISC",
	"Unlicense",
	"CC0-1.0",
}

// Error messages
const (
	ErrStaleCommitMsg = "locked commit %s no longer exists in the repository.\n\nThis usually happens when the remote repository has been force-pushed or the commit was deleted.\nRun 'git-vendor update' to fetch the latest commit and update the lockfile, then try syncing again"
	ErrCheckoutFailed = "checkout locked hash %s failed: %w"
	ErrRefCheckoutFailed = "checkout ref %s failed: %w"
	ErrPathNotFound = "path '%s' not found"
	ErrInvalidURL = "invalid url"
	ErrVendorNotFound = "vendor '%s' not found"
	ErrComplianceFailed = "compliance check failed"
)

// License file names
var LicenseFileNames = []string{"LICENSE", "LICENSE.txt", "COPYING"}

type Manager struct {
	RootDir string
}

func NewManager() *Manager {
	return &Manager{RootDir: VendorDir}
}

func (m *Manager) ConfigPath() string { return filepath.Join(m.RootDir, ConfigName) }
func (m *Manager) LockPath() string   { return filepath.Join(m.RootDir, LockName) }
func (m *Manager) LicensePath(name string) string { 
	return filepath.Join(m.RootDir, LicenseDir, name+".txt") 
}

func IsGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func (m *Manager) Init() {
	if _, err := os.Stat(m.ConfigPath()); err == nil { return }
	os.MkdirAll(m.RootDir, 0755)
	os.MkdirAll(filepath.Join(m.RootDir, LicenseDir), 0755)
	m.saveConfig(types.VendorConfig{})
}

func (m *Manager) ParseSmartURL(rawURL string) (string, string, string) {
	rawURL = cleanURL(rawURL)
	reDeep := regexp.MustCompile(`(github\.com/[^/]+/[^/]+)/(blob|tree)/([^/]+)/(.+)`)
	matches := reDeep.FindStringSubmatch(rawURL)
	if len(matches) == 5 {
		return "https://" + matches[1], matches[3], matches[4]
	}
	base := strings.TrimSuffix(rawURL, "/")
	base = strings.TrimSuffix(base, ".git")
	return base, "", ""
}

func (m *Manager) FetchRepoDir(url, ref, subdir string) ([]string, error) {
	// Create context with 30 second timeout for directory listing
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "git-vendor-index-*")
	if err != nil { return nil, err }
	defer os.RemoveAll(tempDir)

	err = runGitWithContext(ctx, tempDir, "clone", "--filter=blob:none", "--no-checkout", "--depth", "1", url, ".")
	if err != nil { return nil, err }

	if ref != "" && ref != "HEAD" {
		runGitWithContext(ctx, tempDir, "fetch", "origin", ref)
	}
	target := ref
	if target == "" { target = "HEAD" }

	cmd := exec.CommandContext(ctx, "git", "ls-tree", target)
	if subdir != "" && subdir != "." {
		cleanSub := strings.TrimSuffix(subdir, "/")
		cmd.Args = append(cmd.Args, cleanSub + "/")
	}
	cmd.Dir = tempDir
	out, err := cmd.Output()
	if err != nil && subdir != "" {
		cmd = exec.CommandContext(ctx, "git", "ls-tree", target, strings.TrimSuffix(subdir, "/"))
		cmd.Dir = tempDir
		out, err = cmd.Output()
	}
	if err != nil { return nil, fmt.Errorf("git ls-tree failed: %w", err) }

	lines := strings.Split(string(out), "\n")
	var items []string
	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) < 4 { continue }
		objType := parts[1]
		fullPath := strings.Join(parts[3:], " ")

		relName := fullPath
		if subdir != "" && subdir != "." {
			cleanSub := strings.TrimSuffix(subdir, "/") + "/"
			if !strings.HasPrefix(fullPath, cleanSub) { continue }
			relName = strings.TrimPrefix(fullPath, cleanSub)
		}
		if relName == "" { continue }
		if objType == "tree" { items = append(items, relName+"/") } else { items = append(items, relName) }
	}
	sort.Strings(items)
	return items, nil
}

func (m *Manager) ListLocalDir(path string) ([]string, error) {
	if path == "" { path = "." }
	entries, err := os.ReadDir(path)
	if err != nil { return nil, err }

	var items []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() { name += "/" }
		items = append(items, name)
	}
	sort.Strings(items)
	return items, nil
}

func (m *Manager) RemoveVendor(name string) error {
	config, err := m.loadConfig()
	if err != nil { return err }
	found := -1
	for i, v := range config.Vendors {
		if v.Name == name { found = i; break }
	}
	if found == -1 { return fmt.Errorf(ErrVendorNotFound, name) }
	config.Vendors = append(config.Vendors[:found], config.Vendors[found+1:]...)
	
	os.Remove(m.LicensePath(name))

	if err := m.saveConfig(config); err != nil { return err }
	return m.UpdateAll()
}

func (m *Manager) SaveVendor(spec types.VendorSpec) error {
	config, _ := m.loadConfig()
	found := false
	for i, v := range config.Vendors {
		if v.Name == spec.Name {
			config.Vendors[i] = spec
			found = true
			break
		}
	}
	if !found { config.Vendors = append(config.Vendors, spec) }
	if err := m.saveConfig(config); err != nil { return err }
	return m.UpdateAll()
}

func (m *Manager) AddVendor(spec types.VendorSpec) error {
	config, _ := m.loadConfig()
	found := false
	// FIX: Use _ instead of i since we don't need the index
	for _, v := range config.Vendors {
		if v.Name == spec.Name { found = true; break }
	}
	if !found {
		detectedLicense, err := m.CheckGitHubLicense(spec.URL)
		if err == nil {
			spec.License = detectedLicense
		} else {
			spec.License = "UNKNOWN"
		}
		if !m.isLicenseAllowed(spec.License) {
			if !tui.AskToOverrideCompliance(spec.License) {
				return fmt.Errorf(ErrComplianceFailed)
			}
		} else {
			tui.PrintComplianceSuccess(spec.License)
		}
	}
	return m.SaveVendor(spec)
}

func (m *Manager) Sync() error {
	return m.SyncWithOptions("", false)
}

func (m *Manager) SyncDryRun() error {
	return m.sync(true, "", false)
}

func (m *Manager) SyncWithOptions(vendorName string, force bool) error {
	return m.sync(false, vendorName, force)
}

func (m *Manager) sync(dryRun bool, vendorName string, force bool) error {
	config, err := m.loadConfig()
	if err != nil { return err }

	lock, err := m.loadLock()
	if err != nil || len(lock.Vendors) == 0 {
		if dryRun {
			fmt.Println("No lockfile found. Would run update to create lockfile.")
			return nil
		}
		fmt.Println("No lockfile found. Running update...")
		return m.UpdateAll()
	}

	lockMap := make(map[string]map[string]string)
	for _, l := range lock.Vendors {
		if lockMap[l.Name] == nil { lockMap[l.Name] = make(map[string]string) }
		lockMap[l.Name][l.Ref] = l.CommitHash
	}

	if dryRun {
		fmt.Println(tui.StyleTitle("Sync Plan:"))
		fmt.Println()
	}

	for _, v := range config.Vendors {
		// Skip vendors that don't match the filter
		if vendorName != "" && v.Name != vendorName {
			continue
		}

		v.URL = cleanURL(v.URL)
		if dryRun {
			m.previewSyncVendor(v, lockMap[v.Name])
		} else {
			// If force is true, pass nil to ignore lock and re-download
			refs := lockMap[v.Name]
			if force {
				refs = nil
			}
			if _, err := m.syncVendor(v, refs); err != nil {
				return err
			}
		}
	}

	// If vendorName was specified but not found, return error
	if vendorName != "" {
		found := false
		for _, v := range config.Vendors {
			if v.Name == vendorName {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf(ErrVendorNotFound, vendorName)
		}
	}

	return nil
}

func (m *Manager) previewSyncVendor(v types.VendorSpec, lockedRefs map[string]string) {
	fmt.Printf("✓ %s\n", v.Name)

	for _, spec := range v.Specs {
		status := "not synced"
		if lockedRefs != nil {
			if h, ok := lockedRefs[spec.Ref]; ok && h != "" {
				status = fmt.Sprintf("locked: %s", h[:7])
			}
		}

		fmt.Printf("  @ %s (%s)\n", spec.Ref, status)

		if len(spec.Mapping) == 0 {
			fmt.Printf("    (no paths configured)\n")
		} else {
			for _, m := range spec.Mapping {
				dest := m.To
				if dest == "" {
					dest = "(auto)"
				}
				fmt.Printf("    → %s → %s\n", m.From, dest)
			}
		}
	}
	fmt.Println()
}

func (m *Manager) UpdateAll() error {
	config, err := m.loadConfig()
	if err != nil { return err }

	lock := types.VendorLock{}

	for _, v := range config.Vendors {
		v.URL = cleanURL(v.URL)

		updatedRefs, err := m.syncVendor(v, nil)
		if err != nil {
			tui.PrintError("Update Failed", fmt.Sprintf("%s: %v", v.Name, err))
			continue
		}

		for ref, hash := range updatedRefs {
			licenseFile := m.LicensePath(v.Name)

			lock.Vendors = append(lock.Vendors, types.LockDetails{
				Name: v.Name, Ref: ref, CommitHash: hash,
				LicensePath: licenseFile,
				Updated: time.Now().Format(time.RFC3339),
			})

			tui.PrintSuccess(fmt.Sprintf("Updated %s @ %s to commit %s", v.Name, ref, hash[:7]))
		}
	}
	return m.saveLock(lock)
}

func (m *Manager) syncVendor(v types.VendorSpec, lockedRefs map[string]string) (map[string]string, error) {
	fmt.Printf("  • Processing %s...\n", v.Name)
	
	tempDir, err := os.MkdirTemp("", "git-vendor-*")
	if err != nil { return nil, err }
	defer os.RemoveAll(tempDir) 

	runGit(tempDir, "init")
	runGit(tempDir, "remote", "add", "origin", v.URL)
	
	results := make(map[string]string)

	for _, spec := range v.Specs {
		targetCommit := ""
		isLocked := false
		
		if lockedRefs != nil {
			if h, ok := lockedRefs[spec.Ref]; ok && h != "" {
				targetCommit = h
				isLocked = true
			}
		}

		if isLocked {
			// Try shallow fetch first, fall back to full fetch if needed
			if err := runGit(tempDir, "fetch", "--depth", "1", "origin", spec.Ref); err != nil {
				// Shallow fetch failed, try full fetch
				if err := runGit(tempDir, "fetch", "origin"); err != nil {
					return nil, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
				}
			}
			if err := runGit(tempDir, "checkout", targetCommit); err != nil {
				// Detect stale lock hash error and provide helpful message
				errMsg := err.Error()
				if strings.Contains(errMsg, "reference is not a tree") || strings.Contains(errMsg, "not a valid object") {
					return nil, fmt.Errorf(ErrStaleCommitMsg, targetCommit[:7])
				}
				return nil, fmt.Errorf(ErrCheckoutFailed, targetCommit, err)
			}
		} else {
			// Try shallow fetch first, fall back to full fetch if needed
			if err := runGit(tempDir, "fetch", "--depth", "1", "origin", spec.Ref); err != nil {
				// Shallow fetch failed, try full fetch
				if err := runGit(tempDir, "fetch", "origin"); err != nil {
					return nil, fmt.Errorf("failed to fetch ref %s: %w", spec.Ref, err)
				}
			}
			if err := runGit(tempDir, "checkout", "FETCH_HEAD"); err != nil {
				if err := runGit(tempDir, "checkout", spec.Ref); err != nil {
					return nil, fmt.Errorf(ErrRefCheckoutFailed, spec.Ref, err)
				}
			}
		}

		hash, _ := getHeadHash(tempDir)
		results[spec.Ref] = hash

		// License Automation
		var licenseSrc string
		for _, name := range LicenseFileNames {
			path := filepath.Join(tempDir, name)
			if _, err := os.Stat(path); err == nil {
				licenseSrc = path
				break
			}
		}

		if _, err := os.Stat(licenseSrc); err == nil {
			os.MkdirAll(filepath.Join(m.RootDir, LicenseDir), 0755)
			dest := m.LicensePath(v.Name)
			if err := copyFile(licenseSrc, dest); err != nil {
				return nil, fmt.Errorf("failed to copy license from %s to %s: %w", licenseSrc, dest, err)
			}
		}

		for _, mapping := range spec.Mapping {
			srcClean := strings.Replace(mapping.From, "blob/"+spec.Ref+"/", "", 1)
			srcClean = strings.Replace(srcClean, "tree/"+spec.Ref+"/", "", 1)
			
			srcPath := filepath.Join(tempDir, srcClean)
			destPath := mapping.To
			
			if destPath == "" || destPath == "." {
				if spec.DefaultTarget != "" {
					destPath = filepath.Join(spec.DefaultTarget, filepath.Base(srcClean))
				} else {
					destPath = filepath.Base(srcClean)
					if destPath == "." || destPath == "/" { destPath = v.Name }
				}
			}

			info, err := os.Stat(srcPath)
			if err != nil {
				return nil, fmt.Errorf(ErrPathNotFound, srcClean)
			}

			if info.IsDir() {
				os.MkdirAll(destPath, 0755)
				if err := copyDir(srcPath, destPath); err != nil {
					return nil, fmt.Errorf("failed to copy directory %s to %s: %w", srcPath, destPath, err)
				}
			} else {
				os.MkdirAll(filepath.Dir(destPath), 0755)
				if err := copyFile(srcPath, destPath); err != nil {
					return nil, fmt.Errorf("failed to copy file %s to %s: %w", srcPath, destPath, err)
				}
			}
		}
	}
	
	return results, nil
}

// Helpers
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil { return fmt.Errorf("%s", string(output)) }
	return nil
}

func runGitWithContext(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil { return fmt.Errorf("%s", string(output)) }
	return nil
}
func getHeadHash(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil { return "", err }
	return strings.TrimSpace(string(out)), nil
}
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil { return err }
	defer source.Close()
	dest, err := os.Create(dst)
	if err != nil { return err }
	defer dest.Close()
	_, err = io.Copy(dest, source)
	return err
}
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		relPath, _ := filepath.Rel(src, path)
		if strings.Contains(relPath, ".git") { return nil }
		destPath := filepath.Join(dst, relPath)
		if info.IsDir() { return os.MkdirAll(destPath, info.Mode()) }
		return copyFile(path, destPath)
	})
}
func cleanURL(raw string) string { return strings.TrimLeft(strings.TrimSpace(raw), "\\") }
func (m *Manager) CheckGitHubLicense(rawURL string) (string, error) {
	clean := cleanURL(rawURL)
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/\.]+)(\.git)?`)
	matches := re.FindStringSubmatch(clean)
	if len(matches) < 3 { return "", fmt.Errorf(ErrInvalidURL) }
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/license", matches[1], matches[2])
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "git-vendor-cli")
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode == 404 { return "NONE", nil }
	var res struct { License struct { SpdxID string `json:"spdx_id"` } `json:"license"` }
	json.NewDecoder(resp.Body).Decode(&res)
	if res.License.SpdxID == "" { return "UNKNOWN", nil }
	return res.License.SpdxID, nil
}
func (m *Manager) isLicenseAllowed(license string) bool {
	for _, l := range AllowedLicenses {
		if license == l {
			return true
		}
	}
	return false
}
func (m *Manager) GetConfig() (types.VendorConfig, error) { return m.loadConfig() }

func (m *Manager) GetLockHash(vendorName, ref string) string {
	lock, err := m.loadLock()
	if err != nil { return "" }
	for _, l := range lock.Vendors {
		if l.Name == vendorName && l.Ref == ref {
			return l.CommitHash
		}
	}
	return ""
}
func (m *Manager) loadConfig() (types.VendorConfig, error) {
	data, err := os.ReadFile(m.ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return types.VendorConfig{}, nil // OK: file doesn't exist yet
		}
		return types.VendorConfig{}, fmt.Errorf("failed to read vendor.yml: %w", err)
	}
	var cfg types.VendorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return types.VendorConfig{}, fmt.Errorf("invalid vendor.yml: %w", err)
	}
	return cfg, nil
}
func (m *Manager) saveConfig(cfg types.VendorConfig) error {
	data, _ := yaml.Marshal(cfg)
	return os.WriteFile(m.ConfigPath(), data, 0644)
}
func (m *Manager) loadLock() (types.VendorLock, error) {
	data, err := os.ReadFile(m.LockPath())
	if err != nil { return types.VendorLock{}, err }
	var lock types.VendorLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return types.VendorLock{}, fmt.Errorf("invalid vendor.lock: %w", err)
	}
	return lock, nil
}
func (m *Manager) saveLock(lock types.VendorLock) error {
	data, _ := yaml.Marshal(lock)
	return os.WriteFile(m.LockPath(), data, 0644)
}
func (m *Manager) Audit() {
	lock, err := m.loadLock()
	if err != nil { tui.PrintWarning("Audit Failed", "No lockfile."); return }
	tui.PrintSuccess(fmt.Sprintf("Audit Passed. %d vendors locked.", len(lock.Vendors)))
}

// DetectConflicts checks for path conflicts between vendors
func (m *Manager) DetectConflicts() ([]types.PathConflict, error) {
	config, err := m.loadConfig()
	if err != nil {
		return nil, err
	}

	var conflicts []types.PathConflict

	// Build a map of destination paths to vendor+mapping
	type PathOwner struct {
		VendorName string
		Mapping    types.PathMapping
		Ref        string
	}
	pathMap := make(map[string][]PathOwner)

	for _, vendor := range config.Vendors {
		for _, spec := range vendor.Specs {
			for _, mapping := range spec.Mapping {
				destPath := mapping.To

				// Handle auto-naming
				if destPath == "" || destPath == "." {
					if spec.DefaultTarget != "" {
						destPath = filepath.Join(spec.DefaultTarget, filepath.Base(mapping.From))
					} else {
						destPath = filepath.Base(mapping.From)
					}
				}

				// Normalize path
				destPath = filepath.Clean(destPath)

				pathMap[destPath] = append(pathMap[destPath], PathOwner{
					VendorName: vendor.Name,
					Mapping:    mapping,
					Ref:        spec.Ref,
				})
			}
		}
	}

	// Check for conflicts
	for path, owners := range pathMap {
		if len(owners) > 1 {
			// Multiple vendors map to the same path
			for i := 0; i < len(owners)-1; i++ {
				for j := i + 1; j < len(owners); j++ {
					conflicts = append(conflicts, types.PathConflict{
						Path:     path,
						Vendor1:  owners[i].VendorName,
						Vendor2:  owners[j].VendorName,
						Mapping1: owners[i].Mapping,
						Mapping2: owners[j].Mapping,
					})
				}
			}
		}
	}

	// Also check for overlapping directory paths (e.g., "src" and "src/components")
	var allPaths []string
	for path := range pathMap {
		allPaths = append(allPaths, path)
	}

	for i := 0; i < len(allPaths)-1; i++ {
		for j := i + 1; j < len(allPaths); j++ {
			path1 := allPaths[i]
			path2 := allPaths[j]

			// Check if one path is a subdirectory of another
			if isSubPath(path1, path2) {
				owners1 := pathMap[path1]
				owners2 := pathMap[path2]

				// Only report if different vendors
				if owners1[0].VendorName != owners2[0].VendorName {
					conflicts = append(conflicts, types.PathConflict{
						Path:     fmt.Sprintf("%s overlaps with %s", path1, path2),
						Vendor1:  owners1[0].VendorName,
						Vendor2:  owners2[0].VendorName,
						Mapping1: owners1[0].Mapping,
						Mapping2: owners2[0].Mapping,
					})
				}
			}
		}
	}

	return conflicts, nil
}

// isSubPath checks if path1 is a subdirectory of path2 or vice versa
func isSubPath(path1, path2 string) bool {
	path1 = filepath.Clean(path1)
	path2 = filepath.Clean(path2)

	// Check if path2 is under path1
	rel, err := filepath.Rel(path1, path2)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return true
	}

	// Check if path1 is under path2
	rel, err = filepath.Rel(path2, path1)
	if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
		return true
	}

	return false
}

// ValidateConfig performs comprehensive config validation
func (m *Manager) ValidateConfig() error {
	config, err := m.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check for empty vendors
	if len(config.Vendors) == 0 {
		return fmt.Errorf("no vendors configured")
	}

	// Check for duplicate vendor names
	names := make(map[string]bool)
	for _, vendor := range config.Vendors {
		if names[vendor.Name] {
			return fmt.Errorf("duplicate vendor name: %s", vendor.Name)
		}
		names[vendor.Name] = true

		// Validate vendor has URL
		if vendor.URL == "" {
			return fmt.Errorf("vendor %s has no URL", vendor.Name)
		}

		// Validate vendor has at least one spec
		if len(vendor.Specs) == 0 {
			return fmt.Errorf("vendor %s has no specs configured", vendor.Name)
		}

		// Validate each spec
		for _, spec := range vendor.Specs {
			if spec.Ref == "" {
				return fmt.Errorf("vendor %s has a spec with no ref", vendor.Name)
			}

			if len(spec.Mapping) == 0 {
				return fmt.Errorf("vendor %s @ %s has no path mappings", vendor.Name, spec.Ref)
			}

			// Validate each mapping
			for _, mapping := range spec.Mapping {
				if mapping.From == "" {
					return fmt.Errorf("vendor %s @ %s has a mapping with empty 'from' path", vendor.Name, spec.Ref)
				}
			}
		}
	}

	return nil
}