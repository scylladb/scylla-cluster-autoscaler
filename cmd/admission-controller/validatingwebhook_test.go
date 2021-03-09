package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/test/unit"
	v1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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
			name:       "changed capacity in first rack",
			cluster:    changedCapacity,
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
			name:       "changed memory in first rack",
			cluster:    changedMemory,
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

func TestHandle(t *testing.T) {
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{Level: atom})

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

	offUpdateMode := v1alpha1.UpdateModeOff
	modeOffSca := v1alpha1.ScyllaClusterAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sca",
			Namespace: "test-cluster-ns",
		},
		Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
			TargetRef: &v1alpha1.TargetRef{
				Name:      "test-cluster",
				Namespace: "test-cluster-ns",
			},
			UpdatePolicy: &v1alpha1.UpdatePolicy{
				UpdateMode: &offUpdateMode,
			},
		},
		Status: v1alpha1.ScyllaClusterAutoscalerStatus{
			Recommendations: &v1alpha1.ScyllaClusterRecommendations{
				DataCenterRecommendations: []v1alpha1.DataCenterRecommendations{
					{
						Name: "test-dc",
						RackRecommendations: []v1alpha1.RackRecommendations{
							{Name: "rack-1", Members: &v1alpha1.RecommendedRackMembers{Target: 2}},
						},
					},
				},
			},
		},
	}

	c := fake.NewFakeClientWithScheme(scheme, basicCluster, modeOffSca.DeepCopy())

	av := &admissionValidator{Client: nil, logger: logger, scyllaClient: c}

	simpleRequest := admission.Request{
		AdmissionRequest: admissionv1beta1.AdmissionRequest{
			UserInfo: authenticationv1.UserInfo{
				Username: "test-user",
			},
			Object:    encodeRaw(t, basicCluster.DeepCopy()),
			OldObject: encodeRaw(t, basicCluster.DeepCopy()),
		},
	}

	tests := []struct {
		name    string
		req     admission.Request
		allowed bool
	}{
		{
			name:    "unchanged cluster",
			req:     simpleRequest,
			allowed: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := av.Handle(ctx, test.req)
			fmt.Printf("%s\n", resp.String())
		})
	}
}

// encodeRaw is a helper to encode some data into a RawExtension.
func encodeRaw(t *testing.T, input interface{}) runtime.RawExtension {
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: data}
}
