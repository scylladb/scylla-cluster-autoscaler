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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewCluster() *v1.ScyllaCluster {
	return &v1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-ns",
		},
		Spec: v1.ClusterSpec{
			Repository: util.RefFromString("repo"),
			Version:    "2.3.1",
			Datacenter: v1.DatacenterSpec{
				Name: "test-dc",
				Racks: []v1.RackSpec{
					{
						Name:    "test-rack-1",
						Members: 3,
						Storage: v1.StorageSpec{
							Capacity: "5Gi",
						},
					},
				},
			},
		},
		Status: v1.ClusterStatus{
			Racks: map[string]v1.RackStatus{
				"test-rack-1": {
					Version:      "2.3.1",
					Members:      3,
					ReadyMembers: 3,
				},
			},
		},
	}
}

func NewScyllaAutoscalerList() *v1alpha1.ScyllaClusterAutoscalerList {
	autoUpdateMode := v1alpha1.UpdateModeAuto
	return &v1alpha1.ScyllaClusterAutoscalerList{
		Items: []v1alpha1.ScyllaClusterAutoscaler{
			{
				Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
					TargetRef: &v1alpha1.TargetRef{
						Name:      "test-cluster",
						Namespace: "test-ns",
					},
					UpdatePolicy: &v1alpha1.UpdatePolicy{
						UpdateMode: &autoUpdateMode,
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

	cluster := NewCluster()
	oldCluster := NewCluster()
	scas := NewScyllaAutoscalerList()

	err := validateClusterChanges(ctx, logger, cluster, oldCluster, scas)
	require.NoError(t, err, "validateClusterChanges on unchanged cluster should not throw any errors", err)
}
