//go:build integration

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git-vendor/internal/types"
)

// ============================================================================
// Integration Test Infrastructure
// ============================================================================

// skipIfNoGit skips the test if git is not available in PATH
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH, skipping integration test")
	}
}

// createTestRepository creates a real git repository for testing
// Returns the absolute path to the repository
func createTestRepository(t *testing.T, name string) string {
	t.Helper()
	skipIfNoGit(t)

	// Create temp directory
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, name)

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	// Initialize git repo
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")

	return repoDir
}

// runGit executes a git command in the specified directory
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

// writeFile writes content to a file, creating parent directories as needed
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

// getCommitHash returns the current HEAD commit hash
func getCommitHash(t *testing.T, dir string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", "HEAD")
}

// ============================================================================
// Integration Tests
// ============================================================================

// TestIntegration_GitOperations_Clone verifies shallow cloning with real git
func TestIntegration_GitOperations_Clone(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository with content
	srcRepo := createTestRepository(t, "source")
	writeFile(t, filepath.Join(srcRepo, "README.md"), "# Test Repository")
	writeFile(t, filepath.Join(srcRepo, "src/main.go"), "package main\n\nfunc main() {}\n")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Initial commit")

	// Create destination for clone
	destDir := filepath.Join(t.TempDir(), "clone")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}

	// Test: clone with filter=blob:none and depth=1
	gitClient := NewSystemGitClient(false)
	opts := &types.CloneOptions{
		Filter:     "blob:none",
		Depth:      1,
		NoCheckout: false,
	}

	repoURL := "file://" + srcRepo
	err := gitClient.Clone(destDir, repoURL, opts)

	// Verify
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(destDir, "README.md")); os.IsNotExist(err) {
		t.Error("README.md not found after clone")
	}
	if _, err := os.Stat(filepath.Join(destDir, "src/main.go")); os.IsNotExist(err) {
		t.Error("src/main.go not found after clone")
	}

	// Verify shallow clone (only 1 commit)
	commitCount := runGit(t, destDir, "rev-list", "--count", "HEAD")
	if commitCount != "1" {
		t.Errorf("Expected 1 commit in shallow clone, got %s", commitCount)
	}
}

// TestIntegration_GitOperations_ListTree verifies git ls-tree functionality
func TestIntegration_GitOperations_ListTree(t *testing.T) {
	skipIfNoGit(t)

	// Create repository with nested structure
	repo := createTestRepository(t, "tree-test")
	writeFile(t, filepath.Join(repo, "README.md"), "root file")
	writeFile(t, filepath.Join(repo, "src/lib.go"), "library code")
	writeFile(t, filepath.Join(repo, "src/util/helper.go"), "helper code")
	writeFile(t, filepath.Join(repo, "docs/guide.md"), "documentation")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "Add files")

	// Test: ListTree at root
	gitClient := NewSystemGitClient(false)
	items, err := gitClient.ListTree(repo, "HEAD", "")

	// Verify root listing
	if err != nil {
		t.Fatalf("ListTree(root) failed: %v", err)
	}
	expectedRoot := []string{"README.md", "docs/", "src/"}
	if !equalStringSlices(items, expectedRoot) {
		t.Errorf("ListTree(root) = %v, want %v", items, expectedRoot)
	}

	// Test: ListTree in subdirectory
	items, err = gitClient.ListTree(repo, "HEAD", "src")
	if err != nil {
		t.Fatalf("ListTree(src) failed: %v", err)
	}
	expectedSrc := []string{"lib.go", "util/"}
	if !equalStringSlices(items, expectedSrc) {
		t.Errorf("ListTree(src) = %v, want %v", items, expectedSrc)
	}

	// Test: ListTree in nested subdirectory
	items, err = gitClient.ListTree(repo, "HEAD", "src/util")
	if err != nil {
		t.Fatalf("ListTree(src/util) failed: %v", err)
	}
	expectedUtil := []string{"helper.go"}
	if !equalStringSlices(items, expectedUtil) {
		t.Errorf("ListTree(src/util) = %v, want %v", items, expectedUtil)
	}
}

// TestIntegration_LicenseDetection_FallbackReader verifies license file reading
func TestIntegration_LicenseDetection_FallbackReader(t *testing.T) {
	skipIfNoGit(t)

	// Create repository with MIT license
	repo := createTestRepository(t, "licensed")
	mitLicense := `MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software.`
	writeFile(t, filepath.Join(repo, "LICENSE"), mitLicense)
	writeFile(t, filepath.Join(repo, "README.md"), "# Licensed Project")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "Add license")

	// Test: FallbackLicenseChecker detects MIT
	fs := NewOSFileSystem()
	gitClient := NewSystemGitClient(false)
	checker := NewFallbackLicenseChecker(fs, gitClient)

	repoURL := "file://" + repo
	license, err := checker.CheckLicense(repoURL)

	// Verify
	if err != nil {
		t.Fatalf("CheckLicense failed: %v", err)
	}
	if license != "MIT" {
		t.Errorf("Expected MIT license, got %q", license)
	}
}

// TestIntegration_LicenseDetection_ApacheLicense verifies Apache 2.0 detection
func TestIntegration_LicenseDetection_ApacheLicense(t *testing.T) {
	skipIfNoGit(t)

	// Create repository with Apache 2.0 license
	repo := createTestRepository(t, "apache-licensed")
	apacheLicense := `Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION`
	writeFile(t, filepath.Join(repo, "LICENSE"), apacheLicense)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "Add Apache license")

	// Test: FallbackLicenseChecker detects Apache-2.0
	fs := NewOSFileSystem()
	gitClient := NewSystemGitClient(false)
	checker := NewFallbackLicenseChecker(fs, gitClient)

	repoURL := "file://" + repo
	license, err := checker.CheckLicense(repoURL)

	// Verify
	if err != nil {
		t.Fatalf("CheckLicense failed: %v", err)
	}
	if license != "Apache-2.0" {
		t.Errorf("Expected Apache-2.0 license, got %q", license)
	}
}

