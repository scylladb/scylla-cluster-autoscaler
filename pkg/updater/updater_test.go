package updater

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = scyllav1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
}

func NewSingleDcSca(scaMeta *metav1.ObjectMeta, updateMode *v1alpha1.UpdateMode, targetClusterMeta *metav1.ObjectMeta,
	dcName string, rackRecs []v1alpha1.RackRecommendations) *v1alpha1.ScyllaClusterAutoscaler {
	return &v1alpha1.ScyllaClusterAutoscaler{
		ObjectMeta: *scaMeta,
		Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
			TargetRef: &v1alpha1.TargetRef{
				Namespace: targetClusterMeta.Namespace,
				Name:      targetClusterMeta.Name,
			},
			UpdatePolicy: &v1alpha1.UpdatePolicy{
				UpdateMode: updateMode,
			},
		},
		Status: v1alpha1.ScyllaClusterAutoscalerStatus{
			Recommendations: &v1alpha1.ScyllaClusterRecommendations{
				DataCenterRecommendations: []v1alpha1.DataCenterRecommendations{
					{
						Name:                dcName,
						RackRecommendations: rackRecs,
					},
				},
			},
		},
	}
}

func NewSingleDcScyllaCluster(clusterMeta *metav1.ObjectMeta, dcName string, racksSpec []scyllav1.RackSpec,
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

type ExpectedStateSpec struct {
	Name                 string
	ExpectedMembersValue int32
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
	tests := []struct {
		Name           string
		ScyllaCluster  *scyllav1.ScyllaCluster
		Sca            *v1alpha1.ScyllaClusterAutoscaler
		ExpectedStates []ExpectedStateSpec
	}{
		{
			Name: "applied recommendation",
			ScyllaCluster: NewSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: NewSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, basicTestClusterMeta, "test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: &v1alpha1.RecommendedRackMembers{Target: 2}},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{Name: "test-rack-1", ExpectedMembersValue: 2},
			},
		},
		{
			Name: "requested members not equal to ready",
			ScyllaCluster: NewSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 2},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 2, ReadyMembers: 1},
				}),
			Sca: NewSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, basicTestClusterMeta, "test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: &v1alpha1.RecommendedRackMembers{Target: 3}},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{Name: "test-rack-1", ExpectedMembersValue: 2},
			},
		},
		{
			Name: "target members equal to spec members, but not to status members",
			ScyllaCluster: NewSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 2},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: NewSingleDcSca(basicTestAutoModeScaMeta, &autoUpdateMode, basicTestClusterMeta, "test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: &v1alpha1.RecommendedRackMembers{Target: 2}},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{Name: "test-rack-1", ExpectedMembersValue: 2},
			},
		},
		{
			Name: "off mode sca",
			ScyllaCluster: NewSingleDcScyllaCluster(basicTestClusterMeta, "test-dc",
				[]scyllav1.RackSpec{
					{Name: "test-rack-1", Members: 1},
				},
				map[string]scyllav1.RackStatus{
					"test-rack-1": {Members: 1, ReadyMembers: 1},
				}),
			Sca: NewSingleDcSca(basicTestOffModeScaMeta, &offUpdateMode, basicTestClusterMeta, "test-dc",
				[]v1alpha1.RackRecommendations{
					{Name: "test-rack-1", Members: &v1alpha1.RecommendedRackMembers{Target: 2}},
				}),
			ExpectedStates: []ExpectedStateSpec{
				{Name: "test-rack-1", ExpectedMembersValue: 1},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := c.Create(ctx, test.ScyllaCluster)
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
				rack := findRack(expectedState.Name, cluster.Spec.Datacenter.Racks)
				require.NotNil(t, rack)
				require.Equal(t, expectedState.ExpectedMembersValue, rack.Members)
			}

			err = c.Delete(ctx, test.ScyllaCluster)
			require.NoError(t, err, "Couldn't delete scylla cluster. Message: '%s'", err)
			err = c.Delete(ctx, test.Sca)
			require.NoError(t, err, "Couldn't delete SCA. Message: '%s'", err)

		})
	}
}
