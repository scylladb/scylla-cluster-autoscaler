package main

import (
	"context"
	"testing"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	v1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/scylladb/scylla-operator/pkg/controllers/cluster/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rackInfo struct {
	name     string
	members  int32
	capacity string
	cpu      string
	memory   string
}

func NewDoubleRackCluster(clusterName, clusterNamespace, repo, version, dcName string, firstRack, secondRack rackInfo) *v1.ScyllaCluster {
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
						Name:    firstRack.name,
						Members: firstRack.members,
						Storage: v1.StorageSpec{
							Capacity: firstRack.capacity,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(firstRack.cpu),
								corev1.ResourceMemory: resource.MustParse(firstRack.memory),
							},
						},
					},
					{
						Name:    secondRack.name,
						Members: secondRack.members,
						Storage: v1.StorageSpec{
							Capacity: secondRack.capacity,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse(secondRack.cpu),
								corev1.ResourceMemory: resource.MustParse(secondRack.memory),
							},
						},
					},
				},
			},
		},
		Status: v1.ClusterStatus{ // todo maybe not include status in tests
			Racks: map[string]v1.RackStatus{
				firstRack.name: {
					Version:      version,
					Members:      firstRack.members,
					ReadyMembers: firstRack.members,
				},
				secondRack.name: {
					Version:      version,
					Members:      secondRack.members,
					ReadyMembers: secondRack.members,
				},
			},
		},
	}
}

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

func TestValidateClusterChanges(t *testing.T) {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{Level: atom})

	basicCluster := NewDoubleRackCluster("test-cluster", "test-clutser-ns", "repo", "2.3.1", "test-dc",
		rackInfo{
			name:     "rack-1",
			members:  3,
			capacity: "5Gi",
			cpu:      "1",
			memory:   "500M",
		},
		rackInfo{
			name:     "rack-2",
			members:  2,
			capacity: "3Gi",
			cpu:      "0.5",
			memory:   "200M",
		},
	)
	oldBasicCluster := basicCluster.DeepCopy()

	autoUpdateMode := v1alpha1.UpdateModeAuto
	offUpdateMode := v1alpha1.UpdateModeOff

	basicScas := NewDoubleScyllaAutoscalerList("test-cluster", "test-cluster-ns", "other-cluster", "test-cluster-ns", autoUpdateMode, offUpdateMode)

	tests := []struct {
		name       string
		cluster    *v1.ScyllaCluster
		oldCluster *v1.ScyllaCluster
		scas       *v1alpha1.ScyllaClusterAutoscalerList
		allowed    bool
	}{
		{
			name:       "unchanged cluster",
			cluster:    basicCluster,
			oldCluster: oldBasicCluster,
			scas:       basicScas,
			allowed:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateClusterChanges(ctx, logger, test.cluster, test.oldCluster, test.scas)
			if test.allowed {
				require.NoError(t, err, "Wrong value returned from validateClusterChanges function. Message: '%s'", err)
			} else {
				require.Error(t, err, "Wrong value returned from validateClusterChanges function. Message: '%s'", err)
			}
		})
	}
}
