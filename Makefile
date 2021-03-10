REPO        		?= scyllazimnx
TAG		    		?= $(shell git describe --tags --always --abbrev=0)
IMG_PREFIX		    ?= scylla-operator-autoscaler-

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS         ?= "crd:trivialVersions=true,allowDangerousTypes=true"

.EXPORT_ALL_VARIABLES:
GOVERSION               		:= $(shell go version)
GOPATH                  		:= $(shell go env GOPATH)
KUBEBUILDER_ASSETS      		:= $(GOPATH)/bin
PATH                    		:= $(GOPATH)/bin:$(PATH):
RECOMMENDER_IMG         		:= $(REPO)/$(IMG_PREFIX)recommender
UPDATER_IMG       	    		:= $(REPO)/$(IMG_PREFIX)updater
ADMISSION_CONTROLLER_IMG  	    := $(REPO)/$(IMG_PREFIX)admission-controller


.PHONY: default
default: docker-build docker-push deploy

# Run tests
test: fmt vet
	go test ./... -coverprofile cover.out

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy operator_autoscaler in the configured Kubernetes cluster in ~/.kube/config
deploy:
	cd config/recommender && kustomize edit set image recommender=$(RECOMMENDER_IMG)
	cd config/updater && kustomize edit set image updater=$(UPDATER_IMG)
	cd config/admission-controller && kustomize edit set image admission-controller=$(ADMISSION_CONTROLLER_IMG)
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	controller-gen $(CRD_OPTIONS) webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the docker image
docker-build: fmt vet
	envsubst < .goreleaser.yml > .subst.goreleaser.yml
	goreleaser -f .subst.goreleaser.yml --skip-validate --skip-publish --rm-dist --snapshot
	rm -f .subst.goreleaser.yml

# Push the docker image
docker-push:
	docker push ${RECOMMENDER_IMG}
	docker push ${UPDATER_IMG}
	docker push ${ADMISSION_CONTROLLER_IMG}
