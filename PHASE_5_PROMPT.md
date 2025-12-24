# Phase 5: CI/CD & Automation Infrastructure

**Prerequisites:** Phase 4.5 complete (all tests passing, gomock mocks in place)
**Goal:** Add GitHub Actions CI/CD, release automation, and development tooling
**Priority:** HIGH - Essential for production deployment and team collaboration
**Estimated Effort:** 4-6 hours

---

## Current State

**What Works:**
- ✅ All 55 tests passing locally
- ✅ Coverage: 52.7% (critical paths 84-100%)
- ✅ Mocks auto-generated with `make mocks`
- ✅ Build successful with `go build`

**What's Missing:**
- ❌ No automated testing on PR/push
- ❌ No release automation
- ❌ No pre-commit hooks
- ❌ No code quality gates
- ❌ Manual binary builds only

**Problems:**
- Developers can push breaking changes without knowing
- No automated releases for new versions
- No consistent code formatting enforcement
- Manual testing required for every change
- No coverage tracking over time

---

## Goals

1. **GitHub Actions CI** - Automated testing on every PR/push
2. **Release Automation** - Build and publish binaries automatically
3. **Pre-commit Hooks** - Catch issues before commit
4. **Code Quality Tools** - Linting and formatting
5. **Coverage Reporting** - Track coverage trends
6. **Contributing Guide** - Document development workflow

---

## Implementation Steps

### 1. GitHub Actions: Test Workflow

Create `.github/workflows/test.yml`:

```yaml
name: Tests

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    name: Test on ${{ matrix.os }}
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
        go: ['1.21']

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: ${{ matrix.go }}

    - name: Install mockgen
      run: go install github.com/golang/mock/mockgen@latest

    - name: Generate mocks
      run: make mocks

    - name: Run tests
      run: go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

    - name: Upload coverage
      uses: codecov/codecov-action@v4
      with:
        file: ./coverage.txt
        flags: unittests
        name: codecov-${{ matrix.os }}

  lint:
    name: Lint
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'

    - name: golangci-lint
      uses: golangci/golangci-lint-action@v4
      with:
        version: latest
        args: --timeout=5m

  build:
    name: Build
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'

    - name: Build
      run: go build -v -o git-vendor

    - name: Verify binary
      run: ./git-vendor --help
```

### 2. GitHub Actions: Release Workflow

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      with:
        fetch-depth: 0

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.21'

    - name: Run GoReleaser
      uses: goreleaser/goreleaser-action@v5
      with:
        distribution: goreleaser
        version: latest
        args: release --clean
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 3. GoReleaser Configuration

Create `.goreleaser.yml`:

```yaml
version: 2

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
    binary: git-vendor

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}
    format_overrides:
      - goos: windows
        format: zip
    files:
      - LICENSE
      - README.md
      - TROUBLESHOOTING.md

checksum:
  name_template: 'checksums.txt'

snapshot:
  name_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
      - '^chore:'
  groups:
    - title: Features
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: Bug Fixes
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: Others
      order: 999

release:
  github:
    owner: yourusername
    name: git-vendor
  draft: false
  prerelease: auto
  mode: append
  footer: |
    ## Installation

    ### Using Go
    ```bash
    go install github.com/yourusername/git-vendor@{{ .Tag }}
    ```

    ### Binary Downloads
    Download the appropriate binary for your platform from the assets below.
```

### 4. Pre-commit Hook

Create `.githooks/pre-commit`:

```bash
#!/bin/bash
set -e

echo "🔍 Running pre-commit checks..."

# 1. Format check
echo "  → Checking formatting..."
if [ -n "$(gofmt -l .)" ]; then
  echo "❌ Code is not formatted. Run: gofmt -w ."
  gofmt -l .
  exit 1
fi

# 2. Generate mocks
echo "  → Generating mocks..."
make mocks > /dev/null 2>&1

# 3. Check for uncommitted mock files
if git diff --name-only | grep -q "_mock_test.go"; then
  echo "⚠️  Mock files changed but are git-ignored (expected)"
fi

# 4. Run tests
echo "  → Running tests..."
if ! go test ./... > /dev/null 2>&1; then
  echo "❌ Tests failed. Fix them before committing."
  exit 1
fi

# 5. Check for debugging artifacts
echo "  → Checking for debugging artifacts..."
if git diff --cached --name-only | xargs grep -l "fmt.Println\|panic(\"TODO" 2>/dev/null; then
  echo "⚠️  Found debugging statements. Remove them or commit anyway?"
  read -p "Continue? (y/N) " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 1
  fi
fi

echo "✅ Pre-commit checks passed!"
```

Make it executable and install:

```bash
chmod +x .githooks/pre-commit
git config core.hooksPath .githooks
```

### 5. golangci-lint Configuration

Create `.golangci.yml`:

```yaml
linters-settings:
  gofmt:
    simplify: true
  goimports:
    local-prefixes: git-vendor
  govet:
    check-shadowing: true
  errcheck:
    check-type-assertions: true
    check-blank: true
  gocritic:
    enabled-tags:
      - diagnostic
      - experimental
      - opinionated
      - performance
      - style

linters:
  enable:
    - gofmt
    - goimports
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck
    - gocritic
    - misspell
    - revive

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
        - gocritic

run:
  timeout: 5m
  tests: true
  modules-download-mode: readonly
```

### 6. Update Makefile

Add new targets to `Makefile`:

```makefile
# ... existing mocks target ...

.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	gofmt -w .

.PHONY: install-hooks
install-hooks:
	@echo "Installing git hooks..."
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "Done! Hooks installed."

.PHONY: ci
ci: mocks lint test
	@echo "All CI checks passed!"

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f git-vendor git-vendor.exe
	rm -f coverage.out coverage.txt
	rm -f internal/core/*_mock_test.go
	@echo "Done!"
```

### 7. Create CONTRIBUTING.md

Create `CONTRIBUTING.md`:

```markdown
# Contributing to git-vendor

## Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/yourusername/git-vendor
   cd git-vendor
   ```

2. **Install dependencies:**
   ```bash
   # Install mockgen
   go install github.com/golang/mock/mockgen@latest

   # Install golangci-lint
   curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
   ```

3. **Generate mocks:**
   ```bash
   make mocks
   ```

4. **Install pre-commit hooks:**
   ```bash
   make install-hooks
   ```

5. **Run tests:**
   ```bash
   make test
   ```

## Development Workflow

### Making Changes

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes

3. **Important:** If you modify any interface, regenerate mocks:
   ```bash
   make mocks
   ```

4. Run tests:
   ```bash
   make test
   ```

5. Format code:
   ```bash
   make fmt
   ```

6. Run linter:
   ```bash
   make lint
   ```

7. Commit your changes (pre-commit hook will run automatically)

### Pull Request Guidelines

