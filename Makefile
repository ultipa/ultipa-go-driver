# Copyright 2024 Your Name <your.email@example.com>. All rights reserved.
# Use of this source code is governed by a MIT style
# license that can be found in the LICENSE file.

# Build all by default, even if it's not first
.DEFAULT_GOAL := all
TEST_OUTPUT := ./test/data/test_output.log

.PHONY: all
all: tidy format lint test check-coverage

# ==============================================================================
# Build options

ROOT_PACKAGE = ultipa-go-driver
VERSION_PACKAGE = v4.5.0-s4.5

# ==============================================================================
# Targets

## lint: Check syntax and styling of go sources.
.PHONY: lint
lint:
	@echo "===========> Linting code"
	go install golang.org/x/lint/golint@latest
	@find . -name '*.go' ! -path './rpc/*' | xargs golint

## test: Run unit tests and output test results and coverage.
.PHONY: test
test:
	@echo "===========> Clearing previous test output"
	@> $(TEST_OUTPUT)
	@echo "===========> Running tests"
	(go test -timeout 20m -v -coverprofile=coverage.out ./test/... 2>&1 | tee $(TEST_OUTPUT))
	@echo "===========> Generating coverage report"
	(go tool cover -html=coverage.out -o coverage.html 2>&1 | tee -a $(TEST_OUTPUT))


## cover: Check coverage report without running tests again.
.PHONY: cover
cover:
	@echo "===========> Generating coverage report"
	go tool cover -func=coverage.out | tee coverage.txt

## check-coverage: Ensure test coverage is above 98%.
.PHONY: check-coverage
check-coverage:
	@COVERAGE=$(shell go tool cover -func=coverage.out | grep total: | awk '{print $$3}' | sed 's/%//'); \
	if [ "$$COVERAGE" != "" ] && [ "$$COVERAGE" != "0" ] && [ "$$COVERAGE" != "0.00" ] && [ "$$COVERAGE" \< "98.00" ]; then \
		echo "Test coverage below 98%!"; \
		exit 1; \
	else \
		echo "Test coverage meets the requirement."; \
	fi

## format: Gofmt (reformat) package sources (exclude rpc dir if existed).
.PHONY: format
format:
	@echo "===========> Formatting code"
	@find . -type f -name '*.go' ! -path './rpc/*' | xargs gofmt -s -w
	@find . -type f -name '*.go' ! -path './rpc/*' | xargs goimports -w -local $(ROOT_PACKAGE)

## tidy: Tidy up module dependencies.
.PHONY: tidy
tidy:
	@echo "===========> Tidying up module dependencies"
	go mod tidy

## help: Show this help info.
.PHONY: help
help:
	@printf "\nUsage: make <TARGETS> <OPTIONS> ...\n\nTargets:\n"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo "\nOptions:\n  ROOT_PACKAGE=$(ROOT_PACKAGE)\n  VERSION_PACKAGE=$(VERSION_PACKAGE)"