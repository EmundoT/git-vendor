MOCKGEN := $(shell go env GOPATH)/bin/mockgen

.PHONY: mocks
mocks:
	@echo "Generating mocks in core package..."
	go install github.com/golang/mock/mockgen@latest
	$(MOCKGEN) -source=internal/core/git_operations.go -destination=internal/core/git_client_mock_test.go -package=core
	$(MOCKGEN) -source=internal/core/filesystem.go -destination=internal/core/filesystem_mock_test.go -package=core
	$(MOCKGEN) -source=internal/core/config_store.go -destination=internal/core/config_store_mock_test.go -package=core
	$(MOCKGEN) -source=internal/core/lock_store.go -destination=internal/core/lock_store_mock_test.go -package=core
	$(MOCKGEN) -source=internal/core/github_client.go -destination=internal/core/license_checker_mock_test.go -package=core
	@echo "Done!"

.PHONY: test
test:
	go test -mod=mod -v ./...

.PHONY: coverage
coverage:
	go test -mod=mod -cover ./...

.PHONY: test-core
test-core:
	go test -mod=mod -v ./internal/core/...

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

.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	go test -tags=integration -v ./internal/core/...

.PHONY: test-all
test-all: mocks test test-integration
	@echo "All tests passed!"

.PHONY: bench
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./internal/core/ | tee benchmark.txt

.PHONY: test-coverage-html
test-coverage-html:
	@echo "Generating HTML coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in browser"

.PHONY: test-property
test-property:
	@echo "Running property-based tests..."
	go test -v -run Property ./internal/core/

.PHONY: build
build:
	@echo "Building optimized binary..."
	go build -ldflags="-s -w" -o git-vendor
	@echo "Done! Binary: git-vendor"

.PHONY: build-dev
build-dev:
	@echo "Building development binary (with debug info)..."
	go build -o git-vendor
	@echo "Done! Binary: git-vendor"

.PHONY: install-man
install-man:
	@echo "Installing man page..."
	sudo mkdir -p /usr/local/share/man/man1
	sudo cp docs/man/git-vendor.1 /usr/local/share/man/man1/
	@echo "Done! Try: man git-vendor"

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f git-vendor git-vendor.exe
	rm -f coverage.out coverage.txt coverage.html
	rm -f benchmark.txt
	rm -f internal/core/*_mock_test.go
	@echo "Done!"
