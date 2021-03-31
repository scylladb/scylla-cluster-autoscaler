package updater

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
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

func (u *updater) RunOnce(ctx context.Context) error {
	scas := &v1alpha1.ScyllaClusterAutoscalerList{}
	if err := u.client.List(ctx, scas); err != nil {
		return err
	}

	filteredSCAs := filterSCAs(scas)
	for idx := range filteredSCAs {
		sca := &filteredSCAs[idx]
		if recommendationExpired(sca) {
			u.logger.Info(ctx, "skipping update: sca's recommendation expired",
				"sca", sca.Name, "namespace", sca.Namespace)
			continue
		}
		if !updateCooldownExceeded(sca) {
			u.logger.Info(ctx, "skipping update: update cooldown not exceeded",
				"sca", sca.Name, "namespace", sca.Namespace)
			continue
		}

		cluster, err := u.fetchScyllaCluster(ctx, sca.Spec.TargetRef.Name, sca.Spec.TargetRef.Namespace)
		if err != nil {
			return err
		}
		if equalChecksums, err := equalChecksums(cluster, sca); err != nil {
			return err
		} else if equalChecksums {
			u.logger.Info(ctx, "skipping update: latest applied recommendation's checksum is equal to current's",
				"sca", sca.Name, "namespace", sca.Namespace)
			continue
		}
		if !isScyllaClusterReady(cluster) {
			u.logger.Info(ctx, "skipping update: scylla cluster isn't ready",
				"sca", sca.Name, "namespace", sca.Namespace)
			continue
		}

		dcRecs := getDatacenterRecommendations(sca)
		if dcRecs == nil {
			u.logger.Debug(ctx, "no data center recommendations for cluster", "cluster", sca.ClusterName)
			continue
		}

		dataCenterName := cluster.Spec.Datacenter.Name
		rackRecs := getRackRecommendations(dataCenterName, dcRecs)
		if rackRecs == nil {
			u.logger.Debug(ctx, "no rack recommendations for data center", "data center", dataCenterName)
			continue
		}

		for j := range rackRecs {
			rackRec := &rackRecs[j]
			rack := findRack(rackRec.Name, cluster.Spec.Datacenter.Racks)
			if rack == nil {
				u.logger.Debug(ctx, "rack specified in recommendation was not found in cluster's data center",
					"rack", rackRec.Name, "cluster", cluster.Name, "data center", cluster.Spec.Datacenter.Name)
				continue
			}

			applyRackRec(rack, rackRec)
		}

		if err = u.updateScyllaCluster(ctx, cluster, sca.Status.Recommendations); err != nil {
			return err
		}
		if err = u.updateSCAStatus(ctx, sca); err != nil {
			return err
		}
	}

	return nil
}

func filterSCAs(scas *v1alpha1.ScyllaClusterAutoscalerList) []v1alpha1.ScyllaClusterAutoscaler {
	filteredSCAs := make([]v1alpha1.ScyllaClusterAutoscaler, 0)
	for _, sca := range scas.Items {
		if sca.Spec.UpdatePolicy != nil && sca.Spec.UpdatePolicy.UpdateMode == v1alpha1.UpdateModeAuto &&
			sca.Status.UpdateStatus != nil && *sca.Status.UpdateStatus == v1alpha1.UpdateStatusOk {
			filteredSCAs = append(filteredSCAs, sca)
		}
	}

	return filteredSCAs
}

func recommendationExpired(sca *v1alpha1.ScyllaClusterAutoscaler) bool {
	recExpTime := sca.Spec.UpdatePolicy.RecommendationExpirationTime
	return !sca.Status.LastUpdated.IsZero() && recExpTime != nil &&
		time.Now().Sub(sca.Status.LastUpdated.Time) > recExpTime.Duration
}

func updateCooldownExceeded(sca *v1alpha1.ScyllaClusterAutoscaler) bool {
	updateCooldown := sca.Spec.UpdatePolicy.UpdateCooldown
	return sca.Status.LastApplied.IsZero() || updateCooldown == nil ||
		time.Now().Sub(sca.Status.LastApplied.Time) >= updateCooldown.Duration
}

func (u *updater) fetchScyllaCluster(ctx context.Context, name, namespace string) (*scyllav1.ScyllaCluster, error) {
	cluster := &scyllav1.ScyllaCluster{}
	if err := u.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

func equalChecksums(cluster *scyllav1.ScyllaCluster, sca *v1alpha1.ScyllaClusterAutoscaler) (bool, error) {
	if labels := cluster.ObjectMeta.Labels; labels != nil && sca.Status.Recommendations != nil {
		if latestChecksum, ok := labels["sca-latest-checksum"]; ok {
			if newChecksum, err := util.NewChecksum(*sca.Status.Recommendations); err != nil {
				return false, err
			} else {
				return newChecksum == latestChecksum, nil
			}
		}
	}

	return false, nil
}

func isScyllaClusterReady(cluster *scyllav1.ScyllaCluster) bool {
	for _, rack := range cluster.Spec.Datacenter.Racks {
		rackStatus, found := cluster.Status.Racks[rack.Name]
		if !found || rackStatus.Members != rackStatus.ReadyMembers {
			return false
		}
	}

	return true
}

func getDatacenterRecommendations(sca *v1alpha1.ScyllaClusterAutoscaler) []v1alpha1.DatacenterRecommendations {
	if sca.Status.Recommendations == nil {
		return nil
	}

	return sca.Status.Recommendations.DatacenterRecommendations
}

func getRackRecommendations(dataCenterName string, dcRecs []v1alpha1.DatacenterRecommendations) []v1alpha1.RackRecommendations {
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

func applyRackRec(rack *scyllav1.RackSpec, rackRec *v1alpha1.RackRecommendations) {
	if rackRec.Members != nil {
		rack.Members = *rackRec.Members
	}
	if rackRec.Resources != nil {
		if CpuLimitRec, ok := rackRec.Resources.Limits[corev1.ResourceCPU]; ok {
			rack.Resources.Limits[corev1.ResourceCPU] = CpuLimitRec
		}
		if CpuRequestRec, ok := rackRec.Resources.Requests[corev1.ResourceCPU]; ok {
			rack.Resources.Requests[corev1.ResourceCPU] = CpuRequestRec
		}
	}
}

func (u *updater) updateScyllaCluster(ctx context.Context, cluster *scyllav1.ScyllaCluster,
	recs *v1alpha1.ScyllaClusterRecommendations) error {
	newChecksum, err := util.NewChecksum(*recs)
	if err != nil {
		return err
	}
	if cluster.ObjectMeta.Labels == nil {
		cluster.ObjectMeta.Labels = map[string]string{"sca-latest-checksum": newChecksum}
	} else {
		cluster.ObjectMeta.Labels["sca-latest-checksum"] = newChecksum
	}

	if err = u.client.Update(ctx, cluster); err != nil {
		return err
	}
	u.logger.Info(ctx, "cluster updated", "cluster", cluster.Name)
	return nil
}

func (u *updater) updateSCAStatus(ctx context.Context, sca *v1alpha1.ScyllaClusterAutoscaler) error {
	sca.Status.LastApplied = metav1.NewTime(time.Now().UTC())
	if err := u.client.Status().Update(ctx, sca); err != nil {
		return err
	}
	u.logger.Info(ctx, "sca updated", "sca", sca.Name)
	return nil
}
