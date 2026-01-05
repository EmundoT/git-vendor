package core

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/EmundoT/git-vendor/internal/types"
)

// HookExecutor handles pre/post sync hook execution
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
	ui UICallback
}

// NewHookService creates a new hook executor
func NewHookService(ui UICallback) HookExecutor {
	return &hookService{
		ui: ui,
	}
}

// ExecutePreSync runs the pre-sync hook if configured
func (h *hookService) ExecutePreSync(vendor *types.VendorSpec, ctx *types.HookContext) error {
	if vendor.Hooks == nil || vendor.Hooks.PreSync == "" {
		return nil
	}

	fmt.Printf("  🪝 Running pre-sync hook...\n")
	return h.executeHook(vendor.Hooks.PreSync, ctx)
}

// ExecutePostSync runs the post-sync hook if configured
func (h *hookService) ExecutePostSync(vendor *types.VendorSpec, ctx *types.HookContext) error {
	if vendor.Hooks == nil || vendor.Hooks.PostSync == "" {
		return nil
	}

	fmt.Printf("  🪝 Running post-sync hook...\n")
	return h.executeHook(vendor.Hooks.PostSync, ctx)
}

// executeHook runs a shell command with environment context
func (h *hookService) executeHook(command string, ctx *types.HookContext) error {
	// Build environment variables
	env := h.buildEnvironment(ctx)

	// Execute command via shell for proper multiline and pipe support
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	cmd.Dir = ctx.RootDir

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Display output to user
	if len(output) > 0 {
		fmt.Print(string(output))
	}

	if err != nil {
		return fmt.Errorf("hook failed: %w", err)
	}

	return nil
}

// buildEnvironment creates environment variables for hook execution
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

	// Append git-vendor variables
	for key, value := range vendorVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Append custom environment variables if provided
	if ctx.Environment != nil {
		for key, value := range ctx.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return env
}
