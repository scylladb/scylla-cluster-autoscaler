package main

import (
	"context"
	"testing"

	"github.com/scylladb/scylla-operator-autoscaler/pkg/test/unit"
	v1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/scylladb/go-log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
)

func TestValidateClusterChanges(t *testing.T) {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{Level: atom})
	autoUpdateMode := v1alpha1.UpdateModeAuto
	offUpdateMode := v1alpha1.UpdateModeOff

	basicCluster := unit.NewDoubleRackCluster("test-cluster", "test-cluster-ns", "repo", "2.3.1", "test-dc",
		unit.RackInfo{
			Name:     "rack-1",
			Members:  3,
			Capacity: "5Gi",
			CPU:      "1",
			Memory:   "500M",
		},
		unit.RackInfo{
			Name:     "rack-2",
			Members:  2,
			Capacity: "3Gi",
			CPU:      "0.5",
			Memory:   "200M",
		},
	)
	oldBasicCluster := basicCluster.DeepCopy()

	changedClusterName := basicCluster.DeepCopy()
	changedClusterName.Name = "other-cluster"

	changedMembers := basicCluster.DeepCopy()
	changedMembers.Spec.Datacenter.Racks[0].Members = 1

	changedMembersSecondRack := basicCluster.DeepCopy()
	changedMembersSecondRack.Spec.Datacenter.Racks[1].Members = 10

	changedCapacity := basicCluster.DeepCopy()
	changedCapacity.Spec.Datacenter.Racks[0].Storage.Capacity = "1Gi"

	changedCPU := basicCluster.DeepCopy()
	changedCPU.Spec.Datacenter.Racks[0].Resources.Requests = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("0.5"),
		corev1.ResourceMemory: resource.MustParse("5Gi"),
	}

	changedMemory := basicCluster.DeepCopy()
	changedMemory.Spec.Datacenter.Racks[0].Resources.Requests = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1"),
		corev1.ResourceMemory: resource.MustParse("1Gi"),
	}

	basicScas := unit.NewDoubleScyllaAutoscalerList("test-cluster", "test-cluster-ns", "other-cluster", "test-cluster-ns", autoUpdateMode, autoUpdateMode)
	offScas := unit.NewDoubleScyllaAutoscalerList("test-cluster", "test-cluster-ns", "other-cluster", "test-cluster-ns", offUpdateMode, offUpdateMode)

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
		{
			name:       "changed cluster name",
			cluster:    changedClusterName,
			oldCluster: oldBasicCluster,
			scas:       basicScas,
			allowed:    true,
		},
		{
			name:       "changed members in first rack",
			cluster:    changedMembers,
			oldCluster: oldBasicCluster,
			scas:       basicScas,
			allowed:    false,
		},
		{
			name:       "changed members in second rack",
			cluster:    changedMembersSecondRack,
			oldCluster: oldBasicCluster,
			scas:       basicScas,
			allowed:    false,
		},
		{
			name:       "changed cpu in first rack",
			cluster:    changedCPU,
			oldCluster: oldBasicCluster,
			scas:       basicScas,
			allowed:    false,
		},
		{
			name:       "SCA in 'Off' mode",
			cluster:    changedMembers,
			oldCluster: oldBasicCluster,
			scas:       offScas,
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
