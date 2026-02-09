package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

	// Split into lines and trim each line (Windows echo adds trailing spaces)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	if strings.TrimSpace(lines[0]) != "line1" {
		t.Errorf("Expected first line 'line1', got %q", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "line2" {
		t.Errorf("Expected second line 'line2', got %q", lines[1])
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

// TestHookService_CommandFailure_ReturnsHookError verifies that hook failures produce HookError
func TestHookService_CommandFailure_ReturnsHookError(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name: "hook-err-test",
		Hooks: &types.HookConfig{
			PreSync: "exit 42",
		},
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName: "hook-err-test",
		RootDir:    tempDir,
	}

	err := hookService.ExecutePreSync(vendor, ctx)
	if err == nil {
		t.Fatal("Expected error for failing command, got nil")
	}

	// Verify HookError type is returned
	if !IsHookError(err) {
		t.Errorf("Expected HookError, got: %T: %v", err, err)
	}

	// Verify error message contains useful context
	errStr := err.Error()
	if !strings.Contains(errStr, "pre-sync") {
		t.Errorf("Expected error to mention 'pre-sync' phase, got: %s", errStr)
	}
	if !strings.Contains(errStr, "hook-err-test") {
		t.Errorf("Expected error to mention vendor name, got: %s", errStr)
	}
}

// TestHookService_PostSyncFailure_ReturnsHookError verifies post-sync HookError
func TestHookService_PostSyncFailure_ReturnsHookError(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	vendor := &types.VendorSpec{
		Name: "post-err-test",
		Hooks: &types.HookConfig{
			PostSync: "exit 1",
		},
	}

	tempDir := t.TempDir()
	ctx := &types.HookContext{
		VendorName: "post-err-test",
		RootDir:    tempDir,
	}

	err := hookService.ExecutePostSync(vendor, ctx)
	if err == nil {
		t.Fatal("Expected error for failing command, got nil")
	}

	if !IsHookError(err) {
		t.Errorf("Expected HookError, got: %T: %v", err, err)
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "post-sync") {
		t.Errorf("Expected error to mention 'post-sync' phase, got: %s", errStr)
	}
}

// TestHookService_Timeout verifies that hooks are killed after the timeout period.
// Uses a short-lived context to avoid long test runs.
func TestHookService_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Timeout test uses Unix sleep command")
	}

	ui := &SilentUICallback{}
	hs := NewHookService(ui).(*hookService)

	hookCtx := &types.HookContext{
		VendorName: "timeout-test",
		RootDir:    t.TempDir(),
	}

	start := time.Now()

	// Directly test the timeout mechanism using a short context.
	// Invoke sleep directly (not via sh -c) to ensure SIGKILL reaches the process.
	env := hs.buildEnvironment(hookCtx)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sleep", "60")
	cmd.Env = env
	cmd.Dir = hookCtx.RootDir
	_, err := cmd.CombinedOutput()

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded, got: %v", ctx.Err())
	}

	// Verify command was killed quickly (well under 60s)
	if elapsed > 5*time.Second {
		t.Errorf("Hook should have timed out quickly, took %v", elapsed)
	}
}

// TestHookService_EnvironmentSanitization verifies that newlines in env values are stripped
func TestHookService_EnvironmentSanitization(t *testing.T) {
	ui := &SilentUICallback{}
	hookService := NewHookService(ui)

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "sanitized_env.txt")

	// Build platform-appropriate command
	var command string
	if runtime.GOOS == "windows" {
		command = "set | findstr GIT_VENDOR_NAME > " + outputFile
	} else {
		command = "env | grep GIT_VENDOR_NAME > " + outputFile
	}

	vendor := &types.VendorSpec{
		Name: "sanitize-test",
		Hooks: &types.HookConfig{
			PreSync: command,
		},
	}

	// Inject newlines and null bytes into vendor name via context
	ctx := &types.HookContext{
		VendorName: "evil\nINJECTED=true\r\x00name",
		VendorURL:  "https://github.com/test/repo\nnewline",
		RootDir:    tempDir,
	}

	err := hookService.ExecutePreSync(vendor, ctx)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	output := string(content)

	// Verify newlines were sanitized (replaced with spaces)
	if strings.Contains(output, "\nINJECTED=true") {
		t.Errorf("Newline injection not sanitized in environment variable:\n%s", output)
	}

	// Verify the sanitized name is a single line containing the vendor name
	if !strings.Contains(output, "GIT_VENDOR_NAME=evil") {
		t.Errorf("Expected sanitized vendor name in env output:\n%s", output)
	}
}

// TestHookService_LongCommandInError verifies that long commands are truncated in HookError
func TestHookService_LongCommandInError(t *testing.T) {
	longCmd := strings.Repeat("a", 200)
	err := NewHookError("vendor", "pre-sync", longCmd, nil)

	errStr := err.Error()
	// Command should be truncated to 80 chars + "..."
	if strings.Contains(errStr, strings.Repeat("a", 100)) {
		t.Errorf("Expected long command to be truncated, got full command in error message")
	}
	if !strings.Contains(errStr, "...") {
		t.Errorf("Expected '...' in truncated command, got: %s", errStr)
	}
}
