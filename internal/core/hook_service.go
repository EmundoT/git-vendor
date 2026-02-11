package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/EmundoT/git-vendor/internal/types"
)

const (
	// hookTimeout is the maximum duration a hook command can run before being killed.
	// 5 minutes is generous enough for commands like "npm install && npm run build"
	// while preventing indefinite hangs from misconfigured hooks.
	hookTimeout = 5 * time.Minute
)

// HookExecutor handles pre/post sync hook execution.
//
// Hook Execution Security Model (SEC-012):
// git-vendor hooks follow the same trust model as npm scripts, git hooks, and
// Makefile targets: the user who controls vendor.yml controls the commands.
// Hooks are NOT sandboxed.
//
// Security properties:
//   - Hooks execute via sh -c (Unix) or cmd /c (Windows) in the project root
//   - hookTimeout (5 min) prevents indefinite hangs via context.WithTimeout
//   - sanitizeEnvValue strips \n, \r, \x00 from GIT_VENDOR_* env values
//   - Hooks inherit the parent environment (including GITHUB_TOKEN etc.)
//   - Hook output passes directly to stdout/stderr (unfiltered)
//   - Pre-sync hooks run BEFORE clone; temp clone dir is not exposed
//   - Post-sync hook failure does NOT roll back already-copied files
//
// Accepted risks: unsandboxed execution, credential access via env, unfiltered
// output. These match the accepted trust model for configuration-driven tools.
//
//go:generate mockgen -source=hook_service.go -destination=hook_executor_mock_test.go -package=core
type HookExecutor interface {
	// ExecutePreSync runs pre-sync hook if configured
	ExecutePreSync(vendor *types.VendorSpec, ctx *types.HookContext) error

	// ExecutePostSync runs post-sync hook if configured
	ExecutePostSync(vendor *types.VendorSpec, ctx *types.HookContext) error
}

// hookService implements HookExecutor for shell command execution
type hookService struct {
	ui      UICallback
	timeout time.Duration // overridable for testing; defaults to hookTimeout
}

// NewHookService creates a new hook executor
func NewHookService(ui UICallback) HookExecutor {
	return &hookService{
		ui:      ui,
		timeout: hookTimeout,
	}
}

// ExecutePreSync runs the pre-sync hook if configured.
// Returns a HookError if the command fails or times out.
func (h *hookService) ExecutePreSync(vendor *types.VendorSpec, ctx *types.HookContext) error {
	if vendor.Hooks == nil || vendor.Hooks.PreSync == "" {
		return nil
	}

	fmt.Printf("  🪝 Running pre-sync hook...\n")
	if err := h.executeHook(vendor.Hooks.PreSync, ctx); err != nil {
		return NewHookError(vendor.Name, "pre-sync", vendor.Hooks.PreSync, err)
	}
	return nil
}

// ExecutePostSync runs the post-sync hook if configured.
// Returns a HookError if the command fails or times out.
func (h *hookService) ExecutePostSync(vendor *types.VendorSpec, ctx *types.HookContext) error {
	if vendor.Hooks == nil || vendor.Hooks.PostSync == "" {
		return nil
	}

	fmt.Printf("  🪝 Running post-sync hook...\n")
	if err := h.executeHook(vendor.Hooks.PostSync, ctx); err != nil {
		return NewHookError(vendor.Name, "post-sync", vendor.Hooks.PostSync, err)
	}
	return nil
}

// executeHook runs a shell command with environment context and timeout protection.
// The command is killed after hookTimeout (5 minutes) to prevent indefinite hangs.
func (h *hookService) executeHook(command string, hookCtx *types.HookContext) error {
	// Build environment variables
	env := h.buildEnvironment(hookCtx)

	// Create context with timeout to prevent hanging hooks
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	// Execute command via platform-appropriate shell with timeout context
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: Use cmd.exe /c for command execution
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		// Unix (Linux/macOS): Use sh -c for command execution
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Env = env
	cmd.Dir = hookCtx.RootDir

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Display output to user
	if len(output) > 0 {
		fmt.Print(string(output))
	}

	if err != nil {
		// Distinguish timeout from other failures for clearer error messages
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("hook timed out after %s: %w", h.timeout, err)
		}
		return fmt.Errorf("hook failed: %w", err)
	}

	return nil
}

// buildEnvironment creates environment variables for hook execution.
// Values are sanitized to prevent newline injection into the environment block.
func (h *hookService) buildEnvironment(ctx *types.HookContext) []string {
	// Start with current environment
	env := os.Environ()

	// Add git-vendor specific variables
	vendorVars := map[string]string{
		"GIT_VENDOR_NAME":         ctx.VendorName,
		"GIT_VENDOR_URL":          ctx.VendorURL,
		"GIT_VENDOR_REF":          ctx.Ref,
		"GIT_VENDOR_COMMIT":       ctx.CommitHash,
		"GIT_VENDOR_ROOT":         ctx.RootDir,
		"GIT_VENDOR_FILES_COPIED": fmt.Sprintf("%d", ctx.FilesCopied),
		"GIT_VENDOR_DIRS_CREATED": fmt.Sprintf("%d", ctx.DirsCreated),
	}

	// Append git-vendor variables with sanitized values
	for key, value := range vendorVars {
		env = append(env, fmt.Sprintf("%s=%s", key, sanitizeEnvValue(value)))
	}

	// Append custom environment variables if provided
	if ctx.Environment != nil {
		for key, value := range ctx.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, sanitizeEnvValue(value)))
		}
	}

	return env
}

// sanitizeEnvValue strips newlines and null bytes from environment variable values.
// Newlines in env values can corrupt the environment block on some platforms,
// and null bytes terminate C strings prematurely.
func sanitizeEnvValue(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\x00", "")
	return value
}
