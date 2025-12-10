package core

import (
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
	tempDir, err := os.MkdirTemp("", "git-vendor-index-*")
	if err != nil { return nil, err }
	defer os.RemoveAll(tempDir)

	err = runGit(tempDir, "clone", "--filter=blob:none", "--no-checkout", "--depth", "1", url, ".")
	if err != nil { return nil, err }

	if ref != "" && ref != "HEAD" { runGit(tempDir, "fetch", "origin", ref) }
	target := ref
	if target == "" { target = "HEAD" }
	
	cmd := exec.Command("git", "ls-tree", target)
	if subdir != "" && subdir != "." {
		cleanSub := strings.TrimSuffix(subdir, "/")
		cmd.Args = append(cmd.Args, cleanSub + "/")
	}
	cmd.Dir = tempDir
	out, err := cmd.Output()
	if err != nil && subdir != "" {
		cmd = exec.Command("git", "ls-tree", target, strings.TrimSuffix(subdir, "/"))
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
	if found == -1 { return fmt.Errorf("vendor '%s' not found", name) }
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
				return fmt.Errorf("compliance check failed")
			}
		} else {
			tui.PrintComplianceSuccess(spec.License)
		}
	}
	return m.SaveVendor(spec)
}

func (m *Manager) Sync() error {
	config, err := m.loadConfig()
	if err != nil { return err }

	lock, err := m.loadLock()
	if err != nil || len(lock.Vendors) == 0 {
		fmt.Println("No lockfile found. Running update...")
		return m.UpdateAll()
	}

	lockMap := make(map[string]map[string]string)
	for _, l := range lock.Vendors {
		if lockMap[l.Name] == nil { lockMap[l.Name] = make(map[string]string) }
		lockMap[l.Name][l.Ref] = l.CommitHash
	}

	for _, v := range config.Vendors {
		v.URL = cleanURL(v.URL)
		if _, err := m.syncVendor(v, lockMap[v.Name]); err != nil {
			return err
		}
	}
	return nil
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
			runGit(tempDir, "fetch", "--depth", "1", "origin", spec.Ref)
			if err := runGit(tempDir, "checkout", targetCommit); err != nil {
				runGit(tempDir, "fetch", "origin")
				if err := runGit(tempDir, "checkout", targetCommit); err != nil {
					return nil, fmt.Errorf("checkout locked hash %s failed: %w", targetCommit, err)
				}
			}
		} else {
			if err := runGit(tempDir, "fetch", "--depth", "1", "origin", spec.Ref); err != nil {
				runGit(tempDir, "fetch", "origin")
			}
			if err := runGit(tempDir, "checkout", "FETCH_HEAD"); err != nil {
				if err := runGit(tempDir, "checkout", spec.Ref); err != nil {
					return nil, fmt.Errorf("checkout ref %s failed: %w", spec.Ref, err)
				}
			}
		}

		hash, _ := getHeadHash(tempDir)
		results[spec.Ref] = hash

		// License Automation
		licenseSrc := filepath.Join(tempDir, "LICENSE")
		if _, err := os.Stat(licenseSrc); os.IsNotExist(err) {
			licenseSrc = filepath.Join(tempDir, "LICENSE.txt")
		}
		if _, err := os.Stat(licenseSrc); os.IsNotExist(err) {
			licenseSrc = filepath.Join(tempDir, "COPYING")
		}

		if _, err := os.Stat(licenseSrc); err == nil {
			os.MkdirAll(filepath.Join(m.RootDir, LicenseDir), 0755)
			dest := m.LicensePath(v.Name)
			copyFile(licenseSrc, dest)
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
				return nil, fmt.Errorf("path '%s' not found", srcClean)
			}

			if info.IsDir() {
				os.MkdirAll(destPath, 0755)
				copyDir(srcPath, destPath)
			} else {
				os.MkdirAll(filepath.Dir(destPath), 0755)
				copyFile(srcPath, destPath)
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
	if len(matches) < 3 { return "", fmt.Errorf("invalid url") }
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
	allowed := []string{"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC", "Unlicense", "CC0-1.0"}
	for _, l := range allowed { if license == l { return true } }
	return false
}
func (m *Manager) GetConfig() (types.VendorConfig, error) { return m.loadConfig() }
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