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

.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f git-vendor git-vendor.exe
	rm -f coverage.out coverage.txt
	rm -f internal/core/*_mock_test.go
	@echo "Done!"
