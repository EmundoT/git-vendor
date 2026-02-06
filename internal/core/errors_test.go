package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// =============================================================================
// Sentinel Error Tests
// =============================================================================

func TestErrNotInitialized(t *testing.T) {
	if ErrNotInitialized == nil {
		t.Fatal("ErrNotInitialized should not be nil")
	}

	msg := ErrNotInitialized.Error()
	if !strings.Contains(msg, "vendor directory not found") {
		t.Errorf("Expected message to contain 'vendor directory not found', got: %s", msg)
	}
	if !strings.Contains(msg, "git-vendor init") {
		t.Errorf("Expected message to contain 'git-vendor init', got: %s", msg)
	}
}

func TestErrNotInitialized_ErrorsIs(t *testing.T) {
	// Test that errors.Is works with wrapped errors
	wrapped := fmt.Errorf("operation failed: %w", ErrNotInitialized)

	if !errors.Is(wrapped, ErrNotInitialized) {
		t.Error("errors.Is should match wrapped ErrNotInitialized")
	}
}

func TestErrComplianceFailed(t *testing.T) {
	if ErrComplianceFailed == nil {
		t.Fatal("ErrComplianceFailed should not be nil")
	}

	msg := ErrComplianceFailed.Error()
	if !strings.Contains(msg, "compliance") {
		t.Errorf("Expected message to contain 'compliance', got: %s", msg)
	}
}

// =============================================================================
// VendorNotFoundError Tests
// =============================================================================

func TestVendorNotFoundError_Format(t *testing.T) {
	err := NewVendorNotFoundError("my-lib")

	msg := err.Error()

	// Verify ROADMAP 9.5 format: Error/Context/Fix
	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Context:") {
		t.Error("Error message should contain 'Context:'")
	}
	if !strings.Contains(msg, "Fix:") {
		t.Error("Error message should contain 'Fix:'")
	}

	// Verify content
	if !strings.Contains(msg, "my-lib") {
		t.Error("Error message should contain vendor name")
	}
	if !strings.Contains(msg, ConfigPath) {
		t.Error("Error message should reference config path")
	}
	if !strings.Contains(msg, "git-vendor list") {
		t.Error("Error message should suggest 'git-vendor list'")
	}
}

func TestVendorNotFoundError_IsHelper(t *testing.T) {
	err := NewVendorNotFoundError("test-vendor")

	if !IsVendorNotFound(err) {
		t.Error("IsVendorNotFound should return true for VendorNotFoundError")
	}

	// Test with wrapped error
	wrapped := fmt.Errorf("operation failed: %w", err)
	if !IsVendorNotFound(wrapped) {
		t.Error("IsVendorNotFound should return true for wrapped VendorNotFoundError")
	}

	// Test with different error
	if IsVendorNotFound(errors.New("some other error")) {
		t.Error("IsVendorNotFound should return false for other errors")
	}

	if IsVendorNotFound(nil) {
		t.Error("IsVendorNotFound should return false for nil")
	}
}

func TestVendorNotFoundError_ErrorsAs(t *testing.T) {
	err := NewVendorNotFoundError("test-vendor")
	wrapped := fmt.Errorf("failed: %w", err)

	var target *VendorNotFoundError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should extract VendorNotFoundError from wrapped error")
	}

	if target.Name != "test-vendor" {
		t.Errorf("Expected Name 'test-vendor', got '%s'", target.Name)
	}
}

// =============================================================================
// GroupNotFoundError Tests
// =============================================================================

func TestGroupNotFoundError_Format(t *testing.T) {
	err := NewGroupNotFoundError("frontend")

	msg := err.Error()

	// Verify ROADMAP 9.5 format
	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Context:") {
		t.Error("Error message should contain 'Context:'")
	}
	if !strings.Contains(msg, "Fix:") {
		t.Error("Error message should contain 'Fix:'")
	}

	// Verify content
	if !strings.Contains(msg, "frontend") {
		t.Error("Error message should contain group name")
	}
}

func TestGroupNotFoundError_IsHelper(t *testing.T) {
	err := NewGroupNotFoundError("backend")

	if !IsGroupNotFound(err) {
		t.Error("IsGroupNotFound should return true for GroupNotFoundError")
	}

	wrapped := fmt.Errorf("sync failed: %w", err)
	if !IsGroupNotFound(wrapped) {
		t.Error("IsGroupNotFound should return true for wrapped error")
	}

	if IsGroupNotFound(errors.New("other")) {
		t.Error("IsGroupNotFound should return false for other errors")
	}
}

// =============================================================================
// PathNotFoundError Tests
// =============================================================================

