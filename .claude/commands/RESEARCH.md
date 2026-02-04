# Research Methodology for git-vendor

This guide describes how to conduct effective research for the git-vendor project, producing actionable documentation that benefits future development.

---

## Research Process Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  1. EXPLORE    →  2. ANALYZE    →  3. SYNTHESIZE  →  4. PUBLISH │
│  (Codebase)       (Patterns)        (Insights)        (Document)│
└─────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Exploration

### Use the Explore Agent
For comprehensive codebase understanding, use the Task tool with `subagent_type=Explore`:

```
Task: Explore the codebase to understand [topic area]
Focus on:
- Directory structure and file organization
- Key functions and their purposes
- Data flow and dependencies
- Existing integration points
```

### Key Areas to Investigate

| Area | What to Look For |
|------|------------------|
| **internal/core/** | Business logic, interfaces, orchestration patterns |
| **internal/types/** | Data models, config structures, type definitions |
| **internal/tui/** | User interaction patterns, wizard flows |
| **main.go** | Command routing, CLI structure |
| **vendor/** | Config/lock file formats, real-world usage |
| **.claude/commands/** | Existing tooling and automation |
| **docs/ROADMAP.md** | Planned features, architectural guidelines |

### Read Key Files Directly
For specific components, read the source files:
- Interface definitions in `internal/core/*.go`
- Type definitions in `internal/types/types.go`
- Command implementations in `main.go`

---

## Phase 2: Analysis

### Identify Patterns
Look for recurring patterns that inform the research topic:
- How existing features handle similar problems
- API consistency (parameter naming, return formats)
- Error handling approaches
- Interface design patterns (dependency injection)

### Map Integration Points
Document where external systems could connect:
- CLI flags and commands
- Configuration files (vendor.yml, vendor.lock)
- Git provider APIs (GitHub, GitLab, Bitbucket)
- File system operations
- Environment variables

### Assess Gaps and Opportunities
Compare current capabilities against the research topic:
- What exists that supports the topic?
- What's missing or incomplete?
- What could be enhanced vs. built new?
- How does this align with the ROADMAP.md?

---

## Phase 3: Synthesis

### Structure Your Findings

Use this outline for research documents:

```markdown
# [Topic] Review: Structure, Features, and [Focus Area]

## Executive Summary
[2-3 sentences on key findings and recommendations]

## 1. Current State Analysis
[What exists in git-vendor related to this topic]

## 2. [Topic]-Specific Capabilities
[Deep dive on relevant features, with code examples]

## 3. Integration Opportunities
[Specific ways to leverage/extend git-vendor]

## 4. Recommended Architecture
[Diagrams and patterns for implementation]

## 5. Potential Enhancements
[Future work suggestions]

## 6. Conclusion
[Summary and next steps]
```

### Include Concrete Examples
Every research document should include:
- **Working Go code examples** that readers can understand
- **Architecture diagrams** using ASCII art or markdown tables
- **Comparison tables** for feature analysis
- **Code patterns** showing recommended approaches
- **CLI examples** demonstrating usage

### Connect to Existing Features
Always show how findings relate to existing git-vendor capabilities:
- Reference specific functions by name and file
- Include example parameter usage
- Show how features compose together
- Link to relevant roadmap items

---

## Phase 4: Publication

### File Naming Convention
```
ideas/research/REVIEW_{TOPIC}.md    - Comprehensive analysis documents
ideas/research/ANALYSIS_{TOPIC}.md  - Focused technical analysis
ideas/research/GUIDE_{TOPIC}.md     - How-to guides based on research
```

### Update Research Queue
After completing research:

1. Update `ideas/research.md`:
   - Change status from `pending` to `complete`
   - Add relative path in Output column: `[research](research/FILENAME.md)`

2. Commit with descriptive message:
   ```
   docs: Add [topic] research analysis

   - [Key finding 1]
   - [Key finding 2]
   - [Recommendations]
   ```

---

## Quality Checklist

Before finalizing research:

- [ ] **Accuracy**: All code examples verified against source
- [ ] **Completeness**: All relevant git-vendor features covered
- [ ] **Actionable**: Clear recommendations and next steps
- [ ] **Connected**: Links to existing features and roadmap items
- [ ] **Formatted**: Consistent markdown, readable tables, clear headings

---

## Research Topics vs. Implementation Ideas

| Aspect | Research (ideas/research/) | Ideas (ideas/queue.md) |
|--------|---------------------|------------------------|
| **Purpose** | Understand, analyze, recommend | Build, implement, ship |
| **Output** | Documentation | Code changes |
| **Scope** | Broad exploration | Specific features |
| **Timing** | Before implementation | During implementation |

**Relationship**: Research often identifies implementation opportunities that become ideas in `queue.md`. Reference research documents in idea specs.

---

## git-vendor Specific Research Areas

### Supply Chain Security
- SBOM standards (CycloneDX, SPDX)
- CVE databases (OSV.dev, NVD)
- Provenance tracking patterns
- Sigstore/cosign integration

### Git Operations
- Shallow clone optimization
- Partial clone strategies
- Multi-provider support (GitHub, GitLab, Bitbucket)
- Authentication patterns

### Go Ecosystem
- Interface design patterns
- Error handling best practices
- Testing with gomock
- CLI frameworks (cobra alternatives)

### Lockfile Evolution
- Schema versioning strategies
- Backward compatibility patterns
- Migration tooling
- Validation approaches

---

## Example: SBOM Integration Research

A research document for SBOM integration would follow this process:

1. **Explore**: Use Explore agent to map existing lockfile structure, understand vendor.yml schema, identify output patterns
2. **Analyze**: Compare CycloneDX vs SPDX formats, identify mapping from lockfile fields to SBOM fields, review existing Go libraries
3. **Synthesize**: Create format mapping tables, architecture diagrams for SBOM generation, recommend specific Go libraries
4. **Publish**: Document in `ideas/research/REVIEW_SBOM_INTEGRATION.md`, update `ideas/research.md`

Key learnings to apply:
- Start with broad exploration before deep dives
- Let the codebase guide the analysis
- Include both current capabilities AND enhancement suggestions
- Use tables and diagrams for complex comparisons
- Always connect back to the roadmap

---

## Research Tooling

### Codebase Exploration
```bash
# Find all interface definitions
grep -rn "type.*interface" internal/

# Find all exported functions in a package
grep -rn "^func [A-Z]" internal/core/

# Find all error handling patterns
grep -rn "return.*err" internal/core/

# Find all TODO/FIXME comments
grep -rn "TODO\|FIXME" .
```

### Dependency Analysis
```bash
# List all Go dependencies
go list -m all

# Show dependency graph
go mod graph

# Check for updates
go list -u -m all
```

### Architecture Mapping
```bash
# Count lines of code by package
find internal -name "*.go" -not -name "*_test.go" | xargs wc -l

# Find circular imports
go vet ./...

# Generate import graph
go mod graph | grep "git-vendor"
```
