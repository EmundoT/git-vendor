# Ecosystem Protocol Rules

These rules apply to all git-ecosystem projects (git-plumbing, git-agent, git-vendor). They govern documentation, code quality, and communication standards across the ecosystem.

For commit conventions and tag system rules, see `commit-schema-tags.md`.

---

## 1. Definition of Done

Work is INCOMPLETE until ALL of the following pass with exit code zero:

1. **Build succeeds** — `go build ./...` (or language equivalent)
2. **All tests pass** — `go test ./...`
3. **Linter is green** — `go vet ./...` (aligned with local git hooks if configured)
4. **Inline documentation updated** for any changed logic
5. **CLAUDE.md entries updated** if patterns, commands, or architecture changed

Self-correction: If build/test/lint fails, analyze the output, fix, and re-run immediately. Do NOT wait for prompting. Do NOT tailor tests to pass — a passing test MUST verify correctness.

Adapt the Definition of Done to scope. A typo fix does not require CLAUDE.md updates. A new public API does. Always think forward: what future changes might this impact?

## 2. Inline-First Documentation

Documentation MUST live in the source file as language-idiomatic doc comments. Standalone docs are permitted ONLY for:

- `README.md` — project introduction and quick-start
- `CONTRIBUTING.md` — development setup
- `CHANGELOG.md` — release history
- `SECURITY.md` — vulnerability reporting
- `CLAUDE.md` — Claude Code project primer (filepaths, architecture overview, conventions)

Everything else MUST be inline. If information exists in both a standalone doc and source-level comments, the standalone doc is the violation. Delete it and keep the inline version.

CLAUDE.md is a living primer. It MUST contain:
- Where key files live and how to work with them
- Build/test/lint commands
- Architecture overview (files, key exports, one-liner purposes)
- Conventions and patterns
- Legacy Traps / Non-Goals

CLAUDE.md MUST NOT contain:
- Full API documentation (that belongs in source-level doc comments)
- Tutorials or guides (those belong in README or dedicated user-facing docs)
- Duplicated content from sibling docs

## 3. RAG-Ready Writing

Doc blocks MUST NOT use pronouns ("this", "it", "the function") when the referent is ambiguous. Use explicit function/module names so each doc block is self-contained when retrieved in isolation.

```go
// Good:
// ProcessPayment validates the order amount, charges the payment
// provider, and returns a PaymentReceipt on success.

// Bad:
// This processes the payment. It validates and charges, then
// returns the result.
```

Every doc block MUST serve a purpose for a human or LLM reader. No obvious restating of what the code already says.

## 4. DRY Principle

Document a concept ONCE, reference it everywhere else. No duplication across files.

When the same information appears in multiple locations:
1. Identify the canonical source (usually the most specific location)
2. Keep the full content there
3. Replace all other copies with a one-liner reference

This applies especially to:
- Protocol rules (canonical in `git-ecosystem/rules/`)
- Architecture descriptions (canonical in each project's `CLAUDE.md`)
- API documentation (canonical in source-level doc comments)

Cross-project duplication is a vendoring problem. If multiple projects need the same content, vendor it from a canonical source rather than copy-pasting.

## 5. Staleness Tax

Every logic change MUST simultaneously update:
1. Inline doc comments on the changed function/type
2. CLAUDE.md if the change affects patterns, commands, or architecture
3. Relevant rule files if the change affects ecosystem-wide conventions

Stale documentation is worse than no documentation. It actively misleads. When modifying code, verify that all documentation touching that code is still accurate.

## 6. Negative Documentation

When an approach is rejected, document it as a "Legacy Trap" or "Non-Goal" in the project CLAUDE.md. This prevents future agents and contributors from re-discovering and re-proposing the same approach.

Format:
```text
## Non-Goals / Legacy Traps

- Do NOT use Cobra (manual parsing, zero deps)
- Do NOT parse trailers with regex (use git-plumbing's %(trailers) format)
- Do NOT store state outside git (no SQLite, no JSON files)
```

Each entry MUST explain WHY the approach was rejected, not just that it was.

## 7. Architectural Standards

- **RFC 2119 language**: Use MUST, MUST NOT, SHOULD, SHOULD NOT, MAY in all planning and technical feedback. Ambiguous strength words ("probably", "might want to") are not acceptable in specifications.
- **DRY/SOLID**: Propose a refactor before adding features on top of messy or duplicated logic. Three similar code paths is the threshold for extraction.
- **Negative documentation**: Every rejected approach gets a Legacy Trap entry (see section 6).

## 8. Communication Style

- High-context, low-prose. No hedging ("Based on my analysis..."). State the plan or the code.
- ALWAYS label code blocks with a language/format tag, even if just ` ```text `.
- Present options with numbered lists. When multiple approaches exist, present tradeoffs.
- Label response sections by topic (Section A, Point B) for easy reference.
- Each actionable content block (code, commands) gets its own labeled code block.
- When a topic veers off-task, recommend spinning it off to a new context with a ready-to-paste prompt carrying forward relevant prior context.
