package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/EmundoT/git-vendor/internal/purl"
	"github.com/EmundoT/git-vendor/internal/sbom"
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

// SBOMOptions holds configuration for SBOM generation.
// All fields are optional with sensible defaults.
type SBOMOptions struct {
	// ProjectName is the name of the project being documented.
	// If empty, defaults to "unknown-project".
	// Used as: CycloneDX metadata.component.name, SPDX document name prefix.
	ProjectName string

	// SPDXNamespace is the base URL for SPDX document namespaces.
	// If empty, defaults to "https://spdx.org/spdxdocs".
	// The final namespace will be: {SPDXNamespace}/{ProjectName}/{UUID}
	SPDXNamespace string

	// Validate enables schema validation of generated SBOM.
	// When true, the generated SBOM is parsed back to verify it's valid.
	// For CycloneDX: round-trip through cyclonedx-go decoder.
	// For SPDX: JSON structure validation with required field checks.
	Validate bool
}

// SBOMGenerator generates Software Bill of Materials from vendor lockfiles
type SBOMGenerator struct {
	lockStore   LockStore
	configStore ConfigStore
	options     SBOMOptions
}

// NewSBOMGenerator creates a new SBOMGenerator with the given dependencies
func NewSBOMGenerator(lockStore LockStore, configStore ConfigStore, projectName string) *SBOMGenerator {
	return &SBOMGenerator{
		lockStore:   lockStore,
		configStore: configStore,
		options: SBOMOptions{
			ProjectName: sbom.ValidateProjectName(projectName),
		},
	}
}

