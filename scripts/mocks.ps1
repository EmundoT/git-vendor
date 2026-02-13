#!/usr/bin/env pwsh
# scripts/mocks.ps1 — Regenerate gomock mocks (cross-platform, no make required).
# Usage: pwsh scripts/mocks.ps1
# Equivalent to `make mocks` on Unix.

$ErrorActionPreference = "Stop"

Write-Host "Syncing Go vendor directory..."
go mod vendor
if ($LASTEXITCODE -ne 0) { throw "go mod vendor failed" }

Write-Host "Installing mockgen..."
go install github.com/golang/mock/mockgen@latest
if ($LASTEXITCODE -ne 0) { throw "mockgen install failed" }

$mockgen = Join-Path (Join-Path (go env GOPATH) "bin") "mockgen"
if ($IsWindows -or $env:OS -match "Windows") { $mockgen += ".exe" }

$sources = @(
    @{ Source = "internal/core/git_operations.go"; Dest = "internal/core/git_client_mock_test.go" }
    @{ Source = "internal/core/filesystem.go";     Dest = "internal/core/filesystem_mock_test.go" }
    @{ Source = "internal/core/config_store.go";   Dest = "internal/core/config_store_mock_test.go" }
    @{ Source = "internal/core/lock_store.go";     Dest = "internal/core/lock_store_mock_test.go" }
    @{ Source = "internal/core/github_client.go";  Dest = "internal/core/license_checker_mock_test.go" }
)

Write-Host "Generating mocks in core package..."
foreach ($mock in $sources) {
    $src = "-source=" + $mock.Source
    $dst = "-destination=" + $mock.Dest
    & $mockgen $src $dst "-package=core"
    if ($LASTEXITCODE -ne 0) { throw ("mockgen failed for " + $mock.Source) }
}

Write-Host "Done!"