func TestPathNotFoundError_Format(t *testing.T) {
	err := NewPathNotFoundError("src/lib/util.go", "my-vendor", "v1.0.0")

	msg := err.Error()

	// Verify ROADMAP 9.5 format
	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Context:") {
		t.Error("Error message should contain 'Context:'")
	}
	if !strings.Contains(msg, "Fix:") {
		t.Error("Error message should contain 'Fix:'")
	}

	// Verify content
	if !strings.Contains(msg, "src/lib/util.go") {
		t.Error("Error message should contain path")
	}
	if !strings.Contains(msg, "my-vendor") {
		t.Error("Error message should contain vendor name")
	}
	if !strings.Contains(msg, "v1.0.0") {
		t.Error("Error message should contain ref")
	}
}

func TestPathNotFoundError_EmptyContext(t *testing.T) {
	// Test with empty vendor/ref - should still format correctly
	err := NewPathNotFoundError("some/path", "", "")

	msg := err.Error()

	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "some/path") {
		t.Error("Error message should contain path")
	}
	// Context line with vendor@ref should be omitted when empty
	if strings.Contains(msg, "Context: The path does not exist in @") {
		t.Error("Should not show empty vendor@ref context")
	}
}

func TestPathNotFoundError_IsHelper(t *testing.T) {
	err := NewPathNotFoundError("path", "vendor", "ref")

	if !IsPathNotFound(err) {
		t.Error("IsPathNotFound should return true for PathNotFoundError")
	}

	if IsPathNotFound(errors.New("other")) {
		t.Error("IsPathNotFound should return false for other errors")
	}
}

// =============================================================================
// StaleCommitError Tests
// =============================================================================

func TestStaleCommitError_Format(t *testing.T) {
	err := NewStaleCommitError("abc123def456789", "my-vendor", "main")

	msg := err.Error()

	// Verify ROADMAP 9.5 format
	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Context:") {
		t.Error("Error message should contain 'Context:'")
	}
	if !strings.Contains(msg, "Fix:") {
		t.Error("Error message should contain 'Fix:'")
	}

	// Verify commit hash is truncated to 7 chars
	if !strings.Contains(msg, "abc123d") {
		t.Error("Error message should contain truncated commit hash")
	}
	if strings.Contains(msg, "abc123def456789") {
		t.Error("Error message should NOT contain full commit hash")
	}

	// Verify content
	if !strings.Contains(msg, "my-vendor") {
		t.Error("Error message should contain vendor name")
	}
	if !strings.Contains(msg, "main") {
		t.Error("Error message should contain ref")
	}
	if !strings.Contains(msg, "force-pushed") {
		t.Error("Error message should mention force-push possibility")
	}
}

func TestStaleCommitError_ShortHash(t *testing.T) {
	// Test with already short hash
	err := NewStaleCommitError("abc", "vendor", "ref")
	msg := err.Error()

	if !strings.Contains(msg, "abc") {
		t.Error("Short hash should be preserved as-is")
	}
}

func TestStaleCommitError_IsHelper(t *testing.T) {
	err := NewStaleCommitError("hash", "vendor", "ref")

	if !IsStaleCommit(err) {
		t.Error("IsStaleCommit should return true for StaleCommitError")
	}

	wrapped := fmt.Errorf("sync failed: %w", err)
	if !IsStaleCommit(wrapped) {
		t.Error("IsStaleCommit should return true for wrapped error")
	}
}

// =============================================================================
// CheckoutError Tests
// =============================================================================

func TestCheckoutError_Format(t *testing.T) {
	cause := errors.New("permission denied")
	err := NewCheckoutError("abc123", "my-vendor", cause)

	msg := err.Error()

	// Verify ROADMAP 9.5 format
	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Context:") {
		t.Error("Error message should contain 'Context:'")
	}
	if !strings.Contains(msg, "Fix:") {
		t.Error("Error message should contain 'Fix:'")
	}

	// Verify content
	if !strings.Contains(msg, "abc123") {
		t.Error("Error message should contain target")
	}
	if !strings.Contains(msg, "my-vendor") {
		t.Error("Error message should contain vendor name")
	}
	if !strings.Contains(msg, "permission denied") {
		t.Error("Error message should contain cause")
	}
}

func TestCheckoutError_NilCause(t *testing.T) {
	err := NewCheckoutError("target", "vendor", nil)
	msg := err.Error()

	// Should format without cause
	if !strings.Contains(msg, "target") {
		t.Error("Error message should contain target")
	}
	// Should not contain ": <nil>" or similar
	if strings.Contains(msg, "<nil>") {
		t.Error("Error message should not contain '<nil>'")
	}
}

func TestCheckoutError_Unwrap(t *testing.T) {
	cause := errors.New("network error")
	err := NewCheckoutError("ref", "vendor", cause)

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("Unwrap should return the cause")
	}

	// Test errors.Is with cause
	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the cause through Unwrap")
	}
}

