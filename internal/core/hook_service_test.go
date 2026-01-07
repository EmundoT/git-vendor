package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

// No need for testUICallback - use existing SilentUICallback

// TestHookService_PreSyncExecution tests basic pre-sync hook execution
func TestHookService_PreSyncExecution(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Hooks: &types.HookConfig{
			PreSync: "echo 'pre-sync test'",
		},
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName:  "test-vendor",
		VendorURL:   "https://github.com/test/repo",
		Ref:         "main",
		CommitHash:  "abc123",
		RootDir:     tempDir,
		FilesCopied: 5,
	}

	// Execute
	err := hookService.ExecutePreSync(vendor, ctx)

	// Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

// TestHookService_PostSyncExecution tests basic post-sync hook execution
func TestHookService_PostSyncExecution(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name: "test-vendor",
		Hooks: &types.HookConfig{
			PostSync: "echo 'post-sync test'",
		},
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName:  "test-vendor",
		VendorURL:   "https://github.com/test/repo",
		Ref:         "main",
		CommitHash:  "def456",
		RootDir:     tempDir,
		FilesCopied: 10,
	}

	// Execute
	err := hookService.ExecutePostSync(vendor, ctx)

	// Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

// TestHookService_EnvironmentVariables tests that environment variables are properly injected
func TestHookService_EnvironmentVariables(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	// Create a temporary directory for the hook to write to
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "env_output.txt")

	// Build platform-appropriate command to list environment variables
	var command string
	if runtime.GOOS == "windows" {
		// Windows: Use 'set' to list env vars, 'findstr' to filter
		command = "set | findstr GIT_VENDOR > " + outputFile
	} else {
		// Unix: Use 'env' to list env vars, 'grep' to filter
		command = "env | grep GIT_VENDOR > " + outputFile
	}

	vendor := &types.VendorSpec{
		Name: "env-test",
		Hooks: &types.HookConfig{
			PreSync: command,
		},
	}

	ctx := &types.HookContext{
		VendorName:  "env-test",
		VendorURL:   "https://github.com/test/repo",
		Ref:         "main",
		CommitHash:  "def456",
		RootDir:     tempDir,
		FilesCopied: 10,
		DirsCreated: 3,
	}

	// Execute
	err := hookService.ExecutePreSync(vendor, ctx)

	// Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Read the output file and verify environment variables
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Check that all expected environment variables are present
	expectedVars := []string{
		"GIT_VENDOR_NAME=env-test",
		"GIT_VENDOR_URL=https://github.com/test/repo",
		"GIT_VENDOR_REF=main",
		"GIT_VENDOR_COMMIT=def456",
		"GIT_VENDOR_ROOT=" + tempDir,
		"GIT_VENDOR_FILES_COPIED=10",
		"GIT_VENDOR_DIRS_CREATED=3",
	}

	for _, expectedVar := range expectedVars {
		if !strings.Contains(output, expectedVar) {
			t.Errorf("Expected environment variable not found: %s\nOutput:\n%s", expectedVar, output)
		}
	}
}

// TestHookService_MultilineCommand tests that multiline commands are supported
func TestHookService_MultilineCommand(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	// Create a temporary directory for the hook to write to
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "multiline_output.txt")

	// Build platform-appropriate multiline command
	var command string
	if runtime.GOOS == "windows" {
		// Windows: Use echo without quotes, & to chain commands
		command = "echo line1 > " + outputFile + " & echo line2 >> " + outputFile
	} else {
		// Unix: Use echo with quotes, newline to chain commands
		command = "echo 'line1' > " + outputFile + "\necho 'line2' >> " + outputFile
	}

	vendor := &types.VendorSpec{
		Name: "multiline-test",
		Hooks: &types.HookConfig{
			PreSync: command,
		},
	}

	ctx := &types.HookContext{
		VendorName: "multiline-test",
		RootDir:    tempDir,
	}

	// Execute
	err := hookService.ExecutePreSync(vendor, ctx)

	// Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify both lines were written
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := strings.TrimSpace(string(content))
	expectedOutput := "line1\nline2"

	if output != expectedOutput {
		t.Errorf("Expected output:\n%s\nGot:\n%s", expectedOutput, output)
	}
}

// TestHookService_CommandFailure tests that hook failures are properly reported
func TestHookService_CommandFailure(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name: "fail-test",
		Hooks: &types.HookConfig{
			PreSync: "exit 1",
		},
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName: "fail-test",
		RootDir:    tempDir,
	}

	// Execute
	err := hookService.ExecutePreSync(vendor, ctx)

	// Assert
	if err == nil {
		t.Fatal("Expected error for failing command, got nil")
	}

	if !strings.Contains(err.Error(), "hook failed") {
		t.Errorf("Expected 'hook failed' error, got: %v", err)
	}
}

// TestHookService_NoHookConfigured tests that no hook configured is a no-op
func TestHookService_NoHookConfigured(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name:  "no-hook-test",
		Hooks: nil, // No hooks configured
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName: "no-hook-test",
		RootDir:    tempDir,
	}

	// Execute pre-sync
	err := hookService.ExecutePreSync(vendor, ctx)
	if err != nil {
		t.Fatalf("Expected no error when no pre-sync hook configured, got: %v", err)
	}

	// Execute post-sync
	err = hookService.ExecutePostSync(vendor, ctx)
	if err != nil {
		t.Fatalf("Expected no error when no post-sync hook configured, got: %v", err)
	}
}

// TestHookService_EmptyHookCommand tests that empty hook commands are no-ops
func TestHookService_EmptyHookCommand(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name: "empty-hook-test",
		Hooks: &types.HookConfig{
			PreSync:  "", // Empty command
			PostSync: "", // Empty command
		},
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName: "empty-hook-test",
		RootDir:    tempDir,
	}

	// Execute pre-sync
	err := hookService.ExecutePreSync(vendor, ctx)
	if err != nil {
		t.Fatalf("Expected no error for empty pre-sync hook, got: %v", err)
	}

	// Execute post-sync
	err = hookService.ExecutePostSync(vendor, ctx)
	if err != nil {
		t.Fatalf("Expected no error for empty post-sync hook, got: %v", err)
	}
}

// TestHookService_WorkingDirectory tests that hooks run in the correct working directory
func TestHookService_WorkingDirectory(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	// Create a temporary directory that will be the RootDir
	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "pwd_output.txt")

	// Build platform-appropriate command to get current directory
	var command string
	if runtime.GOOS == "windows" {
		// Windows: Use 'cd' to print current directory
		command = "cd > " + outputFile
	} else {
		// Unix: Use 'pwd' to print current directory
		command = "pwd > " + outputFile
	}

	vendor := &types.VendorSpec{
		Name: "pwd-test",
		Hooks: &types.HookConfig{
			PreSync: command,
		},
	}

	ctx := &types.HookContext{
		VendorName: "pwd-test",
		RootDir:    tempDir,
	}

	// Execute
	err := hookService.ExecutePreSync(vendor, ctx)

	// Assert
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Read the output and verify it matches the expected directory
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	actualDir := strings.TrimSpace(string(content))

	// Compare absolute paths (resolve symlinks if any)
	expectedDir, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		expectedDir = tempDir
	}

	actualDirResolved, err := filepath.EvalSymlinks(actualDir)
	if err != nil {
		actualDirResolved = actualDir
	}

	if actualDirResolved != expectedDir {
		t.Errorf("Expected working directory %s, got %s", expectedDir, actualDirResolved)
	}
}