// NewSBOMGeneratorWithOptions creates a new SBOMGenerator with full options
func NewSBOMGeneratorWithOptions(lockStore LockStore, configStore ConfigStore, opts SBOMOptions) *SBOMGenerator {
	opts.ProjectName = sbom.ValidateProjectName(opts.ProjectName)
	return &SBOMGenerator{
		lockStore:   lockStore,
		configStore: configStore,
		options:     opts,
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

	var output []byte
	switch format {
	case SBOMFormatCycloneDX:
		output, err = g.generateCycloneDX(&lock, urlMap)
	case SBOMFormatSPDX:
		output, err = g.generateSPDX(&lock, urlMap)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}

	if err != nil {
		return nil, fmt.Errorf("Generate: render %s SBOM: %w", format, err)
	}

	// Schema validation (if enabled)
	if g.options.Validate {
		if validationErr := g.validateSBOM(output, format); validationErr != nil {
			return nil, fmt.Errorf("schema validation failed: %w", validationErr)
		}
	}

	return output, nil
}

// generateCycloneDX creates a CycloneDX 1.5 JSON SBOM
func (g *SBOMGenerator) generateCycloneDX(lock *types.VendorLock, urlMap map[string]string) ([]byte, error) {
	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()
	bom.Version = 1

	// Set metadata with tool as component (preferred over deprecated Tools field)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	toolVersion := version.GetVersion()
	bom.Metadata = &cdx.Metadata{
		Timestamp: timestamp,
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{
				{
					Type:    cdx.ComponentTypeApplication,
					Name:    "git-vendor",
					Version: toolVersion,
					ExternalReferences: &[]cdx.ExternalReference{
						{
							Type: cdx.ERTypeWebsite,
							URL:  "https://github.com/EmundoT/git-vendor",
						},
					},
				},
			},
		},
		Component: &cdx.Component{
			Type:    cdx.ComponentTypeApplication,
			Name:    g.options.ProjectName,
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

	// Build unique BOM ref using the shared identifier utility
	identity := sbom.VendorIdentity{
		Name:       vendor.Name,
		Ref:        vendor.Ref,
		CommitHash: vendor.CommitHash,
	}
	bomRef := sbom.GenerateBOMRef(identity)

	// Generate PURL using the shared purl package
	purlObj := purl.FromGitURLWithFallback(repoURL, vendor.CommitHash, vendor.Name)
	purlStr := ""
	if purlObj != nil {
		purlStr = purlObj.String()
	}

	component := cdx.Component{
		Type:       cdx.ComponentTypeLibrary,
		BOMRef:     bomRef,
		Name:       vendor.Name,
		Version:    componentVersion,
		PackageURL: purlStr,
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

	// Build external references
	var extRefs []cdx.ExternalReference

	// Add VCS reference for repository URL
	if repoURL != "" {
		extRefs = append(extRefs, cdx.ExternalReference{
			Type: cdx.ERTypeVCS,
			URL:  repoURL,
		})
	}

	if len(extRefs) > 0 {
		component.ExternalReferences = &extRefs
	}

	// Add supplier information (Issue #12)
	if supplier := sbom.ExtractSupplier(repoURL); supplier != nil {
		component.Supplier = &cdx.OrganizationalEntity{
			Name: supplier.Name,
			URL:  &[]string{supplier.URL},
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

	// Build namespace using shared utility (Issue #11)
	docUUID := uuid.New().String()
	namespace := sbom.BuildSPDXNamespace(g.options.SPDXNamespace, g.options.ProjectName, docUUID)

	// Create SPDX document
	doc := &spdx23.Document{
		SPDXVersion:       spdx.Version,
		DataLicense:       spdx.DataLicense,
		SPDXIdentifier:    common.ElementID(sbom.SPDXDocumentID),
		DocumentName:      g.options.ProjectName + "-vendored-sbom",
		DocumentNamespace: namespace,
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
		// RefB must match the package's SPDXID exactly
		relationships = append(relationships, &spdx23.Relationship{
			RefA:         common.MakeDocElementID("", sbom.SPDXDocumentID),
			RefB:         common.MakeDocElementID("", string(pkg.PackageSPDXIdentifier)),
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

	// Build unique SPDX ID using the shared identifier utility (Issue #5)
	identity := sbom.VendorIdentity{
		Name:       vendor.Name,
		Ref:        vendor.Ref,
		CommitHash: vendor.CommitHash,
	}
	spdxID := common.ElementID(sbom.GenerateSPDXID(identity))

	// Determine download location - use repo URL or NOASSERTION
	downloadLocation := "NOASSERTION"
	if repoURL != "" {
		downloadLocation = repoURL
	}

	pkg := &spdx23.Package{
		PackageName:             vendor.Name,
		PackageSPDXIdentifier:   spdxID,
		PackageVersion:          packageVersion,
		PackageDownloadLocation: downloadLocation,
		FilesAnalyzed:           false,
		PackageCopyrightText:    "NOASSERTION",
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

	// Add supplier information (Issue #12)
	if supplier := sbom.ExtractSupplier(repoURL); supplier != nil {
		pkg.PackageSupplier = &common.Supplier{
			Supplier:     supplier.Name,
			SupplierType: "Organization",
		}
	}

	// Add external reference for PURL
	purlObj := purl.FromGitURLWithFallback(repoURL, vendor.CommitHash, vendor.Name)
	if purlObj != nil {
		pkg.PackageExternalReferences = []*spdx23.PackageExternalReference{
			{
				Category: common.CategoryPackageManager,
				RefType:  "purl",
				Locator:  purlObj.String(),
			},
		}
	}

	// Add annotation with git-vendor metadata (Issue #4 - only non-empty values)
	comment := sbom.MetadataComment(vendor.Ref, vendor.CommitHash, vendor.VendoredAt, vendor.VendoredBy)
	if comment != "" {
		pkg.PackageComment = comment
	}

	return pkg
}

// validateSBOM performs schema validation on the generated SBOM.
// This validates the output is well-formed and contains required fields.
//
// For CycloneDX: round-trip decoding through the cyclonedx-go library.
// For SPDX: JSON structure validation plus required field presence checks.
func (g *SBOMGenerator) validateSBOM(data []byte, format SBOMFormat) error {
	switch format {
	case SBOMFormatCycloneDX:
		// Validate by attempting to decode (round-trip validation)
		decoder := cdx.NewBOMDecoder(bytes.NewReader(data), cdx.BOMFileFormatJSON)
		var testBOM cdx.BOM
		if err := decoder.Decode(&testBOM); err != nil {
			return fmt.Errorf("CycloneDX validation: %w", err)
		}
	case SBOMFormatSPDX:
		// Validate by attempting to decode as our JSON structure
		var testDoc spdxJSON
		if err := json.Unmarshal(data, &testDoc); err != nil {
			return fmt.Errorf("SPDX validation: %w", err)
		}
		// Validate required fields per SPDX 2.3 spec
		if err := validateSPDXRequiredFields(&testDoc); err != nil {
			return fmt.Errorf("validateSBOM: %w", err)
		}
	}
	return nil
}

// validateSPDXRequiredFields checks that all required SPDX 2.3 fields are present.
// See: https://spdx.github.io/spdx-spec/v2.3/document-creation-information/
func validateSPDXRequiredFields(doc *spdxJSON) error {
	// Document-level required fields
	if doc.SPDXVersion == "" {
		return fmt.Errorf("SPDX validation: missing spdxVersion")
	}
	if doc.SPDXID == "" {
		return fmt.Errorf("SPDX validation: missing SPDXID")
	}
	if doc.DataLicense == "" {
		return fmt.Errorf("SPDX validation: missing dataLicense")
	}
	if doc.Name == "" {
		return fmt.Errorf("SPDX validation: missing name")
	}
	if doc.DocumentNamespace == "" {
		return fmt.Errorf("SPDX validation: missing documentNamespace")
	}

	// CreationInfo required fields
	if doc.CreationInfo.Created == "" {
		return fmt.Errorf("SPDX validation: missing creationInfo.created")
	}
	if len(doc.CreationInfo.Creators) == 0 {
		return fmt.Errorf("SPDX validation: missing creationInfo.creators")
	}

	// Package-level validation
	for i, pkg := range doc.Packages {
		if pkg.SPDXID == "" {
			return fmt.Errorf("SPDX validation: package[%d] missing SPDXID", i)
		}
		if pkg.Name == "" {
			return fmt.Errorf("SPDX validation: package[%d] missing name", i)
		}
		if pkg.DownloadLocation == "" {
			return fmt.Errorf("SPDX validation: package[%d] missing downloadLocation", i)
		}
	}

	// Relationship validation: ensure targets exist
	packageIDs := make(map[string]bool)
	packageIDs[doc.SPDXID] = true // Document ID is valid target
	for _, pkg := range doc.Packages {
		packageIDs[pkg.SPDXID] = true
	}
	for i, rel := range doc.Relationships {
		if !packageIDs[rel.SPDXElementID] {
			return fmt.Errorf("SPDX validation: relationship[%d] references unknown element %q", i, rel.SPDXElementID)
		}
		if !packageIDs[rel.RelatedSPDXElement] {
			return fmt.Errorf("SPDX validation: relationship[%d] references unknown element %q", i, rel.RelatedSPDXElement)
		}
	}

	return nil
}

// spdxJSON is the JSON representation of an SPDX document
// Using explicit struct to ensure proper JSON field names per SPDX 2.3 spec
type spdxJSON struct {
	SPDXVersion       string                 `json:"spdxVersion"`
	DataLicense       string                 `json:"dataLicense"`
	SPDXID            string                 `json:"SPDXID"`
	Name              string                 `json:"name"`
	DocumentNamespace string                 `json:"documentNamespace"`
	CreationInfo      spdxCreationInfoJSON   `json:"creationInfo"`
	Packages          []spdxPackageJSON      `json:"packages"`
	Relationships     []spdxRelationshipJSON `json:"relationships"`
}

type spdxCreationInfoJSON struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type spdxPackageJSON struct {
	SPDXID           string                `json:"SPDXID"`
	Name             string                `json:"name"`
	VersionInfo      string                `json:"versionInfo"`
	DownloadLocation string                `json:"downloadLocation"`
	Supplier         string                `json:"supplier,omitempty"`
	LicenseDeclared  string                `json:"licenseDeclared"`
	LicenseConcluded string                `json:"licenseConcluded"`
	CopyrightText    string                `json:"copyrightText"`
	FilesAnalyzed    bool                  `json:"filesAnalyzed"`
	Checksums        []spdxChecksumJSON    `json:"checksums,omitempty"`
	ExternalRefs     []spdxExternalRefJSON `json:"externalRefs,omitempty"`
	Comment          string                `json:"comment,omitempty"`
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
			SPDXID:           sbom.FormatSPDXRef(string(pkg.PackageSPDXIdentifier)),
			Name:             pkg.PackageName,
			VersionInfo:      pkg.PackageVersion,
			DownloadLocation: pkg.PackageDownloadLocation,
			LicenseDeclared:  pkg.PackageLicenseDeclared,
			LicenseConcluded: pkg.PackageLicenseConcluded,
			CopyrightText:    pkg.PackageCopyrightText,
			FilesAnalyzed:    pkg.FilesAnalyzed,
			Comment:          pkg.PackageComment,
		}

		// Add supplier (Issue #12)
		if pkg.PackageSupplier != nil && pkg.PackageSupplier.Supplier != "" {
			p.Supplier = fmt.Sprintf("%s: %s", pkg.PackageSupplier.SupplierType, pkg.PackageSupplier.Supplier)
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
					ReferenceCategory: ref.Category,
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
			SPDXElementID:      sbom.FormatSPDXRef(string(rel.RefA.ElementRefID)),
			RelationshipType:   rel.Relationship,
			RelatedSPDXElement: sbom.FormatSPDXRef(string(rel.RefB.ElementRefID)),
		})
	}

	// Build JSON document
	jsonDoc := spdxJSON{
		SPDXVersion:       doc.SPDXVersion,
		DataLicense:       doc.DataLicense,
		SPDXID:            sbom.FormatSPDXRef(string(doc.SPDXIdentifier)),
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
