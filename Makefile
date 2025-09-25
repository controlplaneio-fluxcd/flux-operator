# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

# Makefile for building, testing, and deploying the Flux Operator, CLI and MCP server.

# Image URL to use for all building/pushing image targets
IMG ?= ghcr.io/controlplaneio-fluxcd/flux-operator:latest
FLUX_OPERATOR_VERSION ?= $(shell gh release view --json tagName -q '.tagName')
FLUX_OPERATOR_DEV_VERSION?=0.0.0-$(shell git rev-parse --abbrev-ref HEAD)-$(shell git rev-parse --short HEAD)-$(shell date +%s)
FLUX_VERSION = $(shell gh release view --repo fluxcd/flux2 --json tagName -q '.tagName')
ENVTEST_K8S_VERSION = 1.34.1

# Get the currently used golang install path
# (in GOPATH/bin, unless GOBIN is set).
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: test lint build cli-build mcp-build ## Run all build and test targets.

##@ Development

.PHONY: generate
generate: controller-gen ## Generate deep copies and CRDs from API types.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy.
	go mod tidy

.PHONY: test
test: tidy generate fmt vet envtest ## Run all unit tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v -e /e2e -e /olm) -coverprofile cover.out

.PHONY: test-e2e
test-e2e: ## Run the e2e tests on the Kind cluster.
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci linters.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci linters and perform fixes.
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: generate fmt vet ## Build the flux-operator binary.
	CGO_ENABLED=0 GOFIPS140=latest go build -ldflags="-s -w -X main.VERSION=$(FLUX_OPERATOR_DEV_VERSION)" -o ./bin/flux-operator ./cmd/operator/

.PHONY: run
run: generate fmt vet ## Run the flux-operator locally.
	CGO_ENABLED=0 GOFIPS140=latest go run ./cmd/operator/main.go --enable-leader-election=false

.PHONY: docker-build
docker-build: ## Build flux-operator docker image.
	$(CONTAINER_TOOL) build -t ${IMG} --build-arg VERSION=$(FLUX_OPERATOR_DEV_VERSION) .

.PHONY: docker-push
docker-push: ## Push flux-operator docker image.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: build-installer
build-installer: generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml

.PHONY: build-manifests
build-manifests: ## Generate release manifests with CRDs, RBAC and deployment.
	hack/build-dist-manifests.sh

.PHONY: vendor-flux
vendor-flux: ## Download Flux base manifests and image patches to config/flux dir.
	hack/vendor-flux-manifests.sh $(FLUX_VERSION)

##@ OLM

OLM_VERSION ?= 0.28.0

.PHONY: build-olm-manifests
build-olm-manifests: ## Generate OLM manifests for OperatorHub.
	./hack/build-olm-manifests.sh $(FLUX_OPERATOR_VERSION:v%=%)

.PHONY: build-olm-manifests-ubi
build-olm-manifests-ubi: ## Generate OLM manifests for Red Hat Marketplace.
	./hack/build-olm-manifests.sh $(FLUX_OPERATOR_VERSION:v%=%) true

.PHONY: docker-build-ubi
docker-build-ubi: ## Build flux-operator docker image using UBI base image.
	$(CONTAINER_TOOL) build -t ${IMG}-ubi --build-arg VERSION=$(FLUX_OPERATOR_VERSION) -f config/olm/build/Dockerfile .

.PHONY: test-olm-e2e
test-olm-e2e: build-olm-manifests operator-sdk ## Test OLM manifests for current version.
	./hack/build-olm-images.sh $(FLUX_OPERATOR_VERSION:v%=%)
	export OLM_VERSION=${OLM_VERSION} && \
	export FLUX_OPERATOR_VERSION=$(FLUX_OPERATOR_VERSION:v%=%) && \
	export OPERATOR_SDK_BIN=$(OPERATOR_SDK) && \
	go test ./test/olm/ -v -ginkgo.v

##@ CLI

CLI_IMG ?= ghcr.io/controlplaneio-fluxcd/flux-operator-cli:latest

.PHONY: cli-test
cli-test: tidy fmt vet ## Run CLI tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./cmd/cli/... -v

.PHONY: cli-build
cli-build: tidy fmt vet ## Build CLI binary.
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.VERSION=$(FLUX_OPERATOR_DEV_VERSION)" -o ./bin/flux-operator-cli ./cmd/cli/

.PHONY: cli-docker-build
cli-docker-build: ## Build docker image with the CLI.
	$(CONTAINER_TOOL) build -t ${CLI_IMG} --build-arg VERSION=$(FLUX_OPERATOR_VERSION) -f cmd/cli/Dockerfile .

##@ MCP

MCP_IMG ?= ghcr.io/controlplaneio-fluxcd/flux-operator-mcp:latest

.PHONY: mcp-test
mcp-test: tidy fmt vet ## Run MCP tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./cmd/mcp/... -v

.PHONY: mcp-build
mcp-build: tidy fmt vet ## Build MCP server binary.
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.VERSION=$(FLUX_OPERATOR_DEV_VERSION)" -o ./bin/flux-operator-mcp ./cmd/mcp/

.PHONY: mcp-docker-build
mcp-docker-build: ## Build docker image with the MCP server.
	$(CONTAINER_TOOL) build -t ${MCP_IMG} --build-arg VERSION=$(FLUX_OPERATOR_VERSION) -f cmd/mcp/Dockerfile .

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: load-image
load-image: ## Load flux-operator image into the local Kind cluster.
	kind load docker-image ${IMG}

.PHONY: install
install: generate kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: generate kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: generate kustomize ## Deploy flux-operator to the K8s cluster specified in ~/.kube/config.
	mkdir -p config/dev && cp config/default/* config/dev
	cd config/dev && $(KUSTOMIZE) edit set image ghcr.io/controlplaneio-fluxcd/flux-operator=${IMG}
	$(KUSTOMIZE) build config/dev | $(KUBECTL) apply -f -
	rm -rf config/dev

.PHONY: undeploy
undeploy: kustomize ## Delete flux-operator from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Release

NEXT_VERSION ?= ""

.PHONY: prep-release
prep-release: ## Create release PR for the next version (auto minor bump).
	hack/vendor-flux-manifests.sh $(FLUX_VERSION)
	hack/prep-release.sh $(NEXT_VERSION)

.PHONY: release
release: ## Create a release for the next version.
	@next=$(shell yq '.images[0].newTag' "config/manager/kustomization.yaml") && \
	git tag -s -m "$$next" "$$next" && \
	git push origin "$$next"

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk-$(OPERATOR_SDK_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.7.0
CONTROLLER_TOOLS_VERSION ?= v0.19.0
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
GOLANGCI_LINT_VERSION ?= v2.5.0
OPERATOR_SDK_VERSION ?= v1.41.1

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK) ## Download operator-sdk locally if necessary.
$(OPERATOR_SDK): $(LOCALBIN)
	./hack/install-operator-sdk.sh $(OPERATOR_SDK_VERSION)

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
