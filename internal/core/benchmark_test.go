package core

import (
	"testing"

	"git-vendor/internal/types"
)

// ============================================================================
// Benchmark Tests for Performance Regression Detection
// ============================================================================

// BenchmarkParseSmartURL_GitHub benchmarks GitHub URL parsing
func BenchmarkParseSmartURL_GitHub(b *testing.B) {
	url := "https://github.com/owner/repo/blob/main/src/file.go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseSmartURL(url)
	}
}

// BenchmarkParseSmartURL_GitLab benchmarks GitLab URL parsing with nested groups
func BenchmarkParseSmartURL_GitLab(b *testing.B) {
	url := "https://gitlab.com/group/subgroup/project/-/blob/main/lib/module.go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseSmartURL(url)
	}
}

// BenchmarkParseSmartURL_Generic benchmarks generic git URL parsing (baseline)
func BenchmarkParseSmartURL_Generic(b *testing.B) {
	url := "https://git.example.com/repo.git"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParseSmartURL(url)
	}
}

// BenchmarkValidateDestPath_Safe benchmarks typical safe path validation
func BenchmarkValidateDestPath_Safe(b *testing.B) {
	path := "vendor/lib/module/file.go"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateDestPath(path)
	}
}

// BenchmarkValidateDestPath_Malicious benchmarks security check performance
func BenchmarkValidateDestPath_Malicious(b *testing.B) {
	paths := []string{
		"../../../etc/passwd",
		"/etc/passwd",
		"C:\\Windows\\System32",
		"vendor/../../../etc/shadow",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			_ = ValidateDestPath(path)
		}
	}
}

// BenchmarkValidateVendorSpec benchmarks single vendor spec validation
func BenchmarkValidateVendorSpec(b *testing.B) {
	spec := types.VendorSpec{
		Name:    "example",
		URL:     "https://github.com/owner/repo",
		License: "MIT",
		Specs: []types.BranchSpec{
			{
				Ref: "main",
				Mapping: []types.PathMapping{
					{From: "src", To: "vendor/example"},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark individual validation steps
		_ = spec.Name != ""
		_ = spec.URL != ""
		_ = len(spec.Specs) > 0
	}
}

// BenchmarkCleanURL benchmarks URL cleaning/normalization
func BenchmarkCleanURL(b *testing.B) {
	urls := []string{
		"  https://github.com/owner/repo  ",
		"\\\\path\\to\\repo",
		"https://github.com/owner/repo/",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range urls {
			_ = cleanURL(url)
		}
	}
}

// BenchmarkPathMappingComparison benchmarks path comparison operations
func BenchmarkPathMappingComparison(b *testing.B) {
	mappings := []types.PathMapping{
		{From: "src", To: "vendor/a/src"},
		{From: "lib", To: "vendor/a/lib"},
		{From: "docs", To: "vendor/a/docs"},
		{From: "tests", To: "vendor/a/tests"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark path comparison logic
		for j := 0; j < len(mappings); j++ {
			for k := j + 1; k < len(mappings); k++ {
				_ = mappings[j].To == mappings[k].To
			}
		}
	}
}

// BenchmarkParseLicenseFromContent benchmarks license pattern matching
func BenchmarkParseLicenseFromContent(b *testing.B) {
	mitLicense := `MIT License

Copyright (c) 2024 Example Corp

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software.`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseLicenseFromContent(mitLicense)
	}
}
