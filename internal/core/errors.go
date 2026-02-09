package core

import (
	"errors"
	"fmt"
	"strings"
)

// Error format follows ROADMAP 9.5:
//
//	Error: <what went wrong>
//	  Context: <relevant details>
//	  Fix: <what the user should do>

// =============================================================================
// Sentinel Errors
// =============================================================================

// Sentinel errors for common error conditions.
// These can be used with errors.Is() for error type checking.
var (
	// ErrNotInitialized indicates the vendor directory doesn't exist
	ErrNotInitialized = errors.New("vendor directory not found. Run 'git-vendor init' first")

	// ErrComplianceFailed indicates a license compliance check failed
	ErrComplianceFailed = errors.New("compliance check failed")
)

// =============================================================================
// Structured Error Types
// =============================================================================

// VendorNotFoundError is returned when a vendor name doesn't exist in config.
type VendorNotFoundError struct {
	Name string
}

func (e *VendorNotFoundError) Error() string {
	return fmt.Sprintf("Error: Vendor '%s' not found\n  Context: No vendor with this name exists in %s\n  Fix: Run 'git-vendor list' to see available vendors", e.Name, ConfigPath)
}

// NewVendorNotFoundError creates a VendorNotFoundError.
func NewVendorNotFoundError(name string) *VendorNotFoundError {
	return &VendorNotFoundError{Name: name}
}

// GroupNotFoundError is returned when a group doesn't exist in any vendor.
type GroupNotFoundError struct {
	Name string
}

func (e *GroupNotFoundError) Error() string {
	return fmt.Sprintf("Error: Group '%s' not found\n  Context: No vendor is tagged with this group in %s\n  Fix: Run 'git-vendor list' to see available vendors and their groups", e.Name, ConfigPath)
}

// NewGroupNotFoundError creates a GroupNotFoundError.
func NewGroupNotFoundError(name string) *GroupNotFoundError {
	return &GroupNotFoundError{Name: name}
}

// PathNotFoundError is returned when a source path doesn't exist.
type PathNotFoundError struct {
	Path       string
	VendorName string
	Ref        string
}

func (e *PathNotFoundError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error: Path '%s' not found", e.Path))
	if e.VendorName != "" && e.Ref != "" {
		b.WriteString(fmt.Sprintf("\n  Context: The path does not exist in %s@%s", e.VendorName, e.Ref))
	}
	b.WriteString("\n  Fix: Verify the path exists in the source repository, or run 'git-vendor edit' to update mappings")
	return b.String()
}

// NewPathNotFoundError creates a PathNotFoundError.
func NewPathNotFoundError(path, vendorName, ref string) *PathNotFoundError {
	return &PathNotFoundError{Path: path, VendorName: vendorName, Ref: ref}
}

// StaleCommitError is returned when a locked commit no longer exists in the remote.
type StaleCommitError struct {
	CommitHash string
	VendorName string
	Ref        string
}

func (e *StaleCommitError) Error() string {
	hash := e.CommitHash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error: Locked commit %s no longer exists", hash))
	if e.VendorName != "" {
		b.WriteString(fmt.Sprintf("\n  Context: Vendor '%s'", e.VendorName))
		if e.Ref != "" {
			b.WriteString(fmt.Sprintf(" at ref '%s'", e.Ref))
		}
		b.WriteString(" references a commit that was deleted or force-pushed")
	}
	b.WriteString("\n  Fix: Run 'git-vendor update' to fetch the latest commit and update the lockfile")
	return b.String()
}

// NewStaleCommitError creates a StaleCommitError.
func NewStaleCommitError(commitHash, vendorName, ref string) *StaleCommitError {
	return &StaleCommitError{CommitHash: commitHash, VendorName: vendorName, Ref: ref}
}

// CheckoutError is returned when git checkout fails.
type CheckoutError struct {
	Target     string // commit hash or ref being checked out
	VendorName string
	Cause      error
}

func (e *CheckoutError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error: Failed to checkout '%s'", e.Target))
	if e.VendorName != "" {
		b.WriteString(fmt.Sprintf("\n  Context: While syncing vendor '%s'", e.VendorName))
	}
	if e.Cause != nil {
		b.WriteString(fmt.Sprintf(": %v", e.Cause))
	}
	b.WriteString("\n  Fix: Check network connectivity and repository access, or run 'git-vendor update' to refresh")
	return b.String()
}

func (e *CheckoutError) Unwrap() error {
	return e.Cause
}

// NewCheckoutError creates a CheckoutError.
func NewCheckoutError(target, vendorName string, cause error) *CheckoutError {
	return &CheckoutError{Target: target, VendorName: vendorName, Cause: cause}
}

