package git

import (
	"testing"
)

// TestParseLsRemoteOutput_SingleBranchRef verifies parsing of a single branch reference.
func TestParseLsRemoteOutput_SingleBranchRef(t *testing.T) {
	output := "abc123def456789012345678901234567890abcd\trefs/heads/main"
	hash, err := ParseLsRemoteOutput(output, "main")
	if err != nil {
		t.Fatalf("ParseLsRemoteOutput returned error: %v", err)
	}
	if hash != "abc123def456789012345678901234567890abcd" {
		t.Errorf("expected abc123def456789012345678901234567890abcd, got %s", hash)
	}
}

// TestParseLsRemoteOutput_AnnotatedTag verifies that ^{} (dereferenced) entries are preferred.
func TestParseLsRemoteOutput_AnnotatedTag(t *testing.T) {
	output := "aaa1111111111111111111111111111111111111a\trefs/tags/v1.0.0\n" +
		"bbb2222222222222222222222222222222222222b\trefs/tags/v1.0.0^{}"
	hash, err := ParseLsRemoteOutput(output, "v1.0.0")
	if err != nil {
		t.Fatalf("ParseLsRemoteOutput returned error: %v", err)
	}
	if hash != "bbb2222222222222222222222222222222222222b" {
		t.Errorf("expected bbb2222222222222222222222222222222222b (dereferenced), got %s", hash)
	}
}

// TestParseLsRemoteOutput_EmptyOutput verifies error on empty output.
func TestParseLsRemoteOutput_EmptyOutput(t *testing.T) {
	_, err := ParseLsRemoteOutput("", "main")
	if err == nil {
		t.Fatal("expected error for empty output, got nil")
	}
}

// TestParseLsRemoteOutput_WhitespaceOnly verifies error on whitespace-only output.
func TestParseLsRemoteOutput_WhitespaceOnly(t *testing.T) {
	_, err := ParseLsRemoteOutput("   \n  \t  ", "main")
	if err == nil {
		t.Fatal("expected error for whitespace-only output, got nil")
	}
}

// TestParseLsRemoteOutput_NoMatchingRef verifies error when lines have no valid hash+ref pairs.
func TestParseLsRemoteOutput_NoMatchingRef(t *testing.T) {
	// A line with only one field (no tab-separated ref)
	_, err := ParseLsRemoteOutput("justahash", "main")
	if err == nil {
		t.Fatal("expected error for malformed output, got nil")
	}
}

// TestParseLsRemoteOutput_MultipleRefs verifies that the first matching ref is returned
// when no ^{} entry exists.
func TestParseLsRemoteOutput_MultipleRefs(t *testing.T) {
	output := "aaa1111111111111111111111111111111111111a\trefs/heads/main\n" +
		"bbb2222222222222222222222222222222222222b\trefs/heads/develop"
	hash, err := ParseLsRemoteOutput(output, "main")
	if err != nil {
		t.Fatalf("ParseLsRemoteOutput returned error: %v", err)
	}
	if hash != "aaa1111111111111111111111111111111111111a" {
		t.Errorf("expected first ref hash, got %s", hash)
	}
}

// TestParseLsRemoteOutput_LightweightTag verifies parsing of a lightweight tag (no ^{}).
func TestParseLsRemoteOutput_LightweightTag(t *testing.T) {
	output := "ccc3333333333333333333333333333333333333c\trefs/tags/v2.0.0"
	hash, err := ParseLsRemoteOutput(output, "v2.0.0")
	if err != nil {
		t.Fatalf("ParseLsRemoteOutput returned error: %v", err)
	}
	if hash != "ccc3333333333333333333333333333333333333c" {
		t.Errorf("expected ccc3333333333333333333333333333333333c, got %s", hash)
	}
}
