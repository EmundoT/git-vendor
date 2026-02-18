package core

import (
	"testing"

	"github.com/EmundoT/git-vendor/internal/types"
)

func TestResolveVendorEnforcement_NilConfig(t *testing.T) {
	svc := NewEnforcementService()
	config := &types.VendorConfig{
		Vendors: []types.VendorSpec{
			{Name: "lib-a"},
			{Name: "lib-b"},
		},
	}

	result := svc.ResolveVendorEnforcement(config)
	for _, name := range []string{"lib-a", "lib-b"} {
		if result[name] != EnforcementLenient {
			t.Errorf("ResolveVendorEnforcement: nil config: %s: expected %q, got %q",
				name, EnforcementLenient, result[name])
		}
	}
}

func TestResolveVendorEnforcement_GlobalDefault(t *testing.T) {
	svc := NewEnforcementService()
	config := &types.VendorConfig{
		Compliance: &types.ComplianceConfig{Default: EnforcementStrict},
		Vendors: []types.VendorSpec{
			{Name: "lib-a"},
			{Name: "lib-b", Enforcement: EnforcementInfo},
		},
	}

	result := svc.ResolveVendorEnforcement(config)
	if result["lib-a"] != EnforcementStrict {
		t.Errorf("lib-a: expected %q (inherited), got %q", EnforcementStrict, result["lib-a"])
	}
	if result["lib-b"] != EnforcementInfo {
		t.Errorf("lib-b: expected %q (per-vendor), got %q", EnforcementInfo, result["lib-b"])
	}
}

func TestResolveVendorEnforcement_OverrideMode(t *testing.T) {
	svc := NewEnforcementService()
	config := &types.VendorConfig{
		Compliance: &types.ComplianceConfig{
			Default: EnforcementStrict,
			Mode:    ComplianceModeOverride,
		},
		Vendors: []types.VendorSpec{
			{Name: "lib-a"},
			{Name: "lib-b", Enforcement: EnforcementInfo},
		},
	}

	result := svc.ResolveVendorEnforcement(config)
	// Override mode forces global for all vendors
	if result["lib-a"] != EnforcementStrict {
		t.Errorf("lib-a: expected %q (override), got %q", EnforcementStrict, result["lib-a"])
	}
	if result["lib-b"] != EnforcementStrict {
		t.Errorf("lib-b: expected %q (override ignores per-vendor), got %q", EnforcementStrict, result["lib-b"])
	}
}

func TestComputeExitCode_StrictDrift(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-a", FilesModified: 1},
	}
	enfMap := map[string]string{"lib-a": EnforcementStrict}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 1 {
		t.Errorf("strict drift: expected exit 1, got %d", code)
	}
}

func TestComputeExitCode_LenientDrift(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-a", FilesModified: 2},
	}
	enfMap := map[string]string{"lib-a": EnforcementLenient}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 2 {
		t.Errorf("lenient drift: expected exit 2, got %d", code)
	}
}

func TestComputeExitCode_InfoDrift(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-a", FilesModified: 3},
	}
	enfMap := map[string]string{"lib-a": EnforcementInfo}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 0 {
		t.Errorf("info drift: expected exit 0, got %d", code)
	}
}

func TestComputeExitCode_MixedVendors(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-info", FilesModified: 1},
		{Name: "lib-lenient", FilesModified: 1},
		{Name: "lib-strict", FilesModified: 1},
	}
	enfMap := map[string]string{
		"lib-info":    EnforcementInfo,
		"lib-lenient": EnforcementLenient,
		"lib-strict":  EnforcementStrict,
	}
	// Strict wins
	if code := svc.ComputeExitCode(vendors, enfMap); code != 1 {
		t.Errorf("mixed vendors: expected exit 1 (strict wins), got %d", code)
	}
}

func TestComputeExitCode_NoDrift(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-a", FilesVerified: 5},
	}
	enfMap := map[string]string{"lib-a": EnforcementStrict}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 0 {
		t.Errorf("no drift: expected exit 0, got %d", code)
	}
}

func TestComputeExitCode_DeletedFiles(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-a", FilesDeleted: 1},
	}
	enfMap := map[string]string{"lib-a": EnforcementStrict}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 1 {
		t.Errorf("deleted files with strict: expected exit 1, got %d", code)
	}
}

// TestComputeExitCode_AddedOnly_NoEnforcementTrigger verifies that added-only files
// do NOT trigger enforcement exit codes. Added files are handled by the legacy summary
// (WARN), not by ComputeExitCode which only considers modified+deleted drift.
func TestComputeExitCode_AddedOnly_NoEnforcementTrigger(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-a", FilesAdded: 5},
	}
	enfMap := map[string]string{"lib-a": EnforcementStrict}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 0 {
		t.Errorf("added-only files: expected exit 0 (no enforcement trigger), got %d", code)
	}
}

// TestComputeExitCode_LenientNoDrift_StrictNoDrift verifies that when no vendor
// has drift (all verified), exit code is 0 regardless of enforcement levels.
func TestComputeExitCode_AllVerified_ExitZero(t *testing.T) {
	svc := NewEnforcementService()
	vendors := []types.VendorStatusDetail{
		{Name: "lib-strict", FilesVerified: 10},
		{Name: "lib-lenient", FilesVerified: 5},
	}
	enfMap := map[string]string{
		"lib-strict":  EnforcementStrict,
		"lib-lenient": EnforcementLenient,
	}
	if code := svc.ComputeExitCode(vendors, enfMap); code != 0 {
		t.Errorf("all verified: expected exit 0, got %d", code)
	}
}
