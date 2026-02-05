package core

import "errors"

// Sentinel errors for common error conditions.
// These can be used with errors.Is() for error type checking.
var (
	// ErrNotInitialized indicates the vendor directory doesn't exist
	ErrNotInitialized = errors.New("vendor directory not found. Run 'git-vendor init' first")

	// ErrComplianceFailed indicates a license compliance check failed
	ErrComplianceFailed = errors.New("compliance check failed")
)

// Error message templates for formatted errors.
// Use with fmt.Errorf() to create errors with context.
const (
	// ErrStaleCommitMsg is the message for when a locked commit no longer exists
	ErrStaleCommitMsg = "locked commit %s no longer exists in the repository.\n\nThis usually happens when the remote repository has been force-pushed or the commit was deleted.\nRun 'git-vendor update' to fetch the latest commit and update the lockfile, then try syncing again"

	// ErrCheckoutFailedMsg is the message for checkout failures
	ErrCheckoutFailedMsg = "checkout locked hash %s failed: %w"

	// ErrRefCheckoutFailedMsg is the message for ref checkout failures
	ErrRefCheckoutFailedMsg = "checkout ref %s failed: %w"

	// ErrPathNotFoundMsg is the message for missing paths
	ErrPathNotFoundMsg = "path '%s' not found"

	// ErrVendorNotFoundMsg is the message for missing vendors
	ErrVendorNotFoundMsg = "vendor '%s' not found"
)