// ValidationError is returned when configuration validation fails.
type ValidationError struct {
	VendorName string
	Ref        string
	Field      string
	Message    string
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	b.WriteString("Error: Invalid configuration")
	if e.VendorName != "" {
		b.WriteString(fmt.Sprintf(" for vendor '%s'", e.VendorName))
	}
	b.WriteString(fmt.Sprintf("\n  Context: %s", e.Message))
	if e.Ref != "" {
		b.WriteString(fmt.Sprintf(" (ref: %s)", e.Ref))
	}
	if e.Field != "" {
		b.WriteString(fmt.Sprintf(" [field: %s]", e.Field))
	}
	b.WriteString(fmt.Sprintf("\n  Fix: Edit %s to correct the configuration", ConfigPath))
	return b.String()
}

// NewValidationError creates a ValidationError.
func NewValidationError(vendorName, ref, field, message string) *ValidationError {
	return &ValidationError{
		VendorName: vendorName,
		Ref:        ref,
		Field:      field,
		Message:    message,
	}
}

// =============================================================================
// Error Type Checking Helpers
// =============================================================================

// IsVendorNotFound returns true if err is a VendorNotFoundError.
func IsVendorNotFound(err error) bool {
	var e *VendorNotFoundError
	return errors.As(err, &e)
}

// IsGroupNotFound returns true if err is a GroupNotFoundError.
func IsGroupNotFound(err error) bool {
	var e *GroupNotFoundError
	return errors.As(err, &e)
}

// IsPathNotFound returns true if err is a PathNotFoundError.
func IsPathNotFound(err error) bool {
	var e *PathNotFoundError
	return errors.As(err, &e)
}

// IsStaleCommit returns true if err is a StaleCommitError.
func IsStaleCommit(err error) bool {
	var e *StaleCommitError
	return errors.As(err, &e)
}

// IsCheckoutError returns true if err is a CheckoutError.
func IsCheckoutError(err error) bool {
	var e *CheckoutError
	return errors.As(err, &e)
}

// IsValidationError returns true if err is a ValidationError.
func IsValidationError(err error) bool {
	var e *ValidationError
	return errors.As(err, &e)
}

// HookError is returned when a pre/post-sync hook fails.
type HookError struct {
	VendorName string
	Phase      string // "pre-sync" or "post-sync"
	Command    string // first 80 chars of the command for context
	Cause      error
}

func (e *HookError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error: %s hook failed", e.Phase))
	if e.VendorName != "" {
		b.WriteString(fmt.Sprintf(" for vendor '%s'", e.VendorName))
	}
	if e.Cause != nil {
		b.WriteString(fmt.Sprintf("\n  Context: %v", e.Cause))
	}
	if e.Command != "" {
		cmd := e.Command
		if len(cmd) > 80 {
			cmd = cmd[:80] + "..."
		}
		b.WriteString(fmt.Sprintf("\n  Command: %s", cmd))
	}
	b.WriteString("\n  Fix: Check the hook command in vendor.yml and ensure it exits successfully")
	return b.String()
}

func (e *HookError) Unwrap() error {
	return e.Cause
}

// NewHookError creates a HookError.
func NewHookError(vendorName, phase, command string, cause error) *HookError {
	return &HookError{VendorName: vendorName, Phase: phase, Command: command, Cause: cause}
}

// IsHookError returns true if err is a HookError.
func IsHookError(err error) bool {
	var e *HookError
	return errors.As(err, &e)
}

// OSVAPIError is returned when the OSV.dev API returns a non-OK response.
type OSVAPIError struct {
	StatusCode int
	Body       string // Truncated response body for diagnostics
}

func (e *OSVAPIError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error: OSV.dev API returned HTTP %d", e.StatusCode))
	switch {
	case e.StatusCode == 429:
		b.WriteString("\n  Context: Rate limited by OSV.dev API")
		b.WriteString("\n  Fix: Wait a few minutes before retrying, or use cached results")
	case e.StatusCode >= 500:
		b.WriteString("\n  Context: OSV.dev server error (transient)")
		b.WriteString("\n  Fix: Retry the scan later; cached results will be used if available")
	case e.StatusCode >= 400:
		b.WriteString("\n  Context: Client error — request may be malformed")
		if e.Body != "" {
			b.WriteString(fmt.Sprintf("\n  Detail: %s", e.Body))
		}
	}
	return b.String()
}

// NewOSVAPIError creates an OSVAPIError.
func NewOSVAPIError(statusCode int, body string) *OSVAPIError {
	// Truncate body for error message readability
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	return &OSVAPIError{StatusCode: statusCode, Body: body}
}

// IsOSVAPIError returns true if err is an OSVAPIError.
func IsOSVAPIError(err error) bool {
	var e *OSVAPIError
	return errors.As(err, &e)
}
