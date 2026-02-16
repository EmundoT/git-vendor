package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/EmundoT/git-vendor/internal/types"
)

// CreateVendorEntry adds a new vendor to vendor.yml without triggering sync or update.
// CreateVendorEntry is the non-interactive counterpart to AddVendor — suitable for LLM and scripted workflows.
func (s *VendorSyncer) CreateVendorEntry(name, url, ref, license string) error {
	if name == "" {
		return fmt.Errorf("vendor name is required")
	}
	if url == "" {
		return fmt.Errorf("vendor URL is required")
	}

	exists, err := s.repository.Exists(name)
	if err == nil && exists {
		return fmt.Errorf("vendor '%s' already exists", name)
	}

	if ref == "" {
		ref = "main"
	}

	spec := &types.VendorSpec{
		Name:    name,
		URL:     url,
		License: license,
		Specs: []types.BranchSpec{
			{
				Ref:     ref,
				Mapping: []types.PathMapping{},
			},
		},
	}

	return s.repository.Save(spec)
}

// RenameVendor renames a vendor in config, lockfile, and license file.
func (s *VendorSyncer) RenameVendor(oldName, newName string) error {
	if oldName == "" || newName == "" {
		return fmt.Errorf("both old and new vendor names are required")
	}
	if oldName == newName {
		return fmt.Errorf("old and new names are identical")
	}

	// Check new name doesn't already exist
	newExists, err := s.repository.Exists(newName)
	if err == nil && newExists {
		return fmt.Errorf("vendor '%s' already exists", newName)
	}

	// Load and modify config
	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	idx := FindVendorIndex(cfg.Vendors, oldName)
	if idx < 0 {
		return NewVendorNotFoundError(oldName)
	}
	cfg.Vendors[idx].Name = newName

	if err := s.configStore.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Update lockfile entries (best-effort — lockfile may not exist)
	lock, err := s.lockStore.Load()
	if err == nil {
		changed := false
		for i := range lock.Vendors {
			if lock.Vendors[i].Name == oldName {
				lock.Vendors[i].Name = newName
				changed = true
			}
		}
		if changed {
			if saveErr := s.lockStore.Save(lock); saveErr != nil {
				return fmt.Errorf("save lockfile: %w", saveErr)
			}
		}
	}

	// Rename license file (best-effort)
	oldLicense := filepath.Join(s.rootDir, LicensesDir, oldName+".txt")
	newLicense := filepath.Join(s.rootDir, LicensesDir, newName+".txt")
	_ = os.Rename(oldLicense, newLicense) //nolint:errcheck

	return nil
}