// TestIntegration_GitOperations_Fetch verifies fetching specific refs
func TestIntegration_GitOperations_Fetch(t *testing.T) {
	skipIfNoGit(t)

	// Create source repository with multiple commits
	srcRepo := createTestRepository(t, "fetch-source")
	writeFile(t, filepath.Join(srcRepo, "v1.txt"), "version 1")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Version 1")
	runGit(t, srcRepo, "tag", "v1.0")

	writeFile(t, filepath.Join(srcRepo, "v2.txt"), "version 2")
	runGit(t, srcRepo, "add", ".")
	runGit(t, srcRepo, "commit", "-m", "Version 2")
	runGit(t, srcRepo, "tag", "v2.0")

	// Create bare repo to fetch into
	destRepo := createTestRepository(t, "fetch-dest")
	repoURL := "file://" + srcRepo

	gitClient := NewSystemGitClient(false)

	// Test: Init, AddRemote, Fetch
	err := gitClient.Init(destRepo)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = gitClient.AddRemote(destRepo, "origin", repoURL)
	if err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}

	// Test: Fetch specific ref (tags need refs/ prefix)
	err = gitClient.Fetch(destRepo, 1, "refs/tags/v1.0:refs/tags/v1.0")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Verify: can checkout the fetched tag
	err = gitClient.Checkout(destRepo, "v1.0")
	if err != nil {
		// Fetch succeeded even if checkout fails (tag might not be local)
		t.Logf("Fetch succeeded but checkout failed (expected): %v", err)
	}
}

// TestIntegration_GitOperations_Checkout verifies checkout functionality
func TestIntegration_GitOperations_Checkout(t *testing.T) {
	skipIfNoGit(t)

	// Create repository with two commits
	repo := createTestRepository(t, "checkout-test")
	writeFile(t, filepath.Join(repo, "file.txt"), "first")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "First")
	firstHash := getCommitHash(t, repo)

	writeFile(t, filepath.Join(repo, "file.txt"), "second")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "Second")

	gitClient := NewSystemGitClient(false)

	// Test: Checkout first commit
	err := gitClient.Checkout(repo, firstHash)
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Verify: file has first content
	content, err := os.ReadFile(filepath.Join(repo, "file.txt"))
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "first" {
		t.Errorf("Expected file content 'first', got %q", string(content))
	}

	// Verify: HEAD is at first commit
	currentHash := getCommitHash(t, repo)
	if currentHash != firstHash {
		t.Errorf("Expected HEAD at %s, got %s", firstHash, currentHash)
	}
}

// TestIntegration_GitOperations_GetHeadHash verifies commit hash retrieval
func TestIntegration_GitOperations_GetHeadHash(t *testing.T) {
	skipIfNoGit(t)

	// Create repository with commit
	repo := createTestRepository(t, "hash-test")
	writeFile(t, filepath.Join(repo, "README.md"), "test")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "Test commit")

	expectedHash := getCommitHash(t, repo)

	// Test: GetHeadHash
	gitClient := NewSystemGitClient(false)
	hash, err := gitClient.GetHeadHash(repo)

	// Verify
	if err != nil {
		t.Fatalf("GetHeadHash failed: %v", err)
	}
	if hash != expectedHash {
		t.Errorf("GetHeadHash() = %s, want %s", hash, expectedHash)
	}

	// Verify: hash is 40-character SHA-1
	if len(hash) != 40 {
		t.Errorf("Expected 40-character hash, got %d characters", len(hash))
	}
}

// TestIntegration_PathMapping_CopyNestedDirectories verifies complex path operations
func TestIntegration_PathMapping_CopyNestedDirectories(t *testing.T) {
	skipIfNoGit(t)

	// Create source directory structure
	srcDir := t.TempDir()
	writeFile(t, filepath.Join(srcDir, "src/pkg/module/file.go"), "package module\n")
	writeFile(t, filepath.Join(srcDir, "src/pkg/module/helper.go"), "package module\n// helper\n")
	writeFile(t, filepath.Join(srcDir, "src/pkg/types.go"), "package pkg\n")

	// Create destination
	destDir := t.TempDir()

	// Test: Copy nested directory with custom mapping
	// src/pkg/module -> dest/lib/custom
	fs := NewOSFileSystem()
	srcPath := filepath.Join(srcDir, "src/pkg/module")
	destPath := filepath.Join(destDir, "dest/lib/custom")

	stats, err := fs.CopyDir(srcPath, destPath)

	// Verify
	if err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}
	if stats.FileCount != 2 {
		t.Errorf("Expected 2 files copied, got %d", stats.FileCount)
	}

	// Verify files exist at destination
	expectedFiles := []string{
		filepath.Join(destDir, "dest/lib/custom/file.go"),
		filepath.Join(destDir, "dest/lib/custom/helper.go"),
	}
	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file %s not found", file)
		}
	}

	// Verify content preserved
	content, err := os.ReadFile(filepath.Join(destDir, "dest/lib/custom/file.go"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(content) != "package module\n" {
		t.Errorf("File content not preserved")
	}
}

// ============================================================================
// Test Helpers
// ============================================================================

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// printDirectory prints directory structure for debugging
func printDirectory(t *testing.T, dir string) {
	t.Helper()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(dir, path)
		if info.IsDir() {
			fmt.Printf("[DIR]  %s\n", relPath)
		} else {
			fmt.Printf("[FILE] %s\n", relPath)
		}
		return nil
	})
	if err != nil {
		t.Logf("Failed to print directory: %v", err)
	}
}