func TestCheckoutError_UnwrapNil(t *testing.T) {
	err := NewCheckoutError("ref", "vendor", nil)

	if err.Unwrap() != nil {
		t.Error("Unwrap should return nil when cause is nil")
	}
}

func TestCheckoutError_IsHelper(t *testing.T) {
	err := NewCheckoutError("ref", "vendor", nil)

	if !IsCheckoutError(err) {
		t.Error("IsCheckoutError should return true for CheckoutError")
	}

	wrapped := fmt.Errorf("operation failed: %w", err)
	if !IsCheckoutError(wrapped) {
		t.Error("IsCheckoutError should return true for wrapped error")
	}
}

func TestCheckoutError_ErrorsAs(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewCheckoutError("v1.0.0", "lib", cause)
	wrapped := fmt.Errorf("checkout failed: %w", err)

	var target *CheckoutError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should extract CheckoutError")
	}

	if target.Target != "v1.0.0" {
		t.Errorf("Expected Target 'v1.0.0', got '%s'", target.Target)
	}
	if target.VendorName != "lib" {
		t.Errorf("Expected VendorName 'lib', got '%s'", target.VendorName)
	}
	if target.Cause != cause {
		t.Error("Cause should be preserved")
	}
}

// =============================================================================
// ValidationError Tests
// =============================================================================

func TestValidationError_Format(t *testing.T) {
	err := NewValidationError("my-vendor", "main", "url", "URL cannot be empty")

	msg := err.Error()

	// Verify ROADMAP 9.5 format
	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Context:") {
		t.Error("Error message should contain 'Context:'")
	}
	if !strings.Contains(msg, "Fix:") {
		t.Error("Error message should contain 'Fix:'")
	}

	// Verify content
	if !strings.Contains(msg, "my-vendor") {
		t.Error("Error message should contain vendor name")
	}
	if !strings.Contains(msg, "main") {
		t.Error("Error message should contain ref")
	}
	if !strings.Contains(msg, "url") {
		t.Error("Error message should contain field name")
	}
	if !strings.Contains(msg, "URL cannot be empty") {
		t.Error("Error message should contain message")
	}
	if !strings.Contains(msg, ConfigPath) {
		t.Error("Error message should reference config path")
	}
}

func TestValidationError_PartialFields(t *testing.T) {
	// Test with only some fields populated
	err := NewValidationError("", "", "", "Generic validation error")
	msg := err.Error()

	if !strings.HasPrefix(msg, "Error:") {
		t.Error("Error message should start with 'Error:'")
	}
	if !strings.Contains(msg, "Generic validation error") {
		t.Error("Error message should contain the message")
	}
}

func TestValidationError_IsHelper(t *testing.T) {
	err := NewValidationError("vendor", "ref", "field", "msg")

	if !IsValidationError(err) {
		t.Error("IsValidationError should return true for ValidationError")
	}

	if IsValidationError(errors.New("other")) {
		t.Error("IsValidationError should return false for other errors")
	}
}

// =============================================================================
// Cross-cutting Tests
// =============================================================================

func TestAllIsHelpers_ReturnFalseForNil(t *testing.T) {
	tests := []struct {
		name   string
		helper func(error) bool
	}{
		{"IsVendorNotFound", IsVendorNotFound},
		{"IsGroupNotFound", IsGroupNotFound},
		{"IsPathNotFound", IsPathNotFound},
		{"IsStaleCommit", IsStaleCommit},
		{"IsCheckoutError", IsCheckoutError},
		{"IsValidationError", IsValidationError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.helper(nil) {
				t.Errorf("%s should return false for nil", tt.name)
			}
		})
	}
}

func TestAllIsHelpers_ReturnFalseForUnrelatedError(t *testing.T) {
	unrelated := errors.New("unrelated error")

	tests := []struct {
		name   string
		helper func(error) bool
	}{
		{"IsVendorNotFound", IsVendorNotFound},
		{"IsGroupNotFound", IsGroupNotFound},
		{"IsPathNotFound", IsPathNotFound},
		{"IsStaleCommit", IsStaleCommit},
		{"IsCheckoutError", IsCheckoutError},
		{"IsValidationError", IsValidationError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.helper(unrelated) {
				t.Errorf("%s should return false for unrelated error", tt.name)
			}
		})
	}
}

func TestErrorTypes_ImplementErrorInterface(t *testing.T) {
	// Compile-time check that all error types implement error interface
	var _ error = &VendorNotFoundError{}
	var _ error = &GroupNotFoundError{}
	var _ error = &PathNotFoundError{}
	var _ error = &StaleCommitError{}
	var _ error = &CheckoutError{}
	var _ error = &ValidationError{}

	// Use t to satisfy linter
	t.Log("All error types implement error interface")
}
