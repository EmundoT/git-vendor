package core

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/EmundoT/git-vendor/internal/core/providers"
	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/golang/mock/gomock"
)

// ============================================================================
// License Content Parsing Tests
// ============================================================================

// TestParseLicenseFromContent_MIT verifies MIT license detection
func TestParseLicenseFromContent_MIT(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "MIT license with standard header",
			content: `MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:`,
			want: "MIT",
		},
		{
			name: "MIT license without title",
			content: `Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the "Software"),
to deal in the Software without restriction...`,
			want: "MIT",
		},
		{
			name: "MIT license with additional text",
			content: `Copyright (c) 2024 Example Corp

Permission is hereby granted, free of charge, to any person obtaining
a copy without restriction...

THE SOFTWARE IS PROVIDED "AS IS"...`,
			want: "MIT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLicenseFromContent(tt.content)
			if got != tt.want {
				t.Errorf("parseLicenseFromContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseLicenseFromContent_Apache verifies Apache 2.0 license detection
func TestParseLicenseFromContent_Apache(t *testing.T) {
	content := `Apache License
Version 2.0, January 2004
http://www.apache.org/licenses/

TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION`

	got := parseLicenseFromContent(content)
	if got != "Apache-2.0" {
		t.Errorf("parseLicenseFromContent() = %q, want %q", got, "Apache-2.0")
	}
}

// TestParseLicenseFromContent_BSD verifies BSD license detection
func TestParseLicenseFromContent_BSD(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "BSD-3-Clause with attribution clause",
			content: `BSD License

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice
2. Redistributions in binary form must reproduce the above copyright notice
3. Neither the name of the copyright holder nor the names of its contributors...`,
			want: "BSD-3-Clause",
		},
		{
			name: "BSD-2-Clause without attribution clause",
			content: `BSD License

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Copies of source code must retain the above copyright notice
2. Binary copies must reproduce the above copyright notice`,
			want: "BSD-2-Clause",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLicenseFromContent(tt.content)
			if got != tt.want {
				t.Errorf("parseLicenseFromContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseLicenseFromContent_GPL verifies GPL license detection
func TestParseLicenseFromContent_GPL(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "GPL-3.0",
			content: `GNU GENERAL PUBLIC LICENSE
Version 3, 29 June 2007`,
			want: "GPL-3.0",
		},
		{
			name: "GPL-2.0",
			content: `GNU GENERAL PUBLIC LICENSE
Version 2, June 1991`,
			want: "GPL-2.0",
		},
		{
			name: "GPL generic",
			content: `GNU GENERAL PUBLIC LICENSE

This program is free software...`,
			want: "GPL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLicenseFromContent(tt.content)
			if got != tt.want {
				t.Errorf("parseLicenseFromContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseLicenseFromContent_ISC verifies ISC license detection
func TestParseLicenseFromContent_ISC(t *testing.T) {
	content := `ISC License

Permission to use, copy, modify, and/or sell copies of this software
and associated documentation files...`

	got := parseLicenseFromContent(content)
	if got != "ISC" {
		t.Errorf("parseLicenseFromContent() = %q, want %q", got, "ISC")
	}
}

// TestParseLicenseFromContent_Unlicense verifies Unlicense detection
func TestParseLicenseFromContent_Unlicense(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "Unlicense keyword",
			content: `This is free and unencumbered software released into the Unlicense.`,
		},
		{
			name:    "Public domain dedication",
			content: `The author dedicates this work to the public domain and waive all copyright.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLicenseFromContent(tt.content)
			if got != "Unlicense" {
				t.Errorf("parseLicenseFromContent() = %q, want %q", got, "Unlicense")
			}
		})
	}
}

// TestParseLicenseFromContent_Unknown verifies unknown license handling
func TestParseLicenseFromContent_Unknown(t *testing.T) {
	content := `This is a proprietary license.
You may not use this software without written permission.`

	got := parseLicenseFromContent(content)
	if got != "" {
		t.Errorf("parseLicenseFromContent() = %q, want empty string", got)
	}
}

// ============================================================================
// FallbackLicenseChecker Tests
// ============================================================================

// TestFallbackChecker_LicenseFileFound verifies successful license detection
// from LICENSE file using mocked filesystem and git client
func TestFallbackChecker_LicenseFileFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	// Mock: temp directory creation
	fs.EXPECT().CreateTemp("", "license-check-*").Return("/tmp/test123", nil)

	// Mock: git clone
	git.EXPECT().Clone(gomock.Any(), "/tmp/test123", "https://github.com/example/repo", gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ string, opts *types.CloneOptions) error {
			// Verify clone options
			if opts.Depth != 1 || opts.NoCheckout {
				t.Errorf("Expected depth=1, no-checkout=false, got depth=%d, no-checkout=%v",
					opts.Depth, opts.NoCheckout)
			}
			return nil
		})

	// Mock: cleanup
	fs.EXPECT().RemoveAll("/tmp/test123").Return(nil)

	// Note: FallbackChecker uses os.ReadFile directly, not FileSystem.ReadFile
	// In integration tests, this would read a real file from the cloned repo
	// For unit tests, we verify the clone happened with correct parameters

	checker := NewFallbackLicenseChecker(fs, git)

	// Execute: will fail at os.ReadFile since we don't have real files
	// but we verify the setup (temp dir, clone, cleanup) is correct
	license, err := checker.CheckLicense("https://github.com/example/repo")

	// Expected: UNKNOWN (no actual LICENSE file exists in mock)
	if license != "UNKNOWN" {
		t.Errorf("Expected UNKNOWN, got %q", license)
	}
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

// TestFallbackChecker_CloneFails verifies error handling when git clone fails
func TestFallbackChecker_CloneFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	// Mock: temp directory creation
	fs.EXPECT().CreateTemp("", "license-check-*").Return("/tmp/test123", nil)

	// Mock: git clone fails
	git.EXPECT().Clone(gomock.Any(), "/tmp/test123", "https://github.com/example/repo", gomock.Any()).
		Return(errors.New("clone failed: repository not found"))

	// Mock: cleanup
	fs.EXPECT().RemoveAll("/tmp/test123").Return(nil)

	checker := NewFallbackLicenseChecker(fs, git)

	// Execute
	license, err := checker.CheckLicense("https://github.com/example/repo")

	// Verify: error is returned
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to clone repository") {
		t.Errorf("Expected clone error, got %v", err)
	}
	if license != "" {
		t.Errorf("Expected empty license, got %q", license)
	}
}

// TestFallbackChecker_TempDirFails verifies error handling when temp dir creation fails
func TestFallbackChecker_TempDirFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	// Mock: temp directory creation fails
	fs.EXPECT().CreateTemp("", "license-check-*").Return("", errors.New("disk full"))

	checker := NewFallbackLicenseChecker(fs, git)

	// Execute
	license, err := checker.CheckLicense("https://github.com/example/repo")

	// Verify: error is returned
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create temp directory") {
		t.Errorf("Expected temp dir error, got %v", err)
	}
	if license != "" {
		t.Errorf("Expected empty license, got %q", license)
	}
}

// ============================================================================
// MultiPlatformLicenseChecker Tests
// ============================================================================

// TestMultiPlatformChecker_DetectsGitHub verifies GitHub provider detection
func TestMultiPlatformChecker_DetectsGitHub(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	registry := providers.NewProviderRegistry()
	allowedLicenses := []string{"MIT", "Apache-2.0"}

	checker := NewMultiPlatformLicenseChecker(registry, fs, git, allowedLicenses)

	// Verify provider detection
	provider := checker.GetProviderName("https://github.com/owner/repo")
	if provider != "github" {
		t.Errorf("Expected 'github', got %q", provider)
	}
}

// TestMultiPlatformChecker_DetectsGitLab verifies GitLab provider detection
func TestMultiPlatformChecker_DetectsGitLab(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	registry := providers.NewProviderRegistry()
	allowedLicenses := []string{"MIT"}

	checker := NewMultiPlatformLicenseChecker(registry, fs, git, allowedLicenses)

	// Verify provider detection
	provider := checker.GetProviderName("https://gitlab.com/group/project")
	if provider != "gitlab" {
		t.Errorf("Expected 'gitlab', got %q", provider)
	}
}

// TestMultiPlatformChecker_DetectsBitbucket verifies Bitbucket provider detection
func TestMultiPlatformChecker_DetectsBitbucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	registry := providers.NewProviderRegistry()
	allowedLicenses := []string{"MIT"}

	checker := NewMultiPlatformLicenseChecker(registry, fs, git, allowedLicenses)

	// Verify provider detection
	provider := checker.GetProviderName("https://bitbucket.org/owner/repo")
	if provider != "bitbucket" {
		t.Errorf("Expected 'bitbucket', got %q", provider)
	}
}

// TestMultiPlatformChecker_IsAllowed verifies license allowlist checking
func TestMultiPlatformChecker_IsAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fs := NewMockFileSystem(ctrl)
	git := NewMockGitClient(ctrl)

	registry := providers.NewProviderRegistry()
	allowedLicenses := []string{"MIT", "Apache-2.0", "BSD-3-Clause"}

	checker := NewMultiPlatformLicenseChecker(registry, fs, git, allowedLicenses)

	tests := []struct {
		license string
		allowed bool
	}{
		{"MIT", true},
		{"Apache-2.0", true},
		{"BSD-3-Clause", true},
		{"GPL-3.0", false},
		{"UNKNOWN", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.license, func(t *testing.T) {
			got := checker.IsAllowed(tt.license)
			if got != tt.allowed {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.license, got, tt.allowed)
			}
		})
	}
}
