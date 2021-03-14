package updater

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Updater interface {
	RunOnce(ctx context.Context) error
}

type updater struct {
	client client.Client
	logger log.Logger
}

func NewUpdater(c client.Client, logger log.Logger) Updater {
	return &updater{
		client: c,
		logger: logger,
	}
}

func getDataCenterRecommendations(sca *v1alpha1.ScyllaClusterAutoscaler) []v1alpha1.DataCenterRecommendations {
	if sca.Status.Recommendations == nil {
		return nil
	}

	return sca.Status.Recommendations.DataCenterRecommendations
}

func getRackRecommendations(dataCenterName string, dcRecs []v1alpha1.DataCenterRecommendations) []v1alpha1.RackRecommendations {
	for idx := range dcRecs {
		if dcRecs[idx].Name == dataCenterName {
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

func filterModeOffScas(scas *v1alpha1.ScyllaClusterAutoscalerList) []v1alpha1.ScyllaClusterAutoscaler {
	filteredScas := make([]v1alpha1.ScyllaClusterAutoscaler, 0)
	for _, sca := range scas.Items {
		if sca.Spec.UpdatePolicy != nil && sca.Spec.UpdatePolicy.UpdateMode != nil &&
			*sca.Spec.UpdatePolicy.UpdateMode == v1alpha1.UpdateModeAuto {
			filteredScas = append(filteredScas, sca)
		}
	}

	return filteredScas
}

func (u *updater) RunOnce(ctx context.Context) error {
	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	if err := u.client.List(ctx, scas); err != nil {
		return err
	}

	filteredScas := filterModeOffScas(scas)
	for idx := range filteredScas {
		sca := &filteredScas[idx]

		dcRecs := getDataCenterRecommendations(sca)
		if dcRecs == nil {
			u.logger.Debug(ctx, "no data center recommendations for cluster", "cluster", sca.ClusterName)
			continue
		}

		targetRef := sca.Spec.TargetRef
		cluster := &scyllav1.ScyllaCluster{}
		err := u.client.Get(ctx, client.ObjectKey{
			Namespace: targetRef.Namespace,
			Name:      targetRef.Name,
		}, cluster)
		if err != nil {
			return err
		}

		dataCenterName := cluster.Spec.Datacenter.Name
		rackRecs := getRackRecommendations(dataCenterName, dcRecs)
		if rackRecs == nil {
			u.logger.Debug(ctx, "no rack recommendations for data center", "data center", dataCenterName)
			continue
		}

		rackStatuses := cluster.Status.Racks
		anyRackUpdated := false
		for j := range rackRecs {
			rackRec := &rackRecs[j]

			if rackRec.Members == nil {
				u.logger.Debug(ctx, "no members recommendation for rack", "rack", rackRec.Name)
				continue
			}

			rack := findRack(rackRec.Name, cluster.Spec.Datacenter.Racks)
			if rack == nil {
				u.logger.Debug(ctx, "rack specified in recommendation was not found in cluster's data center",
					"rack", rackRec.Name, "cluster", cluster.Name, "data center", cluster.Spec.Datacenter.Name)
				continue
			}

			rackStatus := rackStatuses[rack.Name]
			requestedMembers := rackStatus.Members
			readyMembers := rackStatus.ReadyMembers

			if requestedMembers != readyMembers {
				u.logger.Debug(ctx, "not applying recommendation: rack's requested members aren't equal to ready",
					"rack", rack.Name, "requested members", requestedMembers, "ready members", readyMembers)
				continue
			} else if rack.Members == rackRec.Members.Target {
				u.logger.Debug(ctx, "not applying recommendation: rack's spec members are equal to target",
					"rack", rack.Name, "requested members", requestedMembers,
					"target members", rackRec.Members.Target)
				continue
			}

			rack.Members = rackRec.Members.Target
			anyRackUpdated = true
			u.logger.Info(ctx, "members recommendation for rack applied",
				"rack", rackRec.Name, "members", rack.Members)
		}

		if anyRackUpdated {
			if err = u.client.Update(ctx, cluster); err != nil {
				return err
			}

			u.logger.Info(ctx, "cluster updated", "cluster", cluster.Name)
		}
	}

	return nil
}
