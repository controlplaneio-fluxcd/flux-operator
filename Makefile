# Copyright 2024 Stefan Prodan.
# SPDX-License-Identifier: AGPL-3.0

# Makefile for building, testing, and deploying the Flux Operator.

# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/controlplaneio-fluxcd/flux-operator:latest

# FLUX_OPERATOR_VERSION refers to the version of the operator to be tested
# under ./config/operatorhub/flux-operator/<version> directory.
FLUX_OPERATOR_VERSION ?= $(shell gh release view --json tagName -q '.tagName')
# OLM_VERSION refers to the version of the Operator Lifecycle Manager to be used.
OLM_VERSION ?= 0.28.0

# FLUX_VERSION refers to the version of Flux to be vendored.
FLUX_VERSION = $(shell gh release view --repo fluxcd/flux2 --json tagName -q '.tagName')

# ENVTEST_K8S_VERSION refers to the version of kubebuilder
# assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.30.0

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
all: build

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

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
test: tidy manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v -e /e2e -e /olm) -coverprofile cover.out

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: vendor-flux
vendor-flux: ## Download Flux base manifests to config/flux dir.
	hack/vendor-flux-manifests.sh $(FLUX_VERSION)

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

PLATFORMS ?= linux/arm64,linux/amd64
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support.
	- $(CONTAINER_TOOL) buildx create --name flux-operator-builder
	$(CONTAINER_TOOL) buildx use flux-operator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm flux-operator-builder

docker-build-ubi: ## Build docker image with the manager using UBI base image.
	$(CONTAINER_TOOL) build -t ${IMG}-ubi --build-arg VERSION=$(FLUX_OPERATOR_VERSION) -f config/olm/build/Dockerfile .

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	$(KUSTOMIZE) build config/default > dist/install.yaml

.PHONY: build-manifests
build-manifests: ## Generate release manifests.
	hack/build-dist-manifests.sh

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: load-image
load-image: ## Load image into a Kind cluster.
	kind load docker-image ${IMG}

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	mkdir -p config/dev && cp config/default/* config/dev
	cd config/dev && $(KUSTOMIZE) edit set image ghcr.io/controlplaneio-fluxcd/flux-operator=${IMG}
	$(KUSTOMIZE) build config/dev | $(KUBECTL) apply -f -
	rm -rf config/dev

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: build-olm-manifests
build-olm-manifests: ## Generate OLM manifests for OperatorHub.
	./hack/build-olm-manifests.sh $(FLUX_OPERATOR_VERSION:v%=%)

.PHONY: build-olm-manifests-ubi
build-olm-manifests-ubi: ## Generate OLM manifests for OperatorHub.
	./hack/build-olm-manifests.sh $(FLUX_OPERATOR_VERSION:v%=%) true

.PHONY: test-olm
test-olm: build-olm-manifests operator-sdk ## Test OLM manifests for current version.
	./hack/build-olm-images.sh $(FLUX_OPERATOR_VERSION:v%=%)
	export OLM_VERSION=${OLM_VERSION} && \
	export FLUX_OPERATOR_VERSION=$(FLUX_OPERATOR_VERSION:v%=%) && \
	export OPERATOR_SDK_BIN=$(OPERATOR_SDK) && \
	go test ./test/olm/ -v -ginkgo.v

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
KUSTOMIZE_VERSION ?= v5.4.1
CONTROLLER_TOOLS_VERSION ?= v0.15.0
ENVTEST_VERSION ?= release-0.18
GOLANGCI_LINT_VERSION ?= v1.57.2
OPERATOR_SDK_VERSION ?= v1.34.2

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK) ## Download operator-sdk locally if necessary.
$(OPERATOR_SDK): $(LOCALBIN)
	$(call go-install-tool,$(OPERATOR_SDK),github.com/operator-framework/operator-sdk/cmd/operator-sdk,$(OPERATOR_SDK_VERSION))

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
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

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
