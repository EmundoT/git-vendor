package core

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/EmundoT/git-vendor/internal/types"
	"github.com/EmundoT/git-vendor/internal/version"
	"github.com/google/uuid"
	"github.com/spdx/tools-golang/spdx"
	"github.com/spdx/tools-golang/spdx/v2/common"
	spdx23 "github.com/spdx/tools-golang/spdx/v2/v2_3"
)

// SBOMFormat represents supported SBOM output formats
type SBOMFormat string

const (
	// SBOMFormatCycloneDX is the CycloneDX 1.5 JSON format
	SBOMFormatCycloneDX SBOMFormat = "cyclonedx"
	// SBOMFormatSPDX is the SPDX 2.3 JSON format
	SBOMFormatSPDX SBOMFormat = "spdx"
)

// SBOMGenerator generates Software Bill of Materials from vendor lockfiles
type SBOMGenerator struct {
	lockStore   LockStore
	configStore ConfigStore
	fs          FileSystem
	projectName string
}

// NewSBOMGenerator creates a new SBOMGenerator with the given dependencies
func NewSBOMGenerator(lockStore LockStore, configStore ConfigStore, fs FileSystem, projectName string) *SBOMGenerator {
	return &SBOMGenerator{
		lockStore:   lockStore,
		configStore: configStore,
		fs:          fs,
		projectName: projectName,
	}
}

