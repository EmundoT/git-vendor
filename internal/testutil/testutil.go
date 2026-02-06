// Package testutil provides shared test utilities for the git-vendor project.
// These helpers are designed for testing serialization (YAML/JSON) round-trips
// and field validation across multiple packages.
package testutil

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// Pointer Helpers
// ============================================================================

// StrPtr creates a pointer to a string - useful for optional fields in tests.
func StrPtr(s string) *string {
	return &s
}

// IntPtr creates a pointer to an int - useful for optional fields in tests.
func IntPtr(i int) *int {
	return &i
}

// BoolPtr creates a pointer to a bool - useful for optional fields in tests.
func BoolPtr(b bool) *bool {
	return &b
}

// ============================================================================
// YAML Round-Trip Assertions
// ============================================================================

// AssertYAMLRoundTrip marshals v to YAML and unmarshals back, failing if not equal.
// Uses reflect.DeepEqual for comparison.
func AssertYAMLRoundTrip[T any](t *testing.T, original T) {
	t.Helper()
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed T
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, parsed) {
		t.Errorf("round-trip mismatch:\noriginal: %+v\nparsed:   %+v", original, parsed)
	}
}

// AssertYAMLOmitsField verifies a field is not present in marshalled YAML output.
// Checks for "fieldName:" pattern in the output.
func AssertYAMLOmitsField(t *testing.T, v any, fieldName string) {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if strings.Contains(string(data), fieldName+":") {
		t.Errorf("expected field %q to be omitted from YAML output, got:\n%s", fieldName, string(data))
	}
}

// AssertYAMLContainsField verifies a field is present in marshalled YAML output.
func AssertYAMLContainsField(t *testing.T, v any, fieldName string) {
	t.Helper()
	data, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if !strings.Contains(string(data), fieldName+":") {
		t.Errorf("expected field %q to be present in YAML output, got:\n%s", fieldName, string(data))
	}
}

// ============================================================================
// JSON Round-Trip Assertions
// ============================================================================

// AssertJSONRoundTrip marshals v to JSON and unmarshals back, failing if not equal.
// Uses reflect.DeepEqual for comparison.
func AssertJSONRoundTrip[T any](t *testing.T, original T) {
	t.Helper()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed T
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, parsed) {
		t.Errorf("round-trip mismatch:\noriginal: %+v\nparsed:   %+v", original, parsed)
	}
}

// AssertJSONOmitsField verifies a field is not present in marshalled JSON output.
// Checks for "fieldName" (with quotes) pattern in the output.
func AssertJSONOmitsField(t *testing.T, v any, fieldName string) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if strings.Contains(string(data), `"`+fieldName+`"`) {
		t.Errorf("expected field %q to be omitted from JSON output, got:\n%s", fieldName, string(data))
	}
}

// AssertJSONContainsField verifies a field is present in marshalled JSON output.
func AssertJSONContainsField(t *testing.T, v any, fieldName string) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if !strings.Contains(string(data), `"`+fieldName+`"`) {
		t.Errorf("expected field %q to be present in JSON output, got:\n%s", fieldName, string(data))
	}
}

// ============================================================================
// Error Assertions
// ============================================================================

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error, got nil", msg)
	}
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: expected no error, got: %v", msg, err)
	}
}

// ============================================================================
// Equality Assertions
// ============================================================================

// AssertEqual fails the test if got != want using reflect.DeepEqual.
func AssertEqual[T any](t *testing.T, got, want T, msg string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %+v, want %+v", msg, got, want)
	}
}

// AssertNotEqual fails the test if got == want using reflect.DeepEqual.
func AssertNotEqual[T any](t *testing.T, got, notWant T, msg string) {
	t.Helper()
	if reflect.DeepEqual(got, notWant) {
		t.Errorf("%s: got %+v, should not equal %+v", msg, got, notWant)
	}
}
