# Spec 010: SBOM Generation

> **Status:** Complete
> **Priority:** P0 - Regulatory Requirement
> **Effort:** 5-7 days
> **Dependencies:** 003 (Metadata Enrichment)
> **Blocks:** 020 (Audit), 023 (Compliance)

---

## Problem Statement

SBOM (Software Bill of Materials) requirements are proliferating:
- EO 14028 (US federal contractors)
- DORA (EU financial services)
- EU Cyber Resilience Act
- NIST SP 800-161

Teams need SBOMs for vendored source code, but existing tools (Syft, Trivy, CycloneDX-gomod) only scan package manifests—they miss vendored code that doesn't appear in any manifest.

## Solution Overview

New command `git vendor sbom` that generates SBOMs in CycloneDX and SPDX formats from the lockfile and vendored file metadata.

```bash
git vendor sbom [--format cyclonedx|spdx] [--output file]
```

## Detailed Design

### 1. Command Interface

```bash
# Default: CycloneDX JSON to stdout
git vendor sbom

# SPDX format
git vendor sbom --format spdx

# Write to file
git vendor sbom --output sbom.json

# Combined
git vendor sbom --format spdx --output sbom.spdx.json
```

### 2. Lockfile to SBOM Mapping

| Lockfile Field | CycloneDX Field | SPDX Field |
|---------------|-----------------|------------|
| name | component.name | package.name |
| URL | component.purl | package.downloadLocation |
| commit_hash | component.version | package.versionInfo |
| source_version_tag | component.version (if available) | package.versionInfo |
| license_spdx | component.licenses[].id | package.licenseDeclared |
| vendored_at | component.properties["vendored_at"] | package.annotation |
| vendored_by | component.properties["vendored_by"] | package.annotation |
| file hashes | component.hashes[] | package.checksums[] |

### 3. CycloneDX Output Structure

```json
{
  "$schema": "http://cyclonedx.org/schema/bom-1.5.schema.json",
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "serialNumber": "urn:uuid:...",
  "version": 1,
  "metadata": {
    "timestamp": "2026-02-04T12:00:00Z",
    "tools": [
      {
        "vendor": "git-vendor",
        "name": "git-vendor",
        "version": "1.1.0"
      }
    ],
    "component": {
      "type": "application",
      "name": "my-project",
      "version": "local"
    }
  },
  "components": [
    {
      "type": "library",
      "bom-ref": "some-lib@abc1234",
      "name": "some-lib",
      "version": "v1.2.3",
      "purl": "pkg:github/owner/some-lib@abc1234",
      "hashes": [
        {
          "alg": "SHA-256",
          "content": "abc123..."
        }
      ],
      "licenses": [
        {
          "license": {
            "id": "MIT"
          }
        }
      ],
      "externalReferences": [
        {
          "type": "vcs",
          "url": "https://github.com/owner/some-lib"
        }
      ],
      "properties": [
        {
          "name": "git-vendor:vendored_at",
          "value": "2026-01-15T10:00:00Z"
        },
        {
          "name": "git-vendor:vendored_by",
          "value": "User <user@example.com>"
        },
        {
          "name": "git-vendor:commit",
          "value": "abc1234567890"
        },
        {
          "name": "git-vendor:ref",
          "value": "main"
        }
      ]
    }
  ]
}
```

### 4. SPDX Output Structure

