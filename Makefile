SHELL := /bin/bash

ROOT_DIR := $(shell git rev-parse --show-toplevel)
GO_FILES := $(shell find . -name "*.go" -not -path "./build/*" -not -path "./template/*" -not -path "**/vendor/*")
TEMPLATE := golang-http

VERSION = $(shell git describe --dirty="-dev")
DOCKER_IMG = form3tech/github-team-approver
DOCKER_TAG = $(VERSION)

.DEFAULT_GOAL := error

.PHONY: error
error:
	@echo "Please check 'README.md' for instructions on how to build and deploy 'github-team-approver'."
	@exit 2

platform := $(shell uname)
pact_version := "1.64.0"

ifeq (${platform},Darwin)
	faas := "$(ROOT_DIR)/faas-cli-darwin"
	pact_filename := "pact-${pact_version}-osx.tar.gz"
else
	faas := "$(ROOT_DIR)/faas-cli"
	pact_filename := "pact-${pact_version}-linux-x86_64.tar.gz"
endif

.PHONY: install-deps
install-deps: install-dep install-faas install-goimports install-pact

.PHONY: install-dep
install-dep:
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

.PHONY: install-faas
install-faas:
	curl -sSL https://cli.openfaas.com | sh

.PHONY: install-goimports
install-goimports:
	go get -u golang.org/x/tools/cmd/goimports

.PHONY: install-pact
install-pact:
	curl -LO https://github.com/pact-foundation/pact-ruby-standalone/releases/download/v${pact_version}/${pact_filename}
	tar xzf ${pact_filename}
	rm ${pact_filename}

.PHONY: build
build: dep $(ROOT_DIR)/template/$(TEMPLATE)
	DOCKER_IMG=$(DOCKER_IMG) DOCKER_TAG=$(DOCKER_TAG) $(faas) build

.PHONY: dep
dep:
	cd github-team-approver && dep ensure -v

.PHONY: deploy
deploy: APP_NAME ?= github-team-approver
deploy: GITHUB_APP_ID ?=
deploy: GITHUB_APP_INSTALLATION_ID ?=
deploy: IGNORED_REPOSITORIES ?=
deploy: LOG_LEVEL ?= info
deploy: MAX_REPLICAS ?= 1
deploy: MIN_REPLICAS ?= 1
deploy: STATUS_NAME ?=
deploy:
	APP_NAME=$(APP_NAME) \
	DOCKER_IMG=$(DOCKER_IMG) \
	DOCKER_TAG=$(DOCKER_TAG) \
	GITHUB_APP_ID=$(GITHUB_APP_ID) \
	GITHUB_APP_INSTALLATION_ID=$(GITHUB_APP_INSTALLATION_ID) \
	IGNORED_REPOSITORIES=$(IGNORED_REPOSITORIES) \
	LOG_LEVEL=$(LOG_LEVEL) \
	MAX_REPLICAS=$(MAX_REPLICAS) \
	MIN_REPLICAS=$(MIN_REPLICAS) \
	STATUS_NAME=$(STATUS_NAME) \
	$(faas) deploy

.PHONY: goimports
goimports:
	goimports -w $(GO_FILES)

.PHONY: push
push:
	echo "$(DOCKER_PASSWORD)" | docker login -u "$(DOCKER_USERNAME)" --password-stdin
	DOCKER_IMG=$(DOCKER_IMG) DOCKER_TAG=$(DOCKER_TAG) $(faas) push

.PHONY: secrets
secrets: GITHUB_APP_PRIVATE_KEY ?= $(ROOT_DIR)/github-app-private-key.pem
secrets: GITHUB_APP_WEBHOOK_SECRET_TOKEN ?= $(ROOT_DIR)/github-app-webhook-secret-token
secrets: LOGZIO_TOKEN ?= $(ROOT_DIR)/logzio-token
secrets:
	$(faas) secret create \
	    github-team-approver-private-key \
	    --from-file $(GITHUB_APP_PRIVATE_KEY)
	$(faas) secret create \
	    github-team-approver-webhook-secret-token \
	    --from-file $(GITHUB_APP_WEBHOOK_SECRET_TOKEN)
	$(faas) secret create \
	    github-team-approver-logzio-token \
	    --from-file $(LOGZIO_TOKEN)

.PHONY: test
test:
	GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH=$(ROOT_DIR)/github-team-approver/examples/github/token.txt \
	RUN_PACT_TESTS=1 \
	STATUS_NAME=github-team-approver \
	go test $(GO_FILES) -count 1 -cover

$(ROOT_DIR)/template/$(TEMPLATE):
	@$(faas) template store pull golang-http

.PHONY: up
up: APP_NAME ?= github-team-approver
up: GITHUB_APP_ID ?=
up: GITHUB_APP_INSTALLATION_ID ?=
up: IGNORED_REPOSITORIES ?=
up: LOG_LEVEL ?= info
up: MAX_REPLICAS ?= 1
up: MIN_REPLICAS ?= 1
up: STATUS_NAME ?=
up: dep
	APP_NAME=$(APP_NAME) \
	DOCKER_IMG=$(DOCKER_IMG) \
	DOCKER_TAG=$(DOCKER_TAG) \
	GITHUB_APP_ID=$(GITHUB_APP_ID) \
	GITHUB_APP_INSTALLATION_ID=$(GITHUB_APP_INSTALLATION_ID) \
	IGNORED_REPOSITORIES=$(IGNORED_REPOSITORIES) \
	LOG_LEVEL=$(LOG_LEVEL) \
	MAX_REPLICAS=$(MAX_REPLICAS) \
	MIN_REPLICAS=$(MIN_REPLICAS) \
	STATUS_NAME=$(STATUS_NAME) \
	$(faas) up
