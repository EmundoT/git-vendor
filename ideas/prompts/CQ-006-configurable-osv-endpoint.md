# PROMPT: CQ-006 Configurable OSV Endpoint

## Task
Make the OSV.dev API endpoint URL configurable for integration testing and air-gapped environments.

## Priority
MEDIUM - Improves testability and enables enterprise deployments.

## Problem
The OSV.dev API URLs are hardcoded constants in `vuln_scanner.go`:
```go
const (
    osvAPIEndpoint      = "https://api.osv.dev/v1/query"
    osvBatchAPIEndpoint = "https://api.osv.dev/v1/querybatch"
)
```

This makes it impossible to:
1. Use a mock OSV server in integration tests
2. Deploy in air-gapped environments with a local OSV mirror
3. Test rate limiting and error handling without hitting the real API

## Affected Files
- `internal/core/vuln_scanner.go`
- `internal/core/vuln_scanner_test.go`

## Implementation

### 1. Add Environment Variable Support
```go
const (
    defaultOSVAPIEndpoint      = "https://api.osv.dev/v1/query"
    defaultOSVBatchAPIEndpoint = "https://api.osv.dev/v1/querybatch"
)

// getOSVEndpoint returns the OSV API endpoint, checking env var first
func getOSVEndpoint() string {
    if endpoint := os.Getenv("GIT_VENDOR_OSV_ENDPOINT"); endpoint != "" {
        return endpoint
    }
    return defaultOSVAPIEndpoint
}

// getOSVBatchEndpoint returns the OSV batch API endpoint
func getOSVBatchEndpoint() string {
    if endpoint := os.Getenv("GIT_VENDOR_OSV_BATCH_ENDPOINT"); endpoint != "" {
        return endpoint
    }
    return defaultOSVBatchAPIEndpoint
}
```

### 2. Update VulnScanner Struct
```go
type VulnScanner struct {
    client          *http.Client
    cacheDir        string
    cacheTTL        time.Duration
    lockStore       LockStore
    configStore     ConfigStore
    osvEndpoint     string  // NEW
    osvBatchEndpoint string // NEW
}

func NewVulnScanner(lockStore LockStore, configStore ConfigStore) *VulnScanner {
    // ...existing code...

    return &VulnScanner{
        client:           &http.Client{},
        cacheDir:         cacheDir,
        cacheTTL:         cacheTTL,
        lockStore:        lockStore,
        configStore:      configStore,
        osvEndpoint:      getOSVEndpoint(),
        osvBatchEndpoint: getOSVBatchEndpoint(),
    }
}
```

### 3. Use Instance Fields Instead of Constants
Replace all uses of `osvAPIEndpoint` with `s.osvEndpoint` and `osvBatchAPIEndpoint` with `s.osvBatchEndpoint`.

### 4. Add Constructor for Testing
```go
// NewVulnScannerWithEndpoints creates a scanner with custom endpoints (for testing)
func NewVulnScannerWithEndpoints(
    lockStore LockStore,
    configStore ConfigStore,
    endpoint string,
    batchEndpoint string,
) *VulnScanner {
    scanner := NewVulnScanner(lockStore, configStore)
    scanner.osvEndpoint = endpoint
    scanner.osvBatchEndpoint = batchEndpoint
    return scanner
}
```

### 5. Integration Test Example
```go
func TestVulnScanner_MockServer(t *testing.T) {
    // Create mock OSV server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(osvResponse{
            Vulns: []osvVuln{
                {ID: "CVE-2024-1234", Summary: "Test vulnerability"},
            },
        })
    }))
    defer server.Close()

    scanner := NewVulnScannerWithEndpoints(
        mockLockStore,
        mockConfigStore,
        server.URL,
        server.URL+"/batch",
    )

    result, err := scanner.Scan("")
    // Assert expectations
}
```

### 6. Update CLAUDE.md Documentation
Add to Environment Variables section:
```markdown
- `GIT_VENDOR_OSV_ENDPOINT` - Override OSV.dev API endpoint (for testing/enterprise)
- `GIT_VENDOR_OSV_BATCH_ENDPOINT` - Override OSV.dev batch API endpoint
```

## Mandatory Tracking Updates
1. Update `ideas/code_quality.md` - change CQ-006 status to "completed"
2. Add completion notes

## Acceptance Criteria
- [ ] OSV endpoints configurable via environment variables
- [ ] `GIT_VENDOR_OSV_ENDPOINT` overrides single query endpoint
- [ ] `GIT_VENDOR_OSV_BATCH_ENDPOINT` overrides batch query endpoint
- [ ] Default endpoints unchanged when env vars not set
- [ ] Test constructor available for mock server testing
- [ ] CLAUDE.md documents new environment variables
- [ ] `go build ./...` - no compilation errors
- [ ] `go test ./...` - all tests pass

## GIT WORKFLOW (MANDATORY)
1. Commit your changes with a descriptive message
2. Fetch and merge from main:
   ```
   git fetch origin main
   git merge origin/main
   ```
3. Resolve any merge conflicts if they occur
4. Push to your branch:
   ```
   git push -u origin <your-branch-name>
   ```