```json
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "my-project-vendored-sbom",
  "documentNamespace": "https://git-vendor.dev/spdx/my-project/...",
  "creationInfo": {
    "created": "2026-02-04T12:00:00Z",
    "creators": [
      "Tool: git-vendor-1.1.0"
    ]
  },
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-some-lib",
      "name": "some-lib",
      "versionInfo": "v1.2.3",
      "downloadLocation": "https://github.com/owner/some-lib",
      "licenseDeclared": "MIT",
      "licenseConcluded": "MIT",
      "copyrightText": "NOASSERTION",
      "checksums": [
        {
          "algorithm": "SHA256",
          "checksumValue": "abc123..."
        }
      ],
      "externalRefs": [
        {
          "referenceCategory": "PACKAGE-MANAGER",
          "referenceType": "purl",
          "referenceLocator": "pkg:github/owner/some-lib@abc1234"
        }
      ],
      "annotations": [
        {
          "annotationType": "OTHER",
          "annotator": "Tool: git-vendor",
          "annotationDate": "2026-01-15T10:00:00Z",
          "comment": "vendored_at=2026-01-15T10:00:00Z, vendored_by=User <user@example.com>"
        }
      ]
    }
  ],
  "relationships": [
    {
      "spdxElementId": "SPDXRef-DOCUMENT",
      "relationshipType": "DESCRIBES",
      "relatedSpdxElement": "SPDXRef-Package-some-lib"
    }
  ]
}
```

### 5. Implementation

#### Dependencies

```go
// go.mod
require (
    github.com/CycloneDX/cyclonedx-go v0.8.0
    github.com/spdx/tools-golang v0.5.3
)
```

#### Core Generator

```go
type SBOMGenerator struct {
    lockStore LockStore
    fs        FileSystem
}

func (g *SBOMGenerator) Generate(format string) ([]byte, error) {
    lock, err := g.lockStore.Load("vendor/vendor.lock")
    if err != nil {
        return nil, fmt.Errorf("load lockfile: %w", err)
    }

    switch format {
    case "cyclonedx":
        return g.generateCycloneDX(lock)
    case "spdx":
        return g.generateSPDX(lock)
    default:
        return nil, fmt.Errorf("unknown format: %s", format)
    }
}
```

#### CycloneDX Generation

```go
func (g *SBOMGenerator) generateCycloneDX(lock *VendorLock) ([]byte, error) {
    bom := cdx.NewBOM()
    bom.SerialNumber = "urn:uuid:" + uuid.New().String()

    bom.Metadata = &cdx.Metadata{
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Tools: &[]cdx.Tool{
            {
                Vendor:  "git-vendor",
                Name:    "git-vendor",
                Version: version.GetVersion(),
            },
        },
    }

    for _, vendor := range lock.Vendors {
        component := cdx.Component{
            Type:    cdx.ComponentTypeLibrary,
            BOMRef:  fmt.Sprintf("%s@%s", vendor.Name, vendor.CommitHash[:7]),
            Name:    vendor.Name,
            Version: g.getVersion(vendor),
            PURL:    g.getPURL(vendor),
        }

        if vendor.LicenseSPDX != "" {
            component.Licenses = &cdx.Licenses{
                {License: &cdx.License{ID: vendor.LicenseSPDX}},
            }
        }

        // Add hashes for each vendored file
        hashes := []cdx.Hash{}
        for _, hash := range vendor.FileHashes {
            hashes = append(hashes, cdx.Hash{
                Algorithm: cdx.HashAlgoSHA256,
                Value:     hash,
            })
        }
        component.Hashes = &hashes

        // Add provenance as properties
        component.Properties = &[]cdx.Property{
            {Name: "git-vendor:vendored_at", Value: vendor.VendoredAt},
            {Name: "git-vendor:vendored_by", Value: vendor.VendoredBy},
            {Name: "git-vendor:commit", Value: vendor.CommitHash},
            {Name: "git-vendor:ref", Value: vendor.Ref},
        }

        bom.Components = append(*bom.Components, component)
    }

    return cdx.Encode(bom, cdx.BOMFileFormatJSON)
}
```

#### PURL Generation

```go
func (g *SBOMGenerator) getPURL(vendor LockDetails) string {
    // Parse URL to determine type
    u, err := url.Parse(vendor.URL)
    if err != nil {
        return ""
    }

    parts := strings.Split(strings.Trim(u.Path, "/"), "/")
    if len(parts) < 2 {
        return ""
    }

    owner := parts[0]
    repo := strings.TrimSuffix(parts[1], ".git")

    switch u.Host {
    case "github.com":
        return fmt.Sprintf("pkg:github/%s/%s@%s", owner, repo, vendor.CommitHash)
    case "gitlab.com":
        return fmt.Sprintf("pkg:gitlab/%s/%s@%s", owner, repo, vendor.CommitHash)
    case "bitbucket.org":
        return fmt.Sprintf("pkg:bitbucket/%s/%s@%s", owner, repo, vendor.CommitHash)
    default:
        return fmt.Sprintf("pkg:generic/%s@%s", vendor.Name, vendor.CommitHash)
    }
}
```

### 6. Validation

SBOMs must pass validation:

```bash
# CycloneDX validation
cyclonedx validate --input-file sbom.json

# SPDX validation
pyspdxtools -i sbom.spdx.json
```

Integrate validation in tests to ensure output is always spec-compliant.

## Test Plan

### Unit Tests

```go
func TestGenerateCycloneDX_SingleVendor(t *testing.T) {
    // Single vendor with all metadata
    // Verify all fields populated correctly
}

func TestGenerateCycloneDX_MultipleVendors(t *testing.T) {
    // Multiple vendors
    // Verify each appears as component
}

func TestGenerateCycloneDX_MissingLicense(t *testing.T) {
    // Vendor without license_spdx
    // Verify license field omitted, not empty
}

func TestGenerateSPDX_ValidOutput(t *testing.T) {
    // Generate SPDX output
    // Verify passes SPDX validation
}

func TestGetPURL_GitHub(t *testing.T) {
    // GitHub URL → pkg:github/owner/repo@commit
}

func TestGetPURL_GitLab(t *testing.T) {
    // GitLab URL → pkg:gitlab/owner/repo@commit
}
```

### Integration Tests

1. Generate CycloneDX SBOM, validate against schema
2. Generate SPDX SBOM, validate against schema
3. Import CycloneDX SBOM into Dependency-Track
4. Import SPDX SBOM into Grype
5. Verify PURL format allows vulnerability matching

### Golden File Tests

Store known-good SBOM outputs in `testdata/sbom/`:
- `testdata/sbom/expected_cyclonedx.json`
- `testdata/sbom/expected_spdx.json`

Compare generated output against golden files (normalize timestamps/UUIDs).

## Acceptance Criteria

- [ ] CycloneDX JSON output validates against CycloneDX 1.5 schema
- [ ] SPDX JSON output validates against SPDX 2.3 schema
- [ ] Every vendored dependency appears as a component with name, version, license, hashes
- [ ] SBOM can be ingested by Dependency-Track without errors
- [ ] SBOM can be scanned by Grype without errors
- [ ] `--output` flag writes to file; without it, writes to stdout (pipeable)
- [ ] PURLs follow Package URL specification

## Rollout Plan

1. Add CycloneDX and SPDX library dependencies
2. Implement CycloneDX generator
3. Implement SPDX generator
4. Add `git vendor sbom` command
5. Add validation tests against official schemas
6. Test integration with Dependency-Track and Grype
7. Document usage in README and docs/COMMANDS.md
8. Release in v1.2.0

## Open Questions

1. **Aggregate hash vs per-file hashes?** Current spec includes per-file hashes. Some tools expect a single aggregate hash. May need to support both.

2. **PURL for private repos?** Current spec generates PURLs for all repos. Private repo PURLs may not resolve. Document this limitation.

## References

- ROADMAP.md Feature 2.1
- CycloneDX Spec: https://cyclonedx.org/docs/1.5/json/
- SPDX Spec: https://spdx.github.io/spdx-spec/v2.3/
- Package URL Spec: https://github.com/package-url/purl-spec
- CycloneDX Go Library: https://github.com/CycloneDX/cyclonedx-go
- SPDX Go Library: https://github.com/spdx/tools-golang
