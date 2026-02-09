# vendor.yml Examples

This directory contains example vendor configurations for common use cases.

## Quick Reference

| Example | Use Case |
|---------|----------|
| [basic-vendor.yml](./basic-vendor.yml) | Single file vendoring |
| [multi-file-vendor.yml](./multi-file-vendor.yml) | Multiple files from same repo |
| [directory-vendor.yml](./directory-vendor.yml) | Vendoring entire directories |
| [multi-branch-vendor.yml](./multi-branch-vendor.yml) | Tracking multiple versions/branches |
| [monorepo-vendor.yml](./monorepo-vendor.yml) | Vendoring from monorepo structure |
| [multiple-repos-vendor.yml](./multiple-repos-vendor.yml) | Multiple vendor dependencies |
| [default-target-vendor.yml](./default-target-vendor.yml) | Using default_target for convenience |

## Example Structure

### Basic Format

```yaml
vendors:
  - name: vendor-name          # Display name
    url: https://github.com/owner/repo
    license: MIT               # Auto-detected by git-vendor
    specs:
      - ref: main              # Branch, tag, or commit
        default_target: ""     # Optional base directory
        mapping:
          - from: remote/path  # Path in repository
            to: local/path     # Destination (empty = auto)
```

## Common Patterns

### Auto-Naming

Leave `to` empty to use automatic naming:

```yaml
- from: src/utils/helper.go
  to: ""  # Will use "helper.go"
```

### Multiple Refs

Track multiple versions from the same repository:

```yaml
specs:
  - ref: v1.0
    mapping: [...]
  - ref: v2.0
    mapping: [...]
```

### Default Target

Set a base directory for all mappings:

```yaml
specs:
  - ref: main
    default_target: internal/vendor
    mapping:
      - from: utils.go
        to: ""  # → internal/vendor/utils.go
```

### Directory Vendoring

Vendor entire directories by using directory paths:

```yaml
- from: src/components
  to: vendor/components  # Entire directory copied
```

## Tips

1. **Use version tags** instead of branches for stability:

   ```yaml
   ref: v1.2.3  # Good
   ref: main    # Less stable
   ```

2. **Vendor specific files** rather than entire repos:

   ```yaml
   # Better: Vendor only what you need
   - from: src/utils/string.go
     to: internal/string.go

   # Avoid: Vendoring entire repository
   - from: .
     to: vendor/entire-repo
   ```

3. **Use meaningful local paths** that reflect your project structure:

   ```yaml
   # Good: Clear organization
   - from: client/http.go
     to: internal/vendor/api/http.go

   # Avoid: Unclear organization
   - from: client/http.go
     to: stuff/thing.go
   ```

4. **Document why you're vendoring** in comments:

   ```yaml
   # We vendor only the retry logic from this library
   # because the full library has dependencies we don't need
   - from: retry/retry.go
     to: internal/retry.go
   ```

## Testing Your Configuration

After creating your `vendor.yml`:

```bash
# 1. Validate by running update
git-vendor update

# 2. Preview what will be synced
git-vendor sync --dry-run

# 3. Apply the configuration
git-vendor sync

# 4. Verify vendored files
ls -la <your-destination-paths>
```

## Getting Started

1. Copy an example that matches your use case
2. Modify the URLs, paths, and refs
3. Place it in your project as `.git-vendor/vendor.yml`
4. Run `git-vendor sync`

For more information, see the [main README](../README.md).