- Write clear commit messages following [Conventional Commits](https://www.conventionalcommits.org/)
- Ensure all tests pass
- Maintain or improve test coverage (target: >60%)
- Update documentation if adding features
- Add tests for bug fixes
- Keep PRs focused on a single concern

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding or updating tests
- `docs`: Documentation only changes
- `chore`: Changes to build process or tools

**Example:**
```
feat(sync): add parallel vendor processing

Implement concurrent syncing of multiple vendors using goroutines
to reduce total sync time for projects with many dependencies.

Closes #123
```

## Testing

### Running Tests

```bash
# All tests
make test

# With coverage
make coverage

# Specific package
go test ./internal/core/...

# Verbose
go test -v ./...
```

### Writing Tests

- Use gomock for mocking dependencies
- Test both happy paths and error cases
- Use table-driven tests for multiple scenarios
- Name tests descriptively: `TestFunctionName_Scenario`

### Mock Generation

Mocks are auto-generated and git-ignored. Generate them with:

```bash
make mocks
```

**Never commit mock files** (`*_mock_test.go`).

## Code Quality

### Formatting

```bash
make fmt
```

### Linting

```bash
make lint
```

### CI Checks

All PRs must pass:
- Tests on Linux, macOS, Windows
- golangci-lint
- Build verification

## Release Process

Releases are automated via GitHub Actions:

1. Tag a new version:
   ```bash
   git tag -a v1.2.3 -m "Release v1.2.3"
   git push origin v1.2.3
   ```

2. GitHub Actions will:
   - Run all tests
   - Build binaries for all platforms
   - Create GitHub release
   - Attach binaries and checksums

## Questions?

Open an issue or discussion on GitHub!
```

### 8. Update README.md

Add CI badge and development section:

```markdown
# git-vendor

[![Tests](https://github.com/yourusername/git-vendor/workflows/Tests/badge.svg)](https://github.com/yourusername/git-vendor/actions)
[![codecov](https://codecov.io/gh/yourusername/git-vendor/branch/main/graph/badge.svg)](https://codecov.io/gh/yourusername/git-vendor)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/git-vendor)](https://goreportcard.com/report/github.com/yourusername/git-vendor)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<!-- ... rest of README ... -->

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

**Quick Start:**
```bash
make mocks    # Generate mocks
make test     # Run tests
make lint     # Run linter
make ci       # Run all CI checks
```
```

---

## Verification Checklist

After implementing Phase 5, verify:

- [ ] `.github/workflows/test.yml` exists and runs on PR
- [ ] `.github/workflows/release.yml` exists
- [ ] `.goreleaser.yml` configured correctly
- [ ] Pre-commit hook installed and working
- [ ] `make lint` passes
- [ ] `make ci` passes all checks
- [ ] CONTRIBUTING.md created
- [ ] README.md has CI badges
- [ ] Create a test tag and verify release builds

**Test the CI:**
```bash
# 1. Create a test branch
git checkout -b test/ci-phase5

# 2. Make a small change
echo "# Test" >> test.txt
git add test.txt
git commit -m "test: verify CI works"

# 3. Push and create PR
git push -u origin test/ci-phase5

# 4. Verify GitHub Actions runs
# Check: https://github.com/yourusername/git-vendor/actions
```

---

## Expected Outcomes

**After Phase 5:**
- ✅ Automated testing on every PR (Linux, macOS, Windows)
- ✅ Code quality gates prevent bad code from merging
- ✅ Release automation (tag → binaries published)
- ✅ Pre-commit hooks catch issues before commit
- ✅ Coverage tracking over time
- ✅ Professional development workflow
- ✅ Easy onboarding for new contributors

**Metrics:**
- CI run time: <5 minutes
- Release time: <10 minutes
- Developer friction: Minimal (hooks, automation)
- Code quality: Consistent (linting enforced)

---

## Common Issues & Solutions

### Issue: golangci-lint Too Slow

**Solution:** Configure timeout and enable caching:
```yaml
# .golangci.yml
run:
  timeout: 10m
  build-tags:
    - integration
```

### Issue: Pre-commit Hook Fails on Windows

**Solution:** Use Git Bash or WSL, or create `.githooks/pre-commit.bat` for Windows

### Issue: GoReleaser Fails to Build

**Solution:** Test locally first:
```bash
goreleaser release --snapshot --clean
```

### Issue: Mock Files Cause CI Failure

**Solution:** Ensure `.gitignore` has `internal/core/*_mock_test.go` and CI regenerates them

---

## Next Steps

After Phase 5 completion:
- **Phase 6:** Multi-platform git support (GitLab, Bitbucket)
- **Phase 7:** Enhanced testing and quality (70%+ coverage)
- **Phase 8:** Advanced features (update checker, parallelism)

**Priority:** Phase 5 is HIGH priority for production deployment.
