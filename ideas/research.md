# Research Queue

> Research topics for git-vendor development. Completed research is stored in `ideas/research/`.

## HIGH Priority

| ID | Status | Title | Brief | Output |
|----|--------|-------|-------|--------|
| R001 | pending | CycloneDX vs SPDX Comparison | Compare SBOM formats, library support in Go, tooling ecosystem, adoption rates | - |
| R002 | pending | OSV.dev API Integration | Research OSV API, rate limits, query patterns, caching strategies | - |
| R003 | pending | Diff Algorithm for Drift Detection | Research diff algorithms for efficient file comparison across repos | - |

## MEDIUM Priority

| ID | Status | Title | Brief | Output |
|----|--------|-------|-------|--------|
| R004 | pending | SPDX License List Integration | Research go-license-detector, SPDX license identification, accuracy rates | - |
| R005 | pending | Compliance Framework Requirements | Deep dive into EO 14028, NIST SP 800-161, DORA Article 28 requirements | - |
| R006 | pending | GitHub Action Best Practices | Research action patterns, caching, PR comment formatting, check APIs | - |
| R007 | pending | Mermaid/Graphviz Integration | Research graph generation libraries, D3.js for interactive HTML | - |

## LOW Priority

| ID | Status | Title | Brief | Output |
|----|--------|-------|-------|--------|
| R010 | pending | Sigstore Integration Patterns | Research cosign, Fulcio, Rekor for cryptographic provenance | - |
| R011 | pending | GRC Platform APIs | Research RegScale, Drata, Vanta APIs for compliance evidence ingestion | - |
| R012 | pending | Competitive Analysis | Compare git-vendor to git submodules, git subtree, manual vendoring | - |
| R013 | pending | InnerSource Patterns | Research InnerSource Commons for adoption strategies | - |

---

## Research Topics from ROADMAP

Per ROADMAP.md, these topics inform feature development:

### Supply Chain Security
- SBOM standards (CycloneDX, SPDX) - Section 6.1
- CVE databases (OSV.dev, NVD, GitHub Advisory DB) - Section 6.2
- Provenance tracking patterns - Section 14.2
- SLSA attestation - Section 14.2

### Git Operations
- Shallow clone optimization for large repos
- Partial clone strategies (`--filter=blob:none`)
- Multi-provider support patterns (GitHub, GitLab, Bitbucket)
- Authentication token handling

### Compliance
- EO 14028 - Executive Order on Cybersecurity
- NIST SP 800-161 - Supply Chain Risk Management
- DORA Article 28 - Third-party ICT libraries
- EU CRA - Cyber Resilience Act
- SOC 2 - Change management evidence

### Adoption & Marketing
- Content marketing strategies for dev tools
- GitHub presence optimization
- Show HN submission best practices
- Awesome Go submission requirements

---

## Research Guidelines

- **Output Location**: All completed research goes in `ideas/research/` folder
- **Naming Convention**: `REVIEW_{TOPIC}.md` or `ANALYSIS_{TOPIC}.md`
- **Methodology**: Follow `.claude/commands/RESEARCH.md` for research process
- **Status Values**: `pending`, `in_progress`, `complete`
- **Linking**: Update the Output column with relative path when complete

## Quality Checklist

Before finalizing research:

- [ ] **Accuracy**: All claims verified against sources
- [ ] **Relevance**: Directly applicable to git-vendor development
- [ ] **Actionable**: Clear recommendations and next steps
- [ ] **Connected**: Links to relevant roadmap items
- [ ] **Formatted**: Consistent markdown, readable tables
