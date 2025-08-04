all: brain-freeze

help: ## Display help
	awk -F ':|##' \
	  '/^\S+:.*##/ { printf "\033[36m%-30s\033[0m %s\n", $$1, $$NF }' \
	  $(MAKEFILE_LIST)
.PHONY: help

lint: ## Run linting
	if ! command -v golangci-lint >& /dev/null; then \
	    brew install golangci-lint; \
	fi
	golangci-lint run
.PHONY: lint

brain-freeze: go.sum go.mod main.go cmd ## Build brain-freeze
	go build

clean:
	rm -rf dist data brain-freeze
.PHONY: clean
