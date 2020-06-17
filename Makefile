SHELL := /bin/bash

ROOT := $(shell git rev-parse --show-toplevel)
GO_FILES := $(shell find . -name "*.go" -not -path "./build/*" -not -path "**/vendor/*")

VERSION = $(shell git describe --dirty="-dev")

.DEFAULT_GOAL := error

.PHONY: error
error:
	@echo "Please check 'README.md' for instructions on how to build and deploy 'github-team-approver'."
	@exit 2

SKAFFOLD_VERSION ?= v1.6.0

platform := $(shell uname)
pact_version := 1.64.0
ifeq (${platform},Darwin)
    pact_filename := "pact-${pact_version}-osx.tar.gz"
	skaffold_url := https://storage.googleapis.com/skaffold/releases/$(SKAFFOLD_VERSION)/skaffold-darwin-amd64
else
    pact_filename := "pact-${pact_version}-linux-x86_64.tar.gz"
	skaffold_url := https://storage.googleapis.com/skaffold/releases/$(SKAFFOLD_VERSION)/skaffold-linux-amd64
endif

.PHONY: install-deps
install-deps: install-goimports install-pact install-skaffold

.PHONY: install-goimports
install-goimports:
	go get -u golang.org/x/tools/cmd/goimports

.PHONY: install-skaffold
install-skaffold:
	@curl -Lo skaffold-bin ${skaffold_url}
	@chmod +x skaffold-bin
	@sudo mv skaffold-bin /usr/local/bin/skaffold

.PHONY: install-pact
install-pact:
	curl -LO https://github.com/pact-foundation/pact-ruby-standalone/releases/download/v${pact_version}/${pact_filename}
	tar xzf ${pact_filename}
	rm ${pact_filename}

.PHONY: dep
dep:
	cd github-team-approver && dep ensure -v

.PHONY: goimports
goimports:
	goimports -w $(GO_FILES)

.PHONY: secret
secret: GITHUB_APP_PRIVATE_KEY_PATH ?= $(ROOT)/github-app-private-key
secret: GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH ?= $(ROOT)/github-app-webhook-secret-token
secret: ENCRYPTION_KEY_PATH ?= $(ROOT)/encryption.key
secret: LOGZIO_TOKEN_PATH ?= $(ROOT)/logzio-token
secret: NAMESPACE ?= github-team-approver
secret:
	@kubectl -n $(NAMESPACE) create secret generic github-team-approver \
		--from-file github-app-private-key=$(GITHUB_APP_PRIVATE_KEY_PATH) \
		--from-file github-app-webhook-secret-token=$(GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH) \
		--from-file encryption_key=$(ENCRYPTION_KEY_PATH) \
		--from-file logzio-token=$(LOGZIO_TOKEN_PATH) \
		--dry-run \
		-o yaml | kubectl apply -n $(NAMESPACE) -f-

.PHONY: skaffold.push
skaffold.push:
	@IMAGE_TAG=$(VERSION) skaffold build --profile push

.PHONY: skaffold.dev
skaffold.dev: GITHUB_APP_ID ?= 
skaffold.dev: GITHUB_APP_INSTALLATION_ID ?= 
skaffold.dev: NAMESPACE ?= github-team-approver
skaffold.dev:
	@GITHUB_APP_ID=$(GITHUB_APP_ID) GITHUB_APP_INSTALLATION_ID=$(GITHUB_APP_INSTALLATION_ID) NAMESPACE=$(NAMESPACE) $(ROOT)/hack/helm-pre-skaffold-template.sh
	@skaffold dev

.PHONY: test
test: EXAMPLES_DIR := $(ROOT)/examples/github
test:
	EXAMPLES_DIR=$(EXAMPLES_DIR) \
	GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH=$(EXAMPLES_DIR)/token.txt \
	ENCRYPTION_KEY_PATH=$(EXAMPLES_DIR)/test.key \
	GITHUB_STATUS_NAME=github-team-approver \
	RUN_PACT_TESTS=1 \
	go test ./internal/... -count 1 -cover
