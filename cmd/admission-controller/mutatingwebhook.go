package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/scylladb/go-log"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// recommendationApplier overwrites ScyllaCluster spec with recomendations given by Recommender (if available)
type recommendationApplier struct {
	Client  client.Client
	decoder *admission.Decoder

	logger log.Logger
}

func (ra *recommendationApplier) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &scyllav1.ScyllaCluster{}

	err := ra.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	ra.logger.Info(ctx, "mutating")
	// mutate ScyllaCluster
	cluster.Spec.Datacenter.Name = "mutated-name"

	marshaledCluster, err := json.Marshal(cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledCluster)
}

func (ra *recommendationApplier) InjectDecoder(d *admission.Decoder) error {
	ra.decoder = d
	return nil
}
