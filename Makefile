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
	go test -v ./...

.PHONY: coverage
coverage:
	go test -cover ./...

.PHONY: test-core
test-core:
	go test -v ./internal/core/...
