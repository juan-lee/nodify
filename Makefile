SHELL := /bin/bash
# Image URL to use all building/pushing image targets
TAG ?= dev
REGISTRY ?= juanlee
CONTROLLER_IMG ?= nodify-controller
DAEMON_IMG ?= nodify-daemon
CONTROLLER_URI ?= $(REGISTRY)/$(CONTROLLER_IMG):$(TAG)
DAEMON_URI ?= $(REGISTRY)/$(DAEMON_IMG):$(TAG)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager daemon

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate fmt lint manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out
	cd daemon && go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt lint
	go build -o bin/manager main.go

# Build daemon binary
daemon: generate fmt lint
	cd daemon && go build -o ../bin/daemon main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt lint manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

set-images: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_URI}
	cd config/daemon && $(KUSTOMIZE) edit set image daemon=${DAEMON_URI}

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: set-images
	$(KUSTOMIZE) build config/default | kubectl apply -f -
	kubectl rollout -n nodify-system restart deploy/nodify-controller-manager
	kubectl rollout -n nodify-system restart daemonset/nodify-nodify

# UnDeploy controller from the configured Kubernetes cluster in ~/.kube/config
undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cd daemon && $(CONTROLLER_GEN) paths=. rbac:roleName=daemon-role output:rbac:dir=../config/daemon

# Run go fmt against code
fmt:
	go fmt ./...

# Run go golangci-lint against code
lint: golangci-lint
	$(GOLANGCI_LINT) run -v --fast=false --timeout=5m
	cd daemon && $(GOLANGCI_LINT) run -v --fast=false --timeout=5m

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build -t ${CONTROLLER_URI} .
	docker build -t ${DAEMON_URI} -f Dockerfile.daemon .

# Push the docker image
docker-push:
	docker push ${CONTROLLER_URI}
	docker push ${DAEMON_URI}

# Release
release: set-images
	mkdir -p dist
	$(KUSTOMIZE) build config/default > bin/nodify.yaml

# Download controller-gen locally if necessary
CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen:
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

# Download kustomize locally if necessary
KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize:
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

# Download golangci-lint locally if necessary
GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
golangci-lint:
	$(call go-get-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint@v1.37.1)

# Download goreleaser locally if necessary
GORELEASER = $(shell pwd)/bin/goreleaser
goreleaser:
	$(call go-get-tool,$(GORELEASER),github.com/goreleaser/goreleaser@v0.160.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
