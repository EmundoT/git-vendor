package core

import (
	"errors"
	"fmt"
)

// Sentinel errors for common error conditions.
// These can be used with errors.Is() for error type checking.
var (
	// ErrNotInitialized indicates the vendor directory doesn't exist
	ErrNotInitialized = errors.New("vendor directory not found. Run 'git-vendor init' first")

	// ErrNoVendorsConfigured indicates no vendors are in the config
	ErrNoVendorsConfigured = errors.New("no vendors configured. Run 'git-vendor add' to add your first dependency")

	// ErrInvalidURL indicates an invalid repository URL format
	ErrInvalidURL = errors.New("invalid url")

	// ErrComplianceFailed indicates a license compliance check failed
	ErrComplianceFailed = errors.New("compliance check failed")

	// ErrNoLockfile indicates the lockfile doesn't exist or is empty
	ErrNoLockfile = errors.New("no lockfile found")

	// ErrCacheCorrupted indicates a cache file is corrupted
	ErrCacheCorrupted = errors.New("cache file corrupted")
)

// Error message templates for formatted errors
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

	// ErrGroupNotFoundMsg is the message for missing groups
	ErrGroupNotFoundMsg = "group '%s' not found in any vendor"

	// ErrDuplicateVendorMsg is the message for duplicate vendor names
	ErrDuplicateVendorMsg = "duplicate vendor name: %s"

	// ErrEmptyVendorURLMsg is the message for vendors with no URL
	ErrEmptyVendorURLMsg = "vendor %s has no URL"

	// ErrNoSpecsMsg is the message for vendors with no specs
	ErrNoSpecsMsg = "vendor %s has no specs configured"

	// ErrEmptyRefMsg is the message for specs with no ref
	ErrEmptyRefMsg = "vendor %s has a spec with no ref"

	// ErrNoMappingsMsg is the message for specs with no mappings
	ErrNoMappingsMsg = "vendor %s @ %s has no path mappings"

	// ErrEmptyFromPathMsg is the message for mappings with empty from path
	ErrEmptyFromPathMsg = "vendor %s @ %s has a mapping with empty 'from' path"
)

// VendorNotFoundError represents an error when a vendor is not found
type VendorNotFoundError struct {
	Name string
}

func (e *VendorNotFoundError) Error() string {
	return fmt.Sprintf(ErrVendorNotFoundMsg, e.Name)
}

// NewVendorNotFoundError creates a new VendorNotFoundError
func NewVendorNotFoundError(name string) *VendorNotFoundError {
	return &VendorNotFoundError{Name: name}
}

// GroupNotFoundError represents an error when a group is not found
type GroupNotFoundError struct {
	Name string
}

func (e *GroupNotFoundError) Error() string {
	return fmt.Sprintf(ErrGroupNotFoundMsg, e.Name)
}

// NewGroupNotFoundError creates a new GroupNotFoundError
func NewGroupNotFoundError(name string) *GroupNotFoundError {
	return &GroupNotFoundError{Name: name}
}

// PathNotFoundError represents an error when a path is not found
type PathNotFoundError struct {
	Path string
}

func (e *PathNotFoundError) Error() string {
	return fmt.Sprintf(ErrPathNotFoundMsg, e.Path)
}

// NewPathNotFoundError creates a new PathNotFoundError
func NewPathNotFoundError(path string) *PathNotFoundError {
	return &PathNotFoundError{Path: path}
}

// StaleCommitError represents an error when a locked commit no longer exists
type StaleCommitError struct {
	CommitHash string
}

func (e *StaleCommitError) Error() string {
	hash := e.CommitHash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return fmt.Sprintf(ErrStaleCommitMsg, hash)
}

// NewStaleCommitError creates a new StaleCommitError
func NewStaleCommitError(commitHash string) *StaleCommitError {
	return &StaleCommitError{CommitHash: commitHash}
}

// CheckoutError represents an error during git checkout
type CheckoutError struct {
	Target string
	Cause  error
}

func (e *CheckoutError) Error() string {
	return fmt.Sprintf("checkout locked hash %s failed: %v", e.Target, e.Cause)
}

func (e *CheckoutError) Unwrap() error {
	return e.Cause
}

// NewCheckoutError creates a new CheckoutError
func NewCheckoutError(target string, cause error) *CheckoutError {
	return &CheckoutError{Target: target, Cause: cause}
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	VendorName string
	Ref        string
	Message    string
}

func (e *ValidationError) Error() string {
	if e.Ref != "" {
		return fmt.Sprintf("vendor %s @ %s: %s", e.VendorName, e.Ref, e.Message)
	}
	if e.VendorName != "" {
		return fmt.Sprintf("vendor %s: %s", e.VendorName, e.Message)
	}
	return e.Message
}

// NewValidationError creates a new ValidationError
func NewValidationError(vendorName, ref, message string) *ValidationError {
	return &ValidationError{
		VendorName: vendorName,
		Ref:        ref,
		Message:    message,
	}
}

// IsVendorNotFound checks if an error is a VendorNotFoundError
func IsVendorNotFound(err error) bool {
	var vnf *VendorNotFoundError
	return errors.As(err, &vnf)
}

// IsGroupNotFound checks if an error is a GroupNotFoundError
func IsGroupNotFound(err error) bool {
	var gnf *GroupNotFoundError
	return errors.As(err, &gnf)
}

// IsStaleCommit checks if an error is a StaleCommitError
func IsStaleCommit(err error) bool {
	var sce *StaleCommitError
	return errors.As(err, &sce)
}

// IsCheckoutError checks if an error is a CheckoutError
func IsCheckoutError(err error) bool {
	var ce *CheckoutError
	return errors.As(err, &ce)
}