// AddMappingToVendor adds a path mapping to an existing vendor's ref.
// If ref is empty, the mapping is added to the first (default) BranchSpec.
func (s *VendorSyncer) AddMappingToVendor(vendorName, from, to, ref string) error {
	if vendorName == "" || from == "" {
		return fmt.Errorf("vendor name and source path are required")
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	idx := FindVendorIndex(cfg.Vendors, vendorName)
	if idx < 0 {
		return NewVendorNotFoundError(vendorName)
	}

	vendor := &cfg.Vendors[idx]
	if len(vendor.Specs) == 0 {
		return fmt.Errorf("vendor '%s' has no specs configured", vendorName)
	}

	// Find the target BranchSpec
	specIdx := 0
	if ref != "" {
		specIdx = -1
		for i, s := range vendor.Specs {
			if s.Ref == ref {
				specIdx = i
				break
			}
		}
		if specIdx < 0 {
			return fmt.Errorf("ref '%s' not found in vendor '%s'", ref, vendorName)
		}
	}

	// Check for duplicate mapping
	for _, m := range vendor.Specs[specIdx].Mapping {
		if m.From == from {
			return fmt.Errorf("mapping from '%s' already exists in vendor '%s'", from, vendorName)
		}
	}

	vendor.Specs[specIdx].Mapping = append(vendor.Specs[specIdx].Mapping, types.PathMapping{
		From: from,
		To:   to,
	})

	return s.configStore.Save(cfg)
}

// RemoveMappingFromVendor removes a path mapping from a vendor by its source path.
// RemoveMappingFromVendor searches all specs (refs) within the vendor.
func (s *VendorSyncer) RemoveMappingFromVendor(vendorName, from string) error {
	if vendorName == "" || from == "" {
		return fmt.Errorf("vendor name and source path are required")
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	idx := FindVendorIndex(cfg.Vendors, vendorName)
	if idx < 0 {
		return NewVendorNotFoundError(vendorName)
	}

	vendor := &cfg.Vendors[idx]
	found := false
	for si := range vendor.Specs {
		for mi, m := range vendor.Specs[si].Mapping {
			if m.From == from {
				vendor.Specs[si].Mapping = append(
					vendor.Specs[si].Mapping[:mi],
					vendor.Specs[si].Mapping[mi+1:]...,
				)
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("mapping from '%s' not found in vendor '%s'", from, vendorName)
	}

	return s.configStore.Save(cfg)
}

// UpdateMappingInVendor changes the destination of an existing mapping.
func (s *VendorSyncer) UpdateMappingInVendor(vendorName, from, newTo string) error {
	if vendorName == "" || from == "" {
		return fmt.Errorf("vendor name and source path are required")
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	idx := FindVendorIndex(cfg.Vendors, vendorName)
	if idx < 0 {
		return NewVendorNotFoundError(vendorName)
	}

	vendor := &cfg.Vendors[idx]
	found := false
	for si := range vendor.Specs {
		for mi := range vendor.Specs[si].Mapping {
			if vendor.Specs[si].Mapping[mi].From == from {
				vendor.Specs[si].Mapping[mi].To = newTo
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("mapping from '%s' not found in vendor '%s'", from, vendorName)
	}

	return s.configStore.Save(cfg)
}

// ShowVendor returns detailed information about a vendor, combining config and lockfile data.
func (s *VendorSyncer) ShowVendor(name string) (map[string]interface{}, error) {
	vendor, err := s.repository.Find(name)
	if err != nil {
		return nil, err
	}

	// Build base vendor data
	data := map[string]interface{}{
		"name":    vendor.Name,
		"url":     vendor.URL,
		"license": vendor.License,
	}

	if len(vendor.Mirrors) > 0 {
		data["mirrors"] = vendor.Mirrors
	}

	if len(vendor.Groups) > 0 {
		data["groups"] = vendor.Groups
	}

	if vendor.Hooks != nil {
		hooks := map[string]interface{}{}
		if vendor.Hooks.PreSync != "" {
			hooks["pre_sync"] = vendor.Hooks.PreSync
		}
		if vendor.Hooks.PostSync != "" {
			hooks["post_sync"] = vendor.Hooks.PostSync
		}
		if len(hooks) > 0 {
			data["hooks"] = hooks
		}
	}

	// Build specs with lockfile metadata
	lock, _ := s.lockStore.Load() //nolint:errcheck
	lockMap := make(map[string]*types.LockDetails)
	for i := range lock.Vendors {
		key := lock.Vendors[i].Name + "@" + lock.Vendors[i].Ref
		lockMap[key] = &lock.Vendors[i]
	}

	specsData := make([]map[string]interface{}, 0, len(vendor.Specs))
	totalMappings := 0
	for _, spec := range vendor.Specs {
		specData := map[string]interface{}{
			"ref": spec.Ref,
		}
		if spec.DefaultTarget != "" {
			specData["default_target"] = spec.DefaultTarget
		}

		mappingsData := make([]map[string]interface{}, 0, len(spec.Mapping))
		for _, m := range spec.Mapping {
			mappingsData = append(mappingsData, map[string]interface{}{
				"from": m.From,
				"to":   m.To,
			})
		}
		specData["mappings"] = mappingsData
		totalMappings += len(spec.Mapping)

		// Add lockfile metadata
		if entry, ok := lockMap[vendor.Name+"@"+spec.Ref]; ok {
			specData["commit_hash"] = entry.CommitHash
			if entry.LicenseSPDX != "" {
				specData["license_spdx"] = entry.LicenseSPDX
			}
			if entry.SourceVersionTag != "" {
				specData["source_version_tag"] = entry.SourceVersionTag
			}
			if entry.VendoredAt != "" {
				specData["vendored_at"] = entry.VendoredAt
			}
			if entry.VendoredBy != "" {
				specData["vendored_by"] = entry.VendoredBy
			}
			if entry.LastSyncedAt != "" {
				specData["last_synced_at"] = entry.LastSyncedAt
			}
			if len(entry.Positions) > 0 {
				posData := make([]map[string]interface{}, 0, len(entry.Positions))
				for _, p := range entry.Positions {
					posData = append(posData, map[string]interface{}{
						"from":        p.From,
						"to":          p.To,
						"source_hash": p.SourceHash,
					})
				}
				specData["positions"] = posData
			}
			if entry.SourceURL != "" {
				specData["source_url"] = entry.SourceURL
			}
		}

		specsData = append(specsData, specData)
	}

	data["specs"] = specsData
	data["mapping_count"] = totalMappings

	return data, nil
}

// GetConfigValue retrieves a config value by dotted key path.
// Supported keys: vendors.<name>.url, vendors.<name>.license, vendors.<name>.ref,
// vendors.<name>.groups, vendor_count.
func (s *VendorSyncer) GetConfigValue(key string) (interface{}, error) {
	cfg, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Handle top-level keys
	switch key {
	case "vendor_count":
		return len(cfg.Vendors), nil
	case "vendors":
		names := make([]string, len(cfg.Vendors))
		for i, v := range cfg.Vendors {
			names[i] = v.Name
		}
		return names, nil
	}

	// Handle vendor-level keys: vendors.<name>.<field>
	if !strings.HasPrefix(key, "vendors.") {
		return nil, fmt.Errorf("unknown config key: %s", key)
	}

	parts := strings.SplitN(key, ".", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid key format: %s (expected vendors.<name>[.<field>])", key)
	}

	vendorName := parts[1]
	vendor := FindVendor(cfg.Vendors, vendorName)
	if vendor == nil {
		return nil, NewVendorNotFoundError(vendorName)
	}

	if len(parts) == 2 {
		// Return the whole vendor as a map
		data := map[string]interface{}{
			"name":    vendor.Name,
			"url":     vendor.URL,
			"license": vendor.License,
			"groups":  vendor.Groups,
		}
		if len(vendor.Mirrors) > 0 {
			data["mirrors"] = vendor.Mirrors
		}
		return data, nil
	}

	field := parts[2]
	switch field {
	case "name":
		return vendor.Name, nil
	case "url":
		return vendor.URL, nil
	case "license":
		return vendor.License, nil
	case "groups":
		return vendor.Groups, nil
	case "mirrors":
		return vendor.Mirrors, nil
	case "ref":
		if len(vendor.Specs) > 0 {
			return vendor.Specs[0].Ref, nil
		}
		return "", nil
	default:
		return nil, fmt.Errorf("unknown vendor field: %s (valid: name, url, license, groups, mirrors, ref)", field)
	}
}

// SetConfigValue sets a config value by dotted key path.
// Supported keys: vendors.<name>.url, vendors.<name>.license, vendors.<name>.ref.
func (s *VendorSyncer) SetConfigValue(key, value string) error {
	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !strings.HasPrefix(key, "vendors.") {
		return fmt.Errorf("unknown config key: %s (only vendors.<name>.<field> keys are settable)", key)
	}

	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid key format: %s (expected vendors.<name>.<field>)", key)
	}

	vendorName := parts[1]
	idx := FindVendorIndex(cfg.Vendors, vendorName)
	if idx < 0 {
		return NewVendorNotFoundError(vendorName)
	}

	field := parts[2]
	switch field {
	case "url":
		cfg.Vendors[idx].URL = value
	case "license":
		cfg.Vendors[idx].License = value
	case "ref":
		if len(cfg.Vendors[idx].Specs) > 0 {
			cfg.Vendors[idx].Specs[0].Ref = value
		} else {
			return fmt.Errorf("vendor '%s' has no specs to set ref on", vendorName)
		}
	case "name":
		return fmt.Errorf("use 'git-vendor rename' to change vendor names")
	default:
		return fmt.Errorf("unknown vendor field: %s (settable: url, license, ref)", field)
	}

	return s.configStore.Save(cfg)
}

// CheckVendorStatus checks the sync status of a single vendor.
func (s *VendorSyncer) CheckVendorStatus(vendorName string) (map[string]interface{}, error) {
	vendor, err := s.repository.Find(vendorName)
	if err != nil {
		return nil, err
	}

	lock, err := s.lockStore.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]interface{}{
				"vendor":  vendorName,
				"status":  "not_synced",
				"message": "no lockfile found",
			}, nil
		}
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	// Build lock map
	lockMap := make(map[string]*types.LockDetails)
	for i := range lock.Vendors {
		key := lock.Vendors[i].Name + "@" + lock.Vendors[i].Ref
		lockMap[key] = &lock.Vendors[i]
	}

	specsStatus := make([]map[string]interface{}, 0, len(vendor.Specs))
	allSynced := true

	for _, spec := range vendor.Specs {
		key := vendorName + "@" + spec.Ref
		entry, hasLock := lockMap[key]

		specStatus := map[string]interface{}{
			"ref": spec.Ref,
		}

		if !hasLock {
			specStatus["status"] = "not_locked"
			allSynced = false
			specsStatus = append(specsStatus, specStatus)
			continue
		}

		specStatus["commit_hash"] = entry.CommitHash

		// Check if files exist
		missingPaths := []string{}
		for _, m := range spec.Mapping {
			dest := m.To
			if dest == "" {
				dest = filepath.Base(m.From)
			}
			// Strip position specifier
			dest, _, _ = types.ParsePathPosition(dest)

			if _, statErr := s.fs.Stat(dest); statErr != nil {
				missingPaths = append(missingPaths, dest)
			}
		}

		if len(missingPaths) > 0 {
			specStatus["status"] = "incomplete"
			specStatus["missing_paths"] = missingPaths
			allSynced = false
		} else {
			specStatus["status"] = "synced"
		}

		specsStatus = append(specsStatus, specStatus)
	}

	status := "synced"
	if !allSynced {
		status = "stale"
	}

	return map[string]interface{}{
		"vendor": vendorName,
		"status": status,
		"specs":  specsStatus,
	}, nil
}

// AddMirror appends a mirror URL to a vendor's Mirrors slice.
// AddMirror validates the URL format and rejects duplicates (same as primary URL or already present).
func (s *VendorSyncer) AddMirror(vendorName, mirrorURL string) error {
	if vendorName == "" || mirrorURL == "" {
		return fmt.Errorf("vendor name and mirror URL are required")
	}

	if err := ValidateVendorURL(mirrorURL); err != nil {
		return fmt.Errorf("invalid mirror URL: %w", err)
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	idx := FindVendorIndex(cfg.Vendors, vendorName)
	if idx < 0 {
		return NewVendorNotFoundError(vendorName)
	}

	vendor := &cfg.Vendors[idx]

	// Reject if same as primary URL
	if vendor.URL == mirrorURL {
		return fmt.Errorf("mirror URL is the same as the primary URL")
	}

	// Reject if already in mirrors
	for _, m := range vendor.Mirrors {
		if m == mirrorURL {
			return fmt.Errorf("mirror URL '%s' already exists for vendor '%s'", mirrorURL, vendorName)
		}
	}

	vendor.Mirrors = append(vendor.Mirrors, mirrorURL)
	return s.configStore.Save(cfg)
}

// RemoveMirror removes a mirror URL from a vendor's Mirrors slice.
// RemoveMirror returns an error if the mirror URL is not found.
func (s *VendorSyncer) RemoveMirror(vendorName, mirrorURL string) error {
	if vendorName == "" || mirrorURL == "" {
		return fmt.Errorf("vendor name and mirror URL are required")
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	idx := FindVendorIndex(cfg.Vendors, vendorName)
	if idx < 0 {
		return NewVendorNotFoundError(vendorName)
	}

	vendor := &cfg.Vendors[idx]
	found := false
	for i, m := range vendor.Mirrors {
		if m == mirrorURL {
			vendor.Mirrors = append(vendor.Mirrors[:i], vendor.Mirrors[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("mirror URL '%s' not found in vendor '%s'", mirrorURL, vendorName)
	}

	return s.configStore.Save(cfg)
}

// ListMirrors returns the primary URL and all mirror URLs for a vendor.
// ListMirrors returns a map with "primary" (string) and "mirrors" ([]string) keys.
func (s *VendorSyncer) ListMirrors(vendorName string) (map[string]interface{}, error) {
	if vendorName == "" {
		return nil, fmt.Errorf("vendor name is required")
	}

	cfg, err := s.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	vendor := FindVendor(cfg.Vendors, vendorName)
	if vendor == nil {
		return nil, NewVendorNotFoundError(vendorName)
	}

	return map[string]interface{}{
		"vendor":  vendorName,
		"primary": vendor.URL,
		"mirrors": vendor.Mirrors,
	}, nil
}
