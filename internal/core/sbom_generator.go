package core

import (
	"encoding/json"
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
	var buf strings.Builder
	encoder := cdx.NewBOMEncoder(&buf, cdx.BOMFileFormatJSON)
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
		// IMPORTANT: RefB must match the package's SPDXID exactly (including "Package-" prefix)
		packageSPDXID := "Package-" + sanitizeSPDXID(vendor.Name)
		relationships = append(relationships, &spdx23.Relationship{
			RefA:         common.MakeDocElementID("", "DOCUMENT"),
			RefB:         common.MakeDocElementID("", packageSPDXID),
			Relationship: "DESCRIBES",
		})
	}

	doc.Packages = packages
	doc.Relationships = relationships

	// Use proper JSON encoding via the SPDX library's struct tags
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

// spdxJSON is the JSON representation of an SPDX document
// Using explicit struct to ensure proper JSON field names per SPDX 2.3 spec
type spdxJSON struct {
	SPDXVersion       string                `json:"spdxVersion"`
	DataLicense       string                `json:"dataLicense"`
	SPDXID            string                `json:"SPDXID"`
	Name              string                `json:"name"`
	DocumentNamespace string                `json:"documentNamespace"`
	CreationInfo      spdxCreationInfoJSON  `json:"creationInfo"`
	Packages          []spdxPackageJSON     `json:"packages"`
	Relationships     []spdxRelationshipJSON `json:"relationships"`
}

type spdxCreationInfoJSON struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type spdxPackageJSON struct {
	SPDXID           string              `json:"SPDXID"`
	Name             string              `json:"name"`
	VersionInfo      string              `json:"versionInfo"`
	DownloadLocation string              `json:"downloadLocation"`
	LicenseDeclared  string              `json:"licenseDeclared"`
	LicenseConcluded string              `json:"licenseConcluded"`
	CopyrightText    string              `json:"copyrightText"`
	FilesAnalyzed    bool                `json:"filesAnalyzed"`
	Checksums        []spdxChecksumJSON  `json:"checksums,omitempty"`
	ExternalRefs     []spdxExternalRefJSON `json:"externalRefs,omitempty"`
	Comment          string              `json:"comment,omitempty"`
}

type spdxChecksumJSON struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

type spdxExternalRefJSON struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

type spdxRelationshipJSON struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// spdxToJSON converts an SPDX document to JSON bytes using proper struct marshaling
func spdxToJSON(doc *spdx23.Document) ([]byte, error) {
	// Build creators list
	creators := make([]string, 0, len(doc.CreationInfo.Creators))
	for _, c := range doc.CreationInfo.Creators {
		creators = append(creators, fmt.Sprintf("%s: %s", c.CreatorType, c.Creator))
	}

	// Build packages
	packages := make([]spdxPackageJSON, 0, len(doc.Packages))
	for _, pkg := range doc.Packages {
		p := spdxPackageJSON{
			SPDXID:           fmt.Sprintf("SPDXRef-%s", pkg.PackageSPDXIdentifier),
			Name:             pkg.PackageName,
			VersionInfo:      pkg.PackageVersion,
			DownloadLocation: pkg.PackageDownloadLocation,
			LicenseDeclared:  pkg.PackageLicenseDeclared,
			LicenseConcluded: pkg.PackageLicenseConcluded,
			CopyrightText:    pkg.PackageCopyrightText,
			FilesAnalyzed:    pkg.FilesAnalyzed,
			Comment:          pkg.PackageComment,
		}

		// Add checksums
		if len(pkg.PackageChecksums) > 0 {
			checksums := make([]spdxChecksumJSON, 0, len(pkg.PackageChecksums))
			for _, cs := range pkg.PackageChecksums {
				checksums = append(checksums, spdxChecksumJSON{
					Algorithm:     string(cs.Algorithm),
					ChecksumValue: cs.Value,
				})
			}
			p.Checksums = checksums
		}

		// Add external refs
		if len(pkg.PackageExternalReferences) > 0 {
			refs := make([]spdxExternalRefJSON, 0, len(pkg.PackageExternalReferences))
			for _, ref := range pkg.PackageExternalReferences {
				refs = append(refs, spdxExternalRefJSON{
					ReferenceCategory: string(ref.Category),
					ReferenceType:     ref.RefType,
					ReferenceLocator:  ref.Locator,
				})
			}
			p.ExternalRefs = refs
		}

		packages = append(packages, p)
	}

	// Build relationships
	relationships := make([]spdxRelationshipJSON, 0, len(doc.Relationships))
	for _, rel := range doc.Relationships {
		relationships = append(relationships, spdxRelationshipJSON{
			SPDXElementID:      fmt.Sprintf("SPDXRef-%s", rel.RefA.ElementRefID),
			RelationshipType:   rel.Relationship,
			RelatedSPDXElement: fmt.Sprintf("SPDXRef-%s", rel.RefB.ElementRefID),
		})
	}

	// Build JSON document
	jsonDoc := spdxJSON{
		SPDXVersion:       doc.SPDXVersion,
		DataLicense:       doc.DataLicense,
		SPDXID:            fmt.Sprintf("SPDXRef-%s", doc.SPDXIdentifier),
		Name:              doc.DocumentName,
		DocumentNamespace: doc.DocumentNamespace,
		CreationInfo: spdxCreationInfoJSON{
			Created:  doc.CreationInfo.Created,
			Creators: creators,
		},
		Packages:      packages,
		Relationships: relationships,
	}

	// Marshal with indentation for readability
	return json.MarshalIndent(jsonDoc, "", "  ")
}