// Generate creates an SBOM in the specified format
func (g *SBOMGenerator) Generate(format SBOMFormat) ([]byte, error) {
	lock, err := g.lockStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load lockfile: %w", err)
	}

	cfg, err := g.configStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Build URL map from config
	urlMap := make(map[string]string)
	for _, v := range cfg.Vendors {
		urlMap[v.Name] = v.URL
	}

	switch format {
	case SBOMFormatCycloneDX:
		return g.generateCycloneDX(&lock, urlMap)
	case SBOMFormatSPDX:
		return g.generateSPDX(&lock, urlMap)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// generateCycloneDX creates a CycloneDX 1.5 JSON SBOM
func (g *SBOMGenerator) generateCycloneDX(lock *types.VendorLock, urlMap map[string]string) ([]byte, error) {
	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()
	bom.Version = 1

	// Set metadata
	timestamp := time.Now().UTC().Format(time.RFC3339)
	toolVersion := version.GetVersion()
	bom.Metadata = &cdx.Metadata{
		Timestamp: timestamp,
		Tools: &cdx.ToolsChoice{
			Tools: &[]cdx.Tool{
				{
					Vendor:  "git-vendor",
					Name:    "git-vendor",
					Version: toolVersion,
				},
			},
		},
		Component: &cdx.Component{
			Type:    cdx.ComponentTypeApplication,
			Name:    g.projectName,
			Version: "local",
		},
	}

	// Build components from lock entries
	components := make([]cdx.Component, 0, len(lock.Vendors))
	for _, vendor := range lock.Vendors {
		repoURL := urlMap[vendor.Name]
		component := g.buildCycloneDXComponent(&vendor, repoURL)
		components = append(components, component)
	}
	bom.Components = &components

	// Encode to JSON
	encoder := cdx.NewBOMEncoder(nil, cdx.BOMFileFormatJSON)
	encoder.SetPretty(true)

	// Use a buffer to encode
	var buf strings.Builder
	encoder = cdx.NewBOMEncoder(&buf, cdx.BOMFileFormatJSON)
	encoder.SetPretty(true)
	if err := encoder.Encode(bom); err != nil {
		return nil, fmt.Errorf("encode CycloneDX: %w", err)
	}

	return []byte(buf.String()), nil
}

// buildCycloneDXComponent creates a CycloneDX component from lock details
func (g *SBOMGenerator) buildCycloneDXComponent(vendor *types.LockDetails, repoURL string) cdx.Component {
	// Determine version - prefer source_version_tag, fall back to commit hash
	componentVersion := vendor.CommitHash
	if vendor.SourceVersionTag != "" {
		componentVersion = vendor.SourceVersionTag
	}

	// Build BOM ref
	shortHash := vendor.CommitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	bomRef := fmt.Sprintf("%s@%s", vendor.Name, shortHash)

	component := cdx.Component{
		Type:       cdx.ComponentTypeLibrary,
		BOMRef:     bomRef,
		Name:       vendor.Name,
		Version:    componentVersion,
		PackageURL: g.getPURL(vendor.Name, repoURL, vendor.CommitHash),
	}

	// Add license if available
	if vendor.LicenseSPDX != "" {
		component.Licenses = &cdx.Licenses{
			{License: &cdx.License{ID: vendor.LicenseSPDX}},
		}
	}

	// Add file hashes
	if len(vendor.FileHashes) > 0 {
		hashes := make([]cdx.Hash, 0, len(vendor.FileHashes))
		for _, hash := range vendor.FileHashes {
			hashes = append(hashes, cdx.Hash{
				Algorithm: cdx.HashAlgoSHA256,
				Value:     hash,
			})
		}
		component.Hashes = &hashes
	}

	// Add external reference for VCS
	if repoURL != "" {
		component.ExternalReferences = &[]cdx.ExternalReference{
			{
				Type: cdx.ERTypeVCS,
				URL:  repoURL,
			},
		}
	}

	// Add properties for git-vendor metadata
	properties := []cdx.Property{
		{Name: "git-vendor:commit", Value: vendor.CommitHash},
		{Name: "git-vendor:ref", Value: vendor.Ref},
	}
	if vendor.VendoredAt != "" {
		properties = append(properties, cdx.Property{Name: "git-vendor:vendored_at", Value: vendor.VendoredAt})
	}
	if vendor.VendoredBy != "" {
		properties = append(properties, cdx.Property{Name: "git-vendor:vendored_by", Value: vendor.VendoredBy})
	}
	if vendor.LastSyncedAt != "" {
		properties = append(properties, cdx.Property{Name: "git-vendor:last_synced_at", Value: vendor.LastSyncedAt})
	}
	component.Properties = &properties

	return component
}

// generateSPDX creates an SPDX 2.3 JSON SBOM
func (g *SBOMGenerator) generateSPDX(lock *types.VendorLock, urlMap map[string]string) ([]byte, error) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	toolVersion := version.GetVersion()

	// Create SPDX document
	doc := &spdx23.Document{
		SPDXVersion:       spdx.Version,
		DataLicense:       spdx.DataLicense,
		SPDXIdentifier:    common.ElementID("DOCUMENT"),
		DocumentName:      g.projectName + "-vendored-sbom",
		DocumentNamespace: fmt.Sprintf("https://git-vendor.dev/spdx/%s/%s", g.projectName, uuid.New().String()),
		CreationInfo: &spdx23.CreationInfo{
			Created: timestamp,
			Creators: []common.Creator{
				{CreatorType: "Tool", Creator: "git-vendor-" + toolVersion},
			},
		},
	}

	// Build packages from lock entries
	packages := make([]*spdx23.Package, 0, len(lock.Vendors))
	relationships := make([]*spdx23.Relationship, 0, len(lock.Vendors))

	for _, vendor := range lock.Vendors {
		repoURL := urlMap[vendor.Name]
		pkg := g.buildSPDXPackage(&vendor, repoURL)
		packages = append(packages, pkg)

		// Add DESCRIBES relationship from document to package
		relationships = append(relationships, &spdx23.Relationship{
			RefA:         common.MakeDocElementID("", "DOCUMENT"),
			RefB:         common.MakeDocElementID("", sanitizeSPDXID(vendor.Name)),
			Relationship: "DESCRIBES",
		})
	}

	doc.Packages = packages
	doc.Relationships = relationships

	// Encode to JSON
	return spdxToJSON(doc)
}

