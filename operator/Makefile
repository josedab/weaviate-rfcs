# Weaviate Operator Makefile

# Image URL to use all building/pushing image targets
IMG ?= weaviate/weaviate-operator:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/manager cmd/manager/main.go

.PHONY: run
run: fmt vet ## Run operator from your host.
	go run ./cmd/manager/main.go

.PHONY: docker-build
docker-build: ## Build docker image.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image.
	docker push ${IMG}

##@ Deployment

.PHONY: install
install: ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/crd/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/crd/

.PHONY: deploy
deploy: ## Deploy operator to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/manager/namespace.yaml
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/deployment.yaml

.PHONY: undeploy
undeploy: ## Undeploy operator from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f config/manager/deployment.yaml
	kubectl delete -f config/rbac/
	kubectl delete -f config/manager/namespace.yaml

##@ Helm

.PHONY: helm-install
helm-install: ## Install operator using Helm.
	helm install weaviate-operator helm/weaviate-operator \
		--namespace weaviate-operator-system \
		--create-namespace

.PHONY: helm-uninstall
helm-uninstall: ## Uninstall operator using Helm.
	helm uninstall weaviate-operator \
		--namespace weaviate-operator-system

.PHONY: helm-upgrade
helm-upgrade: ## Upgrade operator using Helm.
	helm upgrade weaviate-operator helm/weaviate-operator \
		--namespace weaviate-operator-system

##@ Examples

.PHONY: deploy-example
deploy-example: ## Deploy example WeaviateCluster.
	kubectl apply -f config/samples/weaviate_v1_basic.yaml

.PHONY: delete-example
delete-example: ## Delete example WeaviateCluster.
	kubectl delete -f config/samples/weaviate_v1_basic.yaml
