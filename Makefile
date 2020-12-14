# Image URL to use all building/pushing image targets
REPO        ?= rzetelskik/scylla-operator-autoscaler
TAG		    ?= $(shell git describe --tags --always --abbrev=0)
IMG		    ?= $(REPO):latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT         := 1
GOVERSION               := $(shell go version)
GOPATH                  := $(shell go env GOPATH)
KUBEBUILDER_ASSETS      := $(GOPATH)/bin
PATH                    := $(GOPATH)/bin:$(PATH):

.PHONY: default
default: docker-build docker-push deploy

# Run tests
test: fmt vet
	go test ./pkg/... -coverprofile cover.out

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
	go run ./pkg/cmd operator-autoscaler

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy operator_autoscaler in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/operator_autoscaler && kustomize edit set image operator-autoscaler=${IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./pkg/...

# Run go vet against code
vet:
	go vet ./pkg/...

# Build the docker image
docker-build: fmt vet
	export IMG='$(IMG)'
	envsubst < .goreleaser.yaml > .subst.goreleaser.yaml
	goreleaser -f .subst.goreleaser.yaml --skip-validate --skip-publish --rm-dist --snapshot
	rm -f .subst.goreleaser.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# Generate code
generate:
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="$(PKG)"