// buildSPDXPackage creates an SPDX package from lock details
func (g *SBOMGenerator) buildSPDXPackage(vendor *types.LockDetails, repoURL string) *spdx23.Package {
	// Determine version - prefer source_version_tag, fall back to commit hash
	packageVersion := vendor.CommitHash
	if vendor.SourceVersionTag != "" {
		packageVersion = vendor.SourceVersionTag
	}

	// Build SPDX ID (must be alphanumeric with hyphens)
	spdxID := common.ElementID("Package-" + sanitizeSPDXID(vendor.Name))

	pkg := &spdx23.Package{
		PackageName:           vendor.Name,
		PackageSPDXIdentifier: spdxID,
		PackageVersion:        packageVersion,
		PackageDownloadLocation: func() string {
			if repoURL != "" {
				return repoURL
			}
			return "NOASSERTION"
		}(),
		FilesAnalyzed:        false,
		PackageCopyrightText: "NOASSERTION",
	}

	// Add license
	if vendor.LicenseSPDX != "" {
		pkg.PackageLicenseDeclared = vendor.LicenseSPDX
		pkg.PackageLicenseConcluded = vendor.LicenseSPDX
	} else {
		pkg.PackageLicenseDeclared = "NOASSERTION"
		pkg.PackageLicenseConcluded = "NOASSERTION"
	}

	// Add checksums (aggregate hash of all file hashes)
	if len(vendor.FileHashes) > 0 {
		checksums := make([]common.Checksum, 0, len(vendor.FileHashes))
		for _, hash := range vendor.FileHashes {
			checksums = append(checksums, common.Checksum{
				Algorithm: common.SHA256,
				Value:     hash,
			})
		}
		pkg.PackageChecksums = checksums
	}

	// Add external reference for PURL
	purl := g.getPURL(vendor.Name, repoURL, vendor.CommitHash)
	if purl != "" {
		pkg.PackageExternalReferences = []*spdx23.PackageExternalReference{
			{
				Category: common.CategoryPackageManager,
				RefType:  "purl",
				Locator:  purl,
			},
		}
	}

	// Add annotation with git-vendor metadata
	if vendor.VendoredAt != "" || vendor.VendoredBy != "" {
		comment := fmt.Sprintf("vendored_at=%s, vendored_by=%s, ref=%s, commit=%s",
			vendor.VendoredAt, vendor.VendoredBy, vendor.Ref, vendor.CommitHash)
		pkg.PackageComment = comment
	}

	return pkg
}

// getPURL generates a Package URL for the vendor
func (g *SBOMGenerator) getPURL(vendorName, repoURL, commitHash string) string {
	if repoURL == "" {
		return fmt.Sprintf("pkg:generic/%s@%s", vendorName, commitHash)
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Sprintf("pkg:generic/%s@%s", vendorName, commitHash)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return fmt.Sprintf("pkg:generic/%s@%s", vendorName, commitHash)
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	// For GitLab nested groups, combine all parts except the last as the namespace
	if len(parts) > 2 {
		owner = strings.Join(parts[:len(parts)-1], "/")
		repo = strings.TrimSuffix(parts[len(parts)-1], ".git")
	}

	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "github.com") || strings.Contains(host, "github"):
		return fmt.Sprintf("pkg:github/%s/%s@%s", owner, repo, commitHash)
	case strings.Contains(host, "gitlab.com") || strings.Contains(host, "gitlab"):
		return fmt.Sprintf("pkg:gitlab/%s/%s@%s", url.PathEscape(owner), repo, commitHash)
	case strings.Contains(host, "bitbucket.org") || strings.Contains(host, "bitbucket"):
		return fmt.Sprintf("pkg:bitbucket/%s/%s@%s", owner, repo, commitHash)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", vendorName, commitHash)
	}
}

// sanitizeSPDXID converts a string to a valid SPDX identifier
// SPDX IDs must match [a-zA-Z0-9.-]+
func sanitizeSPDXID(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	return result.String()
}

// spdxToJSON converts an SPDX document to JSON bytes
func spdxToJSON(doc *spdx23.Document) ([]byte, error) {
	// Use the SPDX JSON marshaling
	var buf strings.Builder

	// Manual JSON construction for SPDX 2.3 format
	buf.WriteString("{\n")
	buf.WriteString(fmt.Sprintf("  \"spdxVersion\": \"%s\",\n", doc.SPDXVersion))
	buf.WriteString(fmt.Sprintf("  \"dataLicense\": \"%s\",\n", doc.DataLicense))
	buf.WriteString(fmt.Sprintf("  \"SPDXID\": \"SPDXRef-%s\",\n", doc.SPDXIdentifier))
	buf.WriteString(fmt.Sprintf("  \"name\": \"%s\",\n", doc.DocumentName))
	buf.WriteString(fmt.Sprintf("  \"documentNamespace\": \"%s\",\n", doc.DocumentNamespace))

	// Creation info
	buf.WriteString("  \"creationInfo\": {\n")
	buf.WriteString(fmt.Sprintf("    \"created\": \"%s\",\n", doc.CreationInfo.Created))
	buf.WriteString("    \"creators\": [\n")
	for i, creator := range doc.CreationInfo.Creators {
		comma := ","
		if i == len(doc.CreationInfo.Creators)-1 {
			comma = ""
		}
		buf.WriteString(fmt.Sprintf("      \"%s: %s\"%s\n", creator.CreatorType, creator.Creator, comma))
	}
	buf.WriteString("    ]\n")
	buf.WriteString("  },\n")

	// Packages
	buf.WriteString("  \"packages\": [\n")
	for i, pkg := range doc.Packages {
		buf.WriteString("    {\n")
		buf.WriteString(fmt.Sprintf("      \"SPDXID\": \"SPDXRef-%s\",\n", pkg.PackageSPDXIdentifier))
		buf.WriteString(fmt.Sprintf("      \"name\": \"%s\",\n", pkg.PackageName))
		buf.WriteString(fmt.Sprintf("      \"versionInfo\": \"%s\",\n", pkg.PackageVersion))
		buf.WriteString(fmt.Sprintf("      \"downloadLocation\": \"%s\",\n", pkg.PackageDownloadLocation))
		buf.WriteString(fmt.Sprintf("      \"licenseDeclared\": \"%s\",\n", pkg.PackageLicenseDeclared))
		buf.WriteString(fmt.Sprintf("      \"licenseConcluded\": \"%s\",\n", pkg.PackageLicenseConcluded))
		buf.WriteString(fmt.Sprintf("      \"copyrightText\": \"%s\",\n", pkg.PackageCopyrightText))
		buf.WriteString(fmt.Sprintf("      \"filesAnalyzed\": %t", pkg.FilesAnalyzed))

		// Checksums
		if len(pkg.PackageChecksums) > 0 {
			buf.WriteString(",\n      \"checksums\": [\n")
			for j, checksum := range pkg.PackageChecksums {
				comma := ","
				if j == len(pkg.PackageChecksums)-1 {
					comma = ""
				}
				buf.WriteString(fmt.Sprintf("        {\"algorithm\": \"%s\", \"checksumValue\": \"%s\"}%s\n",
					checksum.Algorithm, checksum.Value, comma))
			}
			buf.WriteString("      ]")
		}

		// External references
		if len(pkg.PackageExternalReferences) > 0 {
			buf.WriteString(",\n      \"externalRefs\": [\n")
			for j, ref := range pkg.PackageExternalReferences {
				comma := ","
				if j == len(pkg.PackageExternalReferences)-1 {
					comma = ""
				}
				buf.WriteString(fmt.Sprintf("        {\"referenceCategory\": \"%s\", \"referenceType\": \"%s\", \"referenceLocator\": \"%s\"}%s\n",
					ref.Category, ref.RefType, ref.Locator, comma))
			}
			buf.WriteString("      ]")
		}

		// Comment (annotation)
		if pkg.PackageComment != "" {
			buf.WriteString(fmt.Sprintf(",\n      \"comment\": \"%s\"", escapeJSON(pkg.PackageComment)))
		}

		buf.WriteString("\n    }")
		if i < len(doc.Packages)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ],\n")

	// Relationships
	buf.WriteString("  \"relationships\": [\n")
	for i, rel := range doc.Relationships {
		comma := ","
		if i == len(doc.Relationships)-1 {
			comma = ""
		}
		buf.WriteString(fmt.Sprintf("    {\"spdxElementId\": \"%s\", \"relationshipType\": \"%s\", \"relatedSpdxElement\": \"%s\"}%s\n",
			rel.RefA.ElementRefID, rel.Relationship, rel.RefB.ElementRefID, comma))
	}
	buf.WriteString("  ]\n")
	buf.WriteString("}\n")

	return []byte(buf.String()), nil
}

// escapeJSON escapes special characters for JSON strings
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
