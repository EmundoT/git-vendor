# Claude Commands Index

This document describes all available slash commands and their relationships for git-vendor development.

## Command Overview

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `/PROJECT_PRIMER` | Understand the codebase | Starting a new session, onboarding |
| `/IDEA_WORKFLOW` | Implement ideas from queues | Ready to build features |
| `/IDEA_CURATION` | Maintain queue health | Weekly cleanup, priority rebalancing |
| `/CODE_REVIEW` | Find and fix code issues | Quality audits, pre-release checks |
| `/PM` | Coordinate multi-agent work | Managing parallel execution |
| `/AUDIT_COMPLETED` | Verify completed work | Post-implementation validation |
| `/RESEARCH` | Deep-dive investigation | Complex technical questions |
| `/SECURITY_AUDIT` | Security vulnerability scan | Security reviews, compliance |
| `/TEST_COVERAGE` | Identify and fill test gaps | Coverage improvement cycles |
| `/DOC_SYNC` | Sync documentation with code | Documentation maintenance |
| `/MIGRATION_PLANNER` | Plan lockfile schema migrations | Schema changes, version bumps |
| `/DEPENDENCY_AUDIT` | Analyze Go package dependencies | Refactoring, architecture review |
| `/PERFORMANCE_BASELINE` | Monitor performance metrics | Performance optimization |
| `/RELEASE_READY` | Pre-release validation | Release preparation |

---

## Command Categories

### Core Workflows

| Command | Focus |
|---------|-------|
| `/IDEA_WORKFLOW` | Feature implementation |
| `/IDEA_CURATION` | Backlog management |
| `/PM` | Work coordination |

### Quality Assurance

| Command | Focus |
|---------|-------|
| `/CODE_REVIEW` | Go code quality issues |
| `/SECURITY_AUDIT` | Security vulnerabilities |
| `/TEST_COVERAGE` | Test completeness with gomock |
| `/AUDIT_COMPLETED` | Completion verification |

### Documentation & Maintenance

| Command | Focus |
|---------|-------|
| `/DOC_SYNC` | Documentation accuracy |
| `/DEPENDENCY_AUDIT` | Go module dependencies |
| `/MIGRATION_PLANNER` | Lockfile schema management |

### Performance & Release

| Command | Focus |
|---------|-------|
| `/PERFORMANCE_BASELINE` | Go benchmarks |
| `/RELEASE_READY` | Release validation (orchestrates all gates) |

### Research & Onboarding

| Command | Focus |
|---------|-------|
| `/PROJECT_PRIMER` | Codebase understanding |
| `/RESEARCH` | Technical investigation |

---

## Command Relationships

```
                         ┌──────────────────┐
                         │  RELEASE_READY   │
                         │  (Orchestrator)  │
                         └────────┬─────────┘
                                  │ Runs all gates
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
┌───────────────┐        ┌───────────────┐        ┌───────────────┐
│ SECURITY_AUDIT│        │ TEST_COVERAGE │        │  CODE_REVIEW  │
│  (Gate 1)     │        │   (Gate 2)    │        │   (Gate 4)    │
└───────────────┘        └───────────────┘        └───────────────┘
        │                         │                         │
        │         ┌───────────────┼───────────────┐         │
        │         │               │               │         │
        ▼         ▼               ▼               ▼         ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│   DOC_SYNC    │  │MIGRATION_PLAN │  │PERF_BASELINE  │  │DEPENDENCY_AUD │
│   (Gate 3)    │  │   (Gate 7)    │  │   (Gate 5)    │  │   (Gate 6)    │
└───────────────┘  └───────────────┘  └───────────────┘  └───────────────┘


┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ IDEA_CURATION   │◄───│      PM         │───►│ IDEA_WORKFLOW   │
│ (Queue Health)  │    │ (Coordination)  │    │ (Implementation)│
└─────────────────┘    └────────┬────────┘    └─────────────────┘
                                │
                                ▼
                    ┌─────────────────┐
                    │ AUDIT_COMPLETED │
                    │ (Verification)  │
                    └─────────────────┘
```

