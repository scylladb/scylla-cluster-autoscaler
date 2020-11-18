# Image URL to use all building/pushing image targets
REPO	?= rzetelskik/scylla-operator-autoscaler
#TAG		?= $(shell git describe --tags --always --abbrev=0)
IMG		?= $(REPO):latest

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT		:= 1
KUBEBUILDER_ASSETS	:= $(CURDIR)/bin/deps
PATH				:= $(CURDIR)/bin/deps:$(PATH):
PATH				:= $(CURDIR)/bin/deps/go/bin:$(PATH):
GOROOT				:= $(CURDIR)/bin/deps/go
GOVERSION			:= $(shell go version)

# Run against the configured Kubernetes cluster in ~/.kube/config
run: bin/deps fmt vet
	go run ./pkg/cmd operator-autoscaler

# Deploy operator_autoscaler in the configured Kubernetes cluster in ~/.kube/config
deploy:
	cd config/operator_autoscaler && kustomize edit set image operator-autoscaler=${IMG}
	kustomize build config/default | kubectl apply -f -

# Run go fmt against code
fmt:
	go fmt ./pkg/...

# Run go vet against code
vet:
	go vet ./pkg/...

# Build the docker image
docker-build:
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

release: bin/deps
	goreleaser --rm-dist --snapshot

bin/deps: hack/binary_deps.py
	mkdir -p bin/deps
	hack/binary_deps.py bin/deps