# Image URL to use all building/pushing image targets
REPO	?= rzetelskik/scylla-operator-autoscaler
TAG		?= $(shell git describe --tags --always --abbrev=0)
IMG		?= $(REPO):latest

.EXPORT_ALL_VARIABLES:
DOCKER_BUILDKIT		:= 1
GOVERSION			:= $(shell go version)
GOPATH				:= $(shell go env GOPATH)
KUBEBUILDER_ASSETS	:= $(GOPATH)/bin
PATH				:= $(GOPATH)/bin:$(PATH):

# Run against the configured Kubernetes cluster in ~/.kube/config
run: fmt vet
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
docker-build: fmt vet
	export IMG='$(IMG)'
	envsubst < .goreleaser.yaml > .subst.goreleaser.yaml
	goreleaser -f .subst.goreleaser.yaml --skip-validate --skip-publish --rm-dist --snapshot
	rm -f .subst.goreleaser.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