---

## Workflow Patterns

### Pattern 1: Quality Improvement Cycle

```
CODE_REVIEW → Generate Prompts → Execute → CODE_REVIEW (verify) → Follow-up → Repeat
```

### Pattern 2: Feature Development Cycle

```
IDEA_CURATION → PM → IDEA_WORKFLOW → PM (verify) → AUDIT_COMPLETED
```

### Pattern 3: Parallel Execution

```
PM → Dispatch N prompts → N instances work → PM (verify all) → Merge
```

### Pattern 4: Release Preparation

```
RELEASE_READY → Run all gates → Fix blockers → Re-verify → Release
```

### Pattern 5: Security Hardening

```
SECURITY_AUDIT → Triage by severity → Fix CRITICAL first → Verify → Iterate
```

### Pattern 6: Test Improvement

```
TEST_COVERAGE → Identify gaps → Generate test prompts → Execute → Verify quality
```

---

## Queue Files Reference

| Queue | File | ID Prefix | Spec Location |
|-------|------|-----------|---------------|
| Features | `ideas/queue.md` | 3-digit (001-054) | `ideas/specs/in-progress/`, `ideas/specs/complete/` |
| Code Quality | `ideas/code_quality.md` | CQ-NNN | `ideas/specs/code-quality/` |
| Security | `ideas/security.md` | SEC-NNN | `ideas/specs/security/` |
| Research | `ideas/research.md` | R-NNN | `ideas/research/` |
| Completed | `ideas/completed.md` | (moved here) | (specs moved to complete/) |

---

## Command Details

### /PROJECT_PRIMER

**Purpose:** Understand git-vendor codebase structure and conventions

**Use when:** Starting a fresh session, need context on project architecture, onboarding

---

### /IDEA_WORKFLOW

**Purpose:** Claim and implement ideas from queue files

**Phases:** Sync & Claim → Execution → Expansion → Integration

---

### /IDEA_CURATION

**Purpose:** Maintain healthy idea queues

**Phases:** Audit → Cleanup → Maintenance → Flesh Out → Integration

---

### /CODE_REVIEW

**Purpose:** Systematic Go code quality assessment with verification loop

**Phases:** Discovery → Categorization → Prompt Generation → Output → Review Cycle → Completion

**Key Feature:** Iterative verification - doesn't trust completion claims, always re-checks

**Go Focus:** Interface design, error handling, package structure, naming conventions

---

### /PM

**Purpose:** Project manager coordination for multi-agent work

**Phases:** Queue Assessment → Work Planning → Prompt Generation → Dispatch → Review Cycle → Completion

**Key Feature:** Emphasis on tracking updates - prompts explicitly require queue/doc updates

---

### /AUDIT_COMPLETED

**Purpose:** Verify completed work meets quality standards

**Checks:** Go code quality, documentation, test coverage, implementation completeness

---

### /RESEARCH

**Purpose:** Deep investigation of technical questions

**Focus Areas:** Go patterns, supply chain security, SBOM standards, Git operations

---

### /SECURITY_AUDIT

**Purpose:** Security vulnerability scanning with remediation tracking

**Phases:** Sync & Scan → Severity Classification → Remediation Prompts → Report → Verification Cycle → Sign-Off

**Key Feature:** SLA-based severity (CRITICAL=24hr, HIGH=72hr, etc.)

**Scans for:** Path traversal, command injection, insecure Git operations, credential exposure

---

### /TEST_COVERAGE

**Purpose:** Identify and fill test coverage gaps

**Phases:** Coverage Analysis → Gap Prioritization → Test Prompts → Report → Verification Cycle → Completion

**Key Feature:** Uses gomock for interface testing; verifies tests actually test code

**Checks:** Functions without tests, partial coverage, mock completeness

---

### /DOC_SYNC

**Purpose:** Keep documentation synchronized with code

**Phases:** Inventory → Drift Detection → Categorization → Sync Prompts → Report → Verification → Sign-Off

**Key Feature:** Compares multiple doc sources (CLAUDE.md, README, command help, godoc)

**Detects:** Flag drift, missing docs, inaccurate examples, stale references

---

### /MIGRATION_PLANNER

**Purpose:** Plan and verify lockfile schema migrations

**Phases:** Schema Analysis → Drift Classification → Migration Planning → Prompt Generation → Report → Verification → Approval

**Key Feature:** Always ensures backward compatibility, versioned schema changes

**Handles:** New fields, deprecated fields, schema version bumps

---

### /DEPENDENCY_AUDIT

**Purpose:** Analyze Go module dependencies and architecture

**Phases:** Dependency Discovery → Analysis → Issue Categorization → Refactoring Prompts → Report → Verification → Health Report

**Key Feature:** Builds import graph, detects problematic patterns

**Detects:** Circular imports, unused packages, deep dependency chains, heavy dependencies

---

### /PERFORMANCE_BASELINE

**Purpose:** Establish and monitor Go benchmark baselines

**Phases:** Baseline Assessment → Statistical Analysis → Issue Categorization → Optimization Prompts → Report → Verification → Sign-Off

**Key Feature:** Uses Go's benchstat for statistical significance

**Monitors:** Benchmark results, memory allocations, goroutine counts

---

### /RELEASE_READY

**Purpose:** Pre-release validation orchestrating all quality gates

**Gates:**
1. Security (BLOCKING)
2. Tests (BLOCKING)
3. Documentation (BLOCKING)
4. Code Quality (BLOCKING)
5. Performance (non-blocking)
6. Dependencies (non-blocking)
7. Migrations (conditional)

**Key Feature:** Composite command - runs all other audit commands as gates

---

## Verification Standards

All commands with verification phases use consistent grading:

| Grade | Meaning | Action |
|-------|---------|--------|
| **PASS** | All criteria met, verified | Complete |
| **INCOMPLETE** | Code done, tracking skipped | Tracking follow-up |
| **PARTIAL** | Some work done | Gap follow-up |
| **FAIL** | Nothing done or wrong | Redo prompt |
| **REGRESSION** | Fix introduced new issues | Urgent fix |

---

## Quick Selection Guide

**"I need to..."**

| Goal | Command |
|------|---------|
| Understand this codebase | `/PROJECT_PRIMER` |
| Find Go code quality issues | `/CODE_REVIEW` |
| Find security vulnerabilities | `/SECURITY_AUDIT` |
| Improve test coverage | `/TEST_COVERAGE` |
| Fix documentation drift | `/DOC_SYNC` |
| Plan a lockfile schema migration | `/MIGRATION_PLANNER` |
| Analyze Go module dependencies | `/DEPENDENCY_AUDIT` |
| Check for performance regressions | `/PERFORMANCE_BASELINE` |
| Validate release readiness | `/RELEASE_READY` |
| Implement a feature | `/IDEA_WORKFLOW` |
| Clean up the idea backlog | `/IDEA_CURATION` |
| Coordinate parallel work | `/PM` |
| Verify completed work | `/AUDIT_COMPLETED` |
| Research a technical question | `/RESEARCH` |

---

## Common Command Sequences

### Starting a New Sprint

```
/IDEA_CURATION → /PM → /IDEA_WORKFLOW
```

### Pre-Release

```
/RELEASE_READY → (fix blockers) → /RELEASE_READY (verify)
```

### After Major Refactoring

```
/DEPENDENCY_AUDIT → /TEST_COVERAGE → /CODE_REVIEW
```

### Security Review

```
/SECURITY_AUDIT → (fix by severity) → /SECURITY_AUDIT (verify)
```

### Performance Investigation

```
/PERFORMANCE_BASELINE → /CODE_REVIEW → /DEPENDENCY_AUDIT
```

### Roadmap Feature Implementation

```
/PROJECT_PRIMER → /RESEARCH → /IDEA_WORKFLOW → /TEST_COVERAGE → /DOC_SYNC
```
