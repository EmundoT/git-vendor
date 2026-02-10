package core

// File and directory names
const (
	// VendorDir is the root directory for all vendor-related files.
	// Uses dotfile convention to avoid clashing with Go's vendor/ directory
	// and similar conventions in Ruby (Bundler), PHP (Composer), Rust (cargo vendor).
	VendorDir = ".git-vendor"
	// ConfigFile is the vendor configuration filename
	ConfigFile = "vendor.yml"
	// LockFile is the vendor lock filename
	LockFile = "vendor.lock"
	// LicensesDir is the directory containing cached license files
	LicensesDir = "licenses"
	// CacheDir is the directory for incremental sync cache
	CacheDir = ".cache"
)

// Full paths relative to project root.
// Use these instead of manually concatenating VendorDir + "/" + filename.
const (
	// ConfigPath is the full path to vendor.yml
	ConfigPath = VendorDir + "/" + ConfigFile
	// LockPath is the full path to vendor.lock
	LockPath = VendorDir + "/" + LockFile
	// LicensesPath is the full path to the licenses directory
	LicensesPath = VendorDir + "/" + LicensesDir
	// CachePath is the full path to the cache directory
	CachePath = VendorDir + "/" + CacheDir
)

// Git refs
const (
	// DefaultRef is the default git ref used when none is specified
	DefaultRef = "main"
	// FetchHead is the git FETCH_HEAD reference
	FetchHead = "FETCH_HEAD"
)

// AllowedLicenses defines the list of open-source licenses permitted by default.
// AllowedLicenses uses SPDX license identifiers.
var AllowedLicenses = []string{
	"MIT",
	"Apache-2.0",
	"BSD-3-Clause",
	"BSD-2-Clause",
	"ISC",
	"Unlicense",
	"CC0-1.0",
}

// LicenseFileNames lists standard filenames checked when searching for repository licenses.
// LicenseFileNames entries are checked in order when detecting licenses via file content.
var LicenseFileNames = []string{
	"LICENSE",
	"LICENSE.txt",
	"LICENSE.md",
	"COPYING",
}
