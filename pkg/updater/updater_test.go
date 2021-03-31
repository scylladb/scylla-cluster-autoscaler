package updater

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = scyllav1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

type ExpectedStateSpec struct {
	RackName  string
	Members   *int32
	Resources *corev1.ResourceRequirements
}

func TestUpdater(t *testing.T) {
	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	c := clientBuilder.Build()
	atom := zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, _ := log.NewProduction(log.Config{Level: atom})
	u := NewUpdater(c, logger)
	ctx := context.Background()

	autoUpdateMode := v1alpha1.UpdateModeAuto
	offUpdateMode := v1alpha1.UpdateModeOff
	updateStatusOk := v1alpha1.UpdateStatusOk
	basicTestClusterMeta := &metav1.ObjectMeta{
		Name:      "test-cluster",
		Namespace: "test-cluster-ns",
	}
	basicTestAutoModeScaMeta := &metav1.ObjectMeta{
		Name:      "test-auto-sca",
		Namespace: "test-sca-ns",
	}
	basicTestOffModeScaMeta := &metav1.ObjectMeta{
		Name:      "test-off-sca",
		Namespace: "test-sca-ns",
	}
	testResources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewQuantity(456, resource.DecimalSI),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewQuantity(123, resource.DecimalSI),
		},
	}
	testResourcesRecommendation := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewQuantity(789, resource.DecimalSI),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: *resource.NewQuantity(456, resource.DecimalSI),
		},
	}
	testChecksum, err := util.NewChecksum(v1alpha1.ScyllaClusterRecommendations{
		DatacenterRecommendations: []v1alpha1.DatacenterRecommendations{
			{
				Name: "test-dc",
				RackRecommendations: []v1alpha1.RackRecommendations{
					{
						Name:    "test-rack-1",
						Members: util.Int32ptr(2),
					},
				},
			},
		},
	})
	require.NoError(t, err, "Couldn't get test checksum. Message: '%s'", err)
	testLastUpdatedSCATimestamp := metav1.NewTime(time.Now().Add(time.Hour * time.Duration(-2)))
	testRecExpTime := metav1.Duration{Duration: time.Hour}
	testUpdateCooldown := metav1.Duration{Duration: time.Minute * 20}
	testLastAppliedTimestamp := metav1.NewTime(time.Now().Add(time.Minute * time.Duration(-10)))
	tests := []struct {
		Name           string
		ScyllaCluster  *scyllav1.ScyllaCluster
		Sca            *v1alpha1.ScyllaClusterAutoscaler
		ExpectedStates []ExpectedStateSpec
	}{
		{
			Name: "applied recommendation",
			ScyllaCluster: newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1, Resources: testResources},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: newSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, &updateStatusOk, basicTestClusterMeta,
				"test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: util.Int32ptr(2), Resources: &testResourcesRecommendation},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(2), Resources: &testResourcesRecommendation},
			},
		},
		{
			Name: "off mode sca",
			ScyllaCluster: newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: newSingleDcSca(basicTestOffModeScaMeta, &offUpdateMode, &updateStatusOk, basicTestClusterMeta,
				"test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: util.Int32ptr(2)},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(1)},
			},
		},
		{
			Name: "update status not ok",
			ScyllaCluster: newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: newSingleDcSca(basicTestOffModeScaMeta, &autoUpdateMode, nil, basicTestClusterMeta,
				"test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: util.Int32ptr(2)},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(1)},
			},
		},
		{
			Name: "not equal checksums",
			ScyllaCluster: setChecksumLabel(testChecksum, newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 2},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 2, ReadyMembers: 2},
				})),
			Sca: newSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, &updateStatusOk,
				basicTestClusterMeta, "test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: util.Int32ptr(1)},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(1)},
			},
		},
		{
			Name: "equal checksums",
			ScyllaCluster: setChecksumLabel(testChecksum, newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				})),
			Sca: newSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, &updateStatusOk,
				basicTestClusterMeta, "test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: util.Int32ptr(2)},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(1)},
			},
		},
		{
			Name: "recommendation expired",
			ScyllaCluster: newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: setExpTimeAndTimestamp(&testRecExpTime, testLastUpdatedSCATimestamp,
				newSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, &updateStatusOk, basicTestClusterMeta,
					"test-dc",
					[]v1alpha1.RackRecommendations{
						{Name: "test-rack-1", Members: util.Int32ptr(2)},
					})),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(1)},
			},
		},
		{
			Name: "update cooldown not exceeded",
			ScyllaCluster: newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: setCooldownAndTimestamp(&testUpdateCooldown, testLastAppliedTimestamp,
				newSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, &updateStatusOk, basicTestClusterMeta,
					"test-dc",
					[]v1alpha1.RackRecommendations{
						{Name: "test-rack-1", Members: util.Int32ptr(2)},
					})),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(1)},
			},
		},
		{
			Name: "scylla cluster not ready",
			ScyllaCluster: newSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 2},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 2, ReadyMembers: 1},
				}),
			Sca: newSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, &updateStatusOk, basicTestClusterMeta,
				"test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: util.Int32ptr(3)},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{RackName: "test-rack-1", Members: util.Int32ptr(2)},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err = c.Create(ctx, test.ScyllaCluster)
			require.NoError(t, err, "Couldn't create scylla cluster. Message: '%s'", err)
			err = c.Create(ctx, test.Sca)
			require.NoError(t, err, "Couldn't create SCA. Message: '%s'", err)

			err = u.RunOnce(ctx)
			require.NoError(t, err, "Updater RunOnce. Message: '%s'", err)
			cluster := &scyllav1.ScyllaCluster{}
			err = c.Get(ctx, client.ObjectKey{
				Namespace: test.ScyllaCluster.Namespace,
				Name:      test.ScyllaCluster.Name,
			}, cluster)
			require.NoError(t, err, "Couldn't get scylla cluster. Message: '%s'", err)

			for _, expectedState := range test.ExpectedStates {
				rack := findRack(expectedState.RackName, cluster.Spec.Datacenter.Racks)
				require.NotNil(t, rack)
				if expectedState.Members != nil {
					require.Equal(t, *expectedState.Members, rack.Members)
				}
				if expectedState.Resources != nil {
					for resourceName, expectedQuantity := range expectedState.Resources.Limits {
						rackResourceLimit, ok := rack.Resources.Limits[resourceName]
						require.True(t, ok)
						require.Equal(t, expectedQuantity.Value(), rackResourceLimit.Value())
					}

					for resourceName, expectedQuantity := range expectedState.Resources.Requests {
						rackResourceRequest, ok := rack.Resources.Requests[resourceName]
						require.True(t, ok)
						require.Equal(t, expectedQuantity.Value(), rackResourceRequest.Value())
					}
				}
			}

			err = c.Delete(ctx, test.ScyllaCluster)
			require.NoError(t, err, "Couldn't delete scylla cluster. Message: '%s'", err)
			err = c.Delete(ctx, test.Sca)
			require.NoError(t, err, "Couldn't delete SCA. Message: '%s'", err)

		})
	}
}

