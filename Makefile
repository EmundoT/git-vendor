MOCKGEN := $(shell go env GOPATH)/bin/mockgen

.PHONY: mocks
mocks:
	@echo "Generating mocks..."
	go install github.com/golang/mock/mockgen@latest
	$(MOCKGEN) -source=internal/core/git_operations.go -destination=internal/core/mocks/git_client_mock.go -package=mocks
	$(MOCKGEN) -source=internal/core/filesystem.go -destination=internal/core/mocks/filesystem_mock.go -package=mocks
	$(MOCKGEN) -source=internal/core/config_store.go -destination=internal/core/mocks/config_store_mock.go -package=mocks
	$(MOCKGEN) -source=internal/core/lock_store.go -destination=internal/core/mocks/lock_store_mock.go -package=mocks
	$(MOCKGEN) -source=internal/core/github_client.go -destination=internal/core/mocks/license_checker_mock.go -package=mocks
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
