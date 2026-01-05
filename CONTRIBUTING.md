# Contributing to git-vendor

## Development Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/EmundoT/git-vendor
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
- Maintain or improve test coverage (check current: `make coverage`)
- Update documentation if adding features
- Add tests for bug fixes
- Keep PRs focused on a single concern

### Commit Message Format

```text
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

```text
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

Releases are managed by maintainers using GoReleaser. See [RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md) for detailed instructions.

**For contributors:** Focus on writing good conventional commit messages. These are used to auto-generate release notes.

## Questions?

Open an issue or discussion on GitHub!
