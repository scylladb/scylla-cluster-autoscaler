package admission_controller

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

	doubleRackCluster := unit.NewDoubleRackCluster("test-cluster", "test-cluster-ns", "repo", "2.3.1", "test-dc",
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

	doubleRackWithChangedClusterName := doubleRackCluster.DeepCopy()
	doubleRackWithChangedClusterName.Name = "other-cluster"

	doubleRackWithChangedMembers := doubleRackCluster.DeepCopy()
	doubleRackWithChangedMembers.Spec.Datacenter.Racks[0].Members = 1

	doubleRackWithChangedMembersInSecondRack := doubleRackCluster.DeepCopy()
	doubleRackWithChangedMembersInSecondRack.Spec.Datacenter.Racks[1].Members = 10

	doubleRackWithChangedCapacity := doubleRackCluster.DeepCopy()
	doubleRackWithChangedCapacity.Spec.Datacenter.Racks[0].Storage.Capacity = "1Gi"

	doubleRackWithChangedCPU := doubleRackCluster.DeepCopy()
	doubleRackWithChangedCPU.Spec.Datacenter.Racks[0].Resources.Requests = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("0.5"),
		corev1.ResourceMemory: resource.MustParse("500M"),
	}

	singleRackCluster := doubleRackCluster.DeepCopy()
	singleRackCluster.Spec.Datacenter.Racks = singleRackCluster.Spec.Datacenter.Racks[:len(singleRackCluster.Spec.Datacenter.Racks)-1]

	autoModeDoubleScaList := unit.NewDoubleScyllaAutoscalerList("test-cluster", "test-cluster-ns", "other-cluster", "test-cluster-ns", autoUpdateMode, autoUpdateMode)
	offModeDoubleScaList := unit.NewDoubleScyllaAutoscalerList("test-cluster", "test-cluster-ns", "other-cluster", "test-cluster-ns", offUpdateMode, offUpdateMode)

	tests := []struct {
		name            string
		cluster         *v1.ScyllaCluster
		oldCluster      *v1.ScyllaCluster
		scas            *v1alpha1.ScyllaClusterAutoscalerList
		scaledResources []string
		allowed         bool
	}{
		{
			name:            "allow empty update",
			cluster:         doubleRackCluster,
			oldCluster:      doubleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         true,
		},
		{
			name:            "allow changing cluster's name",
			cluster:         doubleRackWithChangedClusterName,
			oldCluster:      doubleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         true,
		},
		{
			name:            "deny changing member count",
			cluster:         doubleRackWithChangedMembers,
			oldCluster:      doubleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         false,
		},
		{
			name:            "deny changing member count in second rack",
			cluster:         doubleRackWithChangedMembersInSecondRack,
			oldCluster:      doubleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         false,
		},
		{
			name:            "deny changing CPU resources when CPU is scaled",
			cluster:         doubleRackWithChangedCPU,
			oldCluster:      doubleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         false,
		},
		{
			name:            "allow changing CPU resources when only Memory is scaled",
			cluster:         doubleRackWithChangedCPU,
			oldCluster:      doubleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"memory"},
			allowed:         true,
		},
		{
			name:            "allow changing member count while SCA is in 'OFF' mode",
			cluster:         doubleRackWithChangedMembers,
			oldCluster:      doubleRackCluster,
			scas:            offModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         true,
		},
		{
			name:            "allow adding new rack to cluster",
			cluster:         doubleRackCluster,
			oldCluster:      singleRackCluster,
			scas:            autoModeDoubleScaList,
			scaledResources: []string{"cpu"},
			allowed:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateClusterChanges(ctx, logger, test.cluster, test.oldCluster, test.scas, test.scaledResources)
			if test.allowed {
				require.NoError(t, err, "Wrong value returned from validateClusterChanges function. Message: '%s'", err)
			} else {
				require.Error(t, err, "Wrong value returned from validateClusterChanges function. Message: '%s'", err)
			}
		})
	}
}
