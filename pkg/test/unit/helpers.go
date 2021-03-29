package unit

import (
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	v1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RackInfo - basic info about rack (values that are administered by autoscaler)
type RackInfo struct {
	Name     string
	Members  int32
	Capacity string
	CPU      string
	Memory   string
}

// NewDoubleRackCluster returns cluster with two racks described by given parameters
func NewDoubleRackCluster(clusterName, clusterNamespace, repo, version, dcName string, firstRack, secondRack RackInfo) *v1.ScyllaCluster {
	return &v1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterNamespace,
		},
		Spec: v1.ClusterSpec{
			Repository: util.RefFromString(repo),
			Version:    version,
			Datacenter: v1.DatacenterSpec{
				Name: dcName,
				Racks: []v1.RackSpec{
					{
						Name:    firstRack.Name,
						Members: firstRack.Members,
						Storage: v1.StorageSpec{
							Capacity: firstRack.Capacity,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(firstRack.CPU),
								corev1.ResourceMemory: resource.MustParse(firstRack.Memory),
							},
						},
					},
					{
						Name:    secondRack.Name,
						Members: secondRack.Members,
						Storage: v1.StorageSpec{
							Capacity: secondRack.Capacity,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(secondRack.CPU),
								corev1.ResourceMemory: resource.MustParse(secondRack.Memory),
							},
						},
					},
				},
			},
		},
		Status: v1.ClusterStatus{
			Racks: map[string]v1.RackStatus{
				firstRack.Name: {
					Version:      version,
					Members:      firstRack.Members,
					ReadyMembers: firstRack.Members,
				},
				secondRack.Name: {
					Version:      version,
					Members:      secondRack.Members,
					ReadyMembers: secondRack.Members,
				},
			},
		},
	}
}

// NewDoubleScyllaAutoscalerList returns list with two autoscalers described by given parameters
func NewDoubleScyllaAutoscalerList(firstName, firstNamespace, secondName, secondNamespace string, firstMode, secondMode v1alpha1.UpdateMode) *v1alpha1.ScyllaClusterAutoscalerList {
	return &v1alpha1.ScyllaClusterAutoscalerList{
		Items: []v1alpha1.ScyllaClusterAutoscaler{
			{
				Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
					TargetRef: &v1alpha1.TargetRef{
						Name:      firstName,
						Namespace: firstNamespace,
					},
					UpdatePolicy: &v1alpha1.UpdatePolicy{
						UpdateMode: &firstMode,
					},
				},
			},
			{
				Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
					TargetRef: &v1alpha1.TargetRef{
						Name:      secondName,
						Namespace: secondNamespace,
					},
					UpdatePolicy: &v1alpha1.UpdatePolicy{
						UpdateMode: &secondMode,
					},
				},
			},
		},
	}
}
