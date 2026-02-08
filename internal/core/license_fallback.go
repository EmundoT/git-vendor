package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// FallbackLicenseChecker implements license detection by reading LICENSE files
// This is used when no API is available (Bitbucket, generic git, or API failures)
type FallbackLicenseChecker struct {
	fs        FileSystem
	gitClient GitClient
}

// NewFallbackLicenseChecker creates a new fallback license checker
func NewFallbackLicenseChecker(fs FileSystem, gitClient GitClient) *FallbackLicenseChecker {
	return &FallbackLicenseChecker{
		fs:        fs,
		gitClient: gitClient,
	}
}

// CheckLicense reads LICENSE file from repository and attempts to detect license
func (c *FallbackLicenseChecker) CheckLicense(repoURL string) (string, error) {
	// Create temp directory for shallow clone
	tempDir, err := c.fs.CreateTemp("", "license-check-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = c.fs.RemoveAll(tempDir) }() //nolint:errcheck // cleanup in defer

	// Shallow clone (depth=1) to minimize download
	opts := &types.CloneOptions{
		Depth:      1,
		NoCheckout: false, // Need to checkout to read files
	}

	if err := c.gitClient.Clone(context.Background(), tempDir, repoURL, opts); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	// Try common license file names
	licenseFiles := []string{
		"LICENSE",
		"LICENSE.txt",
		"LICENSE.md",
		"COPYING",
		"COPYING.txt",
		"COPYING.md",
		"LICENSE-MIT",
		"LICENSE-APACHE",
		"LICENCE", // British spelling
		"LICENCE.txt",
		"LICENCE.md",
	}

	for _, filename := range licenseFiles {
		path := filepath.Join(tempDir, filename)
		// Use os.ReadFile directly (FileSystem interface doesn't have ReadFile)
		content, err := os.ReadFile(path)
		if err != nil {
			continue // File doesn't exist, try next
		}

		// Parse license from file content
		license := parseLicenseFromContent(string(content))
		if license != "" {
			return license, nil
		}
	}

	// Could not detect license from any file
	return "UNKNOWN", nil
}

// parseLicenseFromContent analyzes LICENSE file content to detect license type
// Uses pattern matching for common licenses
func parseLicenseFromContent(content string) string {
	lower := strings.ToLower(content)

	// Pattern matching for common licenses
	// Order matters: check more specific patterns first

	// Apache 2.0
	if strings.Contains(lower, "apache license") &&
		strings.Contains(lower, "version 2.0") {
		return "Apache-2.0"
	}

	// MIT License
	if strings.Contains(lower, "mit license") ||
		(strings.Contains(lower, "permission is hereby granted, free of charge") &&
			strings.Contains(lower, "without restriction")) {
		return "MIT"
	}

	// BSD Licenses
	if strings.Contains(lower, "bsd") && strings.Contains(lower, "redistribution") {
		// Count numbered clauses to distinguish BSD-2 from BSD-3
		if strings.Contains(lower, "3. neither the name") ||
			strings.Count(lower, "redistribution") >= 2 {
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"
	}

	// GNU GPL
	if strings.Contains(lower, "gnu general public license") {
		if strings.Contains(lower, "version 3") {
			return "GPL-3.0"
		}
		if strings.Contains(lower, "version 2") {
			return "GPL-2.0"
		}
		return "GPL" // Generic GPL
	}

	// GNU LGPL
	if strings.Contains(lower, "gnu lesser general public license") {
		if strings.Contains(lower, "version 3") {
			return "LGPL-3.0"
		}
		if strings.Contains(lower, "version 2") {
			return "LGPL-2.1"
		}
		return "LGPL"
	}

	// Mozilla Public License
	if strings.Contains(lower, "mozilla public license") {
		if strings.Contains(lower, "version 2.0") {
			return "MPL-2.0"
		}
		return "MPL"
	}

	// ISC License
	if strings.Contains(lower, "isc license") ||
		(strings.Contains(lower, "permission to use, copy, modify") &&
			strings.Contains(lower, "and/or sell copies")) {
		return "ISC"
	}

	// Unlicense / Public Domain
	if strings.Contains(lower, "unlicense") ||
		(strings.Contains(lower, "public domain") &&
			strings.Contains(lower, "waive all copyright")) {
		return "Unlicense"
	}

	// Creative Commons Zero (CC0)
	if strings.Contains(lower, "cc0") ||
		(strings.Contains(lower, "creative commons") &&
			strings.Contains(lower, "public domain dedication")) {
		return "CC0-1.0"
	}

	// Boost Software License
	if strings.Contains(lower, "boost software license") {
		return "BSL-1.0"
	}

	// WTFPL
	if strings.Contains(lower, "do what the fuck you want") {
		return "WTFPL"
	}

	// Could not detect license
	return ""
}
