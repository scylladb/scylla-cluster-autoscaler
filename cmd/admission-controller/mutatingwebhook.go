package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// recommendationApplier overwrites ScyllaCluster spec with recomendations given by Recommender (if available)
type recommendationApplier struct {
	Client  client.Client
	decoder *admission.Decoder

	logger log.Logger
}

func getDataCenterRecommendations(sca *v1alpha1.ScyllaClusterAutoscaler) []v1alpha1.DataCenterRecommendations {
	if sca.Status.Recommendations == nil {
		return nil
	}
	dcRecs := sca.Status.Recommendations.DataCenterRecommendations
	if dcRecs == nil || len(dcRecs) == 0 {
		return nil
	}
	return dcRecs
}

func getRackRecommendations(dataCenterName string,
	dcRecs []v1alpha1.DataCenterRecommendations) []v1alpha1.RackRecommendations {
	for idx := range dcRecs {
		if dcRecs[idx].Name == dataCenterName {
			if dcRecs[idx].RackRecommendations == nil {
				return nil
			}
			return dcRecs[idx].RackRecommendations
		}
	}
	return nil
}

func findRack(rackName string, racks []scyllav1.RackSpec) *scyllav1.RackSpec {
	for idx := range racks {
		if rackName == racks[idx].Name {
			return &racks[idx]
		}
	}
	return nil
}

func mutateCluster(ctx context.Context, logger log.Logger, cluster *scyllav1.ScyllaCluster) error {
	logger.Info(ctx, "Starting mutation of ScyllaCluster")

	c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("Failed to create a client: %s", err)
	}

	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	err = c.List(ctx, scas)
	if err != nil {
		return fmt.Errorf("Failed to get SCAs: %s", err)
	}

	logger.Debug(ctx, "SCAs fetched", "num", len(scas.Items))

	for idx := range scas.Items {
		sca := &scas.Items[idx]

		if *sca.Spec.UpdatePolicy.UpdateMode == v1alpha1.UpdateModeOff {
			logger.Debug(ctx, "Autoscaler has 'off' scaling policy, skipping", "autoscaler", sca.ObjectMeta.Name)
			continue
		}

		logger.Debug(ctx, "Cluster has 'Initial' or 'Auto' scaling policy")

		dcRecs := getDataCenterRecommendations(sca)
		if dcRecs == nil {
			logger.Debug(ctx, "No recommendations for cluster", "cluster", cluster.Name)
			continue
		}

		dataCenterName := cluster.Spec.Datacenter.Name

		if cluster.Spec.Datacenter.Name != dataCenterName {
			logger.Debug(ctx, "Data center name does not match")
			continue
		}

		logger.Info(ctx, "Found data center with name", "data center", dataCenterName)

		rackRecs := getRackRecommendations(dataCenterName, dcRecs)
		if rackRecs == nil {
			logger.Debug(ctx, "No recommendations for data center", "data center", dataCenterName)
			continue
		}

		for j := range rackRecs {
			rackRec := &rackRecs[j]

			if rackRec.Members == nil {
				logger.Debug(ctx, "No members recommendation for rack", "rack", rackRec.Name)
				continue
			}

			rack := findRack(rackRec.Name, cluster.Spec.Datacenter.Racks)
			if rack == nil {
				logger.Debug(ctx, "Could not find rack matching recommendation", "rack", rackRec.Name)
				continue
			}

			if rackRec.Name != rack.Name {
				logger.Debug(ctx, "Rack name does not match")
				continue
			}

			rack.Members = rackRec.Members.Target

			logger.Info(ctx, "Rack updated", "rack", rackRec.Name)
		}
	}

	return nil
}

func (ra *recommendationApplier) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &scyllav1.ScyllaCluster{}

	err := ra.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = mutateCluster(ctx, ra.logger, cluster)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

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