func newSingleDcScyllaCluster(clusterMeta *metav1.ObjectMeta, dcName string, racksSpec []scyllav1.RackSpec,
	racksStatus map[string]scyllav1.RackStatus) *scyllav1.ScyllaCluster {
	return &scyllav1.ScyllaCluster{
		ObjectMeta: *clusterMeta,
		Spec: scyllav1.ClusterSpec{
			Datacenter: scyllav1.DatacenterSpec{
				Name:  dcName,
				Racks: racksSpec,
			},
		},
		Status: scyllav1.ClusterStatus{
			Racks: racksStatus,
		},
	}
}

func newSingleDcSca(scaMeta *metav1.ObjectMeta, updateMode *v1alpha1.UpdateMode, updateStatus *v1alpha1.UpdateStatus,
	targetClusterMeta *metav1.ObjectMeta, dcName string, rackRecs []v1alpha1.RackRecommendations) *v1alpha1.ScyllaClusterAutoscaler {
	return &v1alpha1.ScyllaClusterAutoscaler{
		ObjectMeta: *scaMeta,
		Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
			TargetRef: &v1alpha1.TargetRef{
				Namespace: targetClusterMeta.Namespace,
				Name:      targetClusterMeta.Name,
			},
			UpdatePolicy: &v1alpha1.UpdatePolicy{
				UpdateMode: *updateMode,
			},
		},
		Status: v1alpha1.ScyllaClusterAutoscalerStatus{
			UpdateStatus: updateStatus,
			Recommendations: &v1alpha1.ScyllaClusterRecommendations{
				DatacenterRecommendations: []v1alpha1.DatacenterRecommendations{
					{
						Name:                dcName,
						RackRecommendations: rackRecs,
					},
				},
			},
		},
	}
}

func setChecksumLabel(checksum string, cluster *scyllav1.ScyllaCluster) *scyllav1.ScyllaCluster {
	cluster.ObjectMeta.Labels = map[string]string{"sca-latest-checksum": checksum}
	return cluster
}

func setExpTimeAndTimestamp(recExpTime *metav1.Duration, lastUpdated metav1.Time,
	sca *v1alpha1.ScyllaClusterAutoscaler) *v1alpha1.ScyllaClusterAutoscaler {
	sca.Spec.UpdatePolicy.RecommendationExpirationTime = recExpTime
	sca.Status.LastUpdated = lastUpdated
	return sca
}

func setCooldownAndTimestamp(updateCooldown *metav1.Duration, lastApplied metav1.Time,
	sca *v1alpha1.ScyllaClusterAutoscaler) *v1alpha1.ScyllaClusterAutoscaler {
	sca.Spec.UpdatePolicy.UpdateCooldown = updateCooldown
	sca.Status.LastApplied = lastApplied
	return sca
}
