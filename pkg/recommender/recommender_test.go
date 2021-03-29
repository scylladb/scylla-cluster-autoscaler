package recommender

import (
	"context"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	mockprometheusapi "github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics/mock"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"math"
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

func TestCalculateCPU(t *testing.T) {

	var (
		factor2   = 2.0
		factorMax = float64(math.MaxInt64)
		factor10  = 10.0
	)
	tests := []struct {
		name                        string
		current, min, max, expected *resource.Quantity
		factor                      float64
		t                           *resource.Quantity
	}{
		{
			name:     "Allowed scaling with Value",
			current:  util.ParseQuantity("2"),
			min:      util.ParseQuantity("1"),
			max:      util.ParseQuantity("10"),
			factor:   factor2,
			expected: util.ParseQuantity("4"),
		},
		{
			name:     "Allowed scaling with MilliValue",
			current:  util.ParseQuantity("1001m"),
			min:      util.ParseQuantity("1"),
			max:      util.ParseQuantity("10"),
			factor:   factor2,
			expected: util.ParseQuantity("2002m"),
		},
		{
			name:     "Simple scaling with nil max and min values",
			current:  util.ParseQuantity("2"),
			factor:   factor2,
			expected: util.ParseQuantity("4"),
		},
		{
			name:     "Scaled CPU Value exceeds max",
			current:  util.ParseQuantity("10"),
			min:      util.ParseQuantity("1"),
			max:      util.ParseQuantity("10"),
			factor:   factor2,
			expected: util.ParseQuantity("10"),
		},
		{
			name:     "Current MilliValue and scaled Value (Value()*factor) overflow int64",
			current:  util.NewQuantity(math.MaxInt64),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   factorMax,
			expected: util.NewQuantity(math.MaxInt64 - 1),
		},
		{
			name:     "Current MilliValue does not overflow int64 but scaled MilliValue and Value (MilliValue()*factor) do",
			current:  util.NewMilliQuantity(math.MaxInt64),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   factorMax,
			expected: util.NewQuantity(math.MaxInt64 - 1),
		},
		{
			name:     "Current MilliValue does not overflow int64, scaled MilliValue (MilliValue()*factor) does, but scaled value does not",
			current:  util.NewMilliQuantity(math.MaxInt64),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   factor10,
			expected: resource.NewQuantity(int64(factor10*float64(util.NewMilliQuantity(math.MaxInt64).Value())), resource.DecimalSI),
		},
		{
			name:     "Current MilliValue does overflow int64 but scaled Value (Value()*factor) doesn't",
			current:  util.NewQuantity(math.MaxInt64 / 100),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   factor10,
			expected: resource.NewQuantity(int64(factor10*float64(util.NewQuantity(math.MaxInt64/100).Value())), resource.DecimalSI),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := CalculateCPU(test.current, test.min, test.max, test.factor)
			if res.Value() != test.expected.Value() && res.MilliValue() != test.expected.MilliValue() {
				t.Errorf("test \"%s\" failed, expectedRecommendations %v, got %v", test.name, test.expected.Value(), res.Value())
			}

		})
	}
}

func TestCalculateMembers(t *testing.T) {

	tests := []struct {
		name                        string
		current, min, max, expected int32
		factor                      float64
	}{
		{
			name:     "Allowed scaling up",
			current:  2,
			min:      1,
			max:      10,
			factor:   2,
			expected: 4,
		},
		{
			name:     "Scaling up capped by max value",
			current:  2,
			min:      1,
			max:      10,
			factor:   100,
			expected: 10,
		},
		{
			name:     "Scaling down capped by min value",
			current:  100,
			min:      11,
			max:      20,
			factor:   0.1,
			expected: 11,
		},
		{
			name:     "Scaled current value would overflow int32",
			current:  math.MaxInt32 - 1,
			min:      1,
			max:      math.MaxInt32 - 2,
			factor:   2,
			expected: math.MaxInt32 - 2,
		},
		{
			name:     "Scaled current value would overflow int32, but with nil min and max values",
			current:  math.MaxInt32 - 1,
			factor:   2,
			expected: math.MaxInt32,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var min, max *int32
			if test.min == test.max && test.max == 0 {
				min, max = nil, nil
			} else {
				min, max = &test.min, &test.max
			}
			res := CalculateMembers(test.current, min, max, test.factor)
			if res != test.expected {
				t.Errorf("test \"%s\" failed, expectedRecommendations %v, got %v", test.name, test.expected, res)
			}

		})
	}
}

func TestRunOnce(t *testing.T) {
	const (
		dcName            = "dc_name"
		rackName          = "rack_name"
		ruleName          = "rule_name"
		baseMembers       = 3
		baseCpu           = "5"
		higherCpu         = "40"
		memory            = "1Gi"
		minAllowedMembers = 1
		maxAllowedMembers = 100
		priority1         = 1
		priority2         = 2
		priority3         = 3
		factor2           = 2.0
		factor4           = 4.0
		factor6           = 6.0
		scName            = "test-sc"
		scNamespace       = "test-sc-ns"
		scaName           = "test-sca"
		scaNamespace      = "test-sca-ns"
	)
	var (
		minAllowedCpu             = resource.MustParse("1")
		maxAllowedCpu             = resource.MustParse("100")
		duration5                 = util.DurationPtr(5)
		duration10                = util.DurationPtr(10)
		statusTargetFetchFail     = v1alpha1.UpdateStatusTargetFetchFail
		statusTargetNotReady      = v1alpha1.UpdateStatusTargetNotReady
		statusRecommendationsFail = v1alpha1.UpdateStatusRecommendationsFail
	)
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
	c := clientBuilder.Build()
	m := mockprometheusapi.NewMockApi(mockprometheusapi.SimpleQueryFunction(), mockprometheusapi.SimpleRangedQueryFunction())
	pp := metrics.NewPrometheusProvider(m, logger, time.Minute)
	r := New(c, pp, logger)

	tests := []struct {
		name                    string
		sc                      *scyllav1.ScyllaCluster
		sca                     *v1alpha1.ScyllaClusterAutoscaler
		expectedRecommendations *v1alpha1.ScyllaClusterRecommendations
		expectedStatus          *v1alpha1.UpdateStatus
	}{
		{
			name: "Recommend scaling members because of better priority",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeHorizontal, factor2),
						*newScalingRule(ruleName, priority2, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, factor4),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, baseCpu, baseCpu, memory, baseMembers*factor2),
			),
		},
		{
			name: "Recommends scaling cpu because of better priority",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority2, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeHorizontal, factor2),
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, factor4),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, stringMulFloat64(baseCpu, factor4), stringMulFloat64(baseCpu, factor4), memory, baseMembers),
			),
		},
		{
			name: "Recommends scaling cpu because of better priority, but with duration for, duration step",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority2, mockprometheusapi.QueryWillReturnTrue, duration5, duration10, v1alpha1.ScalingModeHorizontal, factor2),
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, duration5, duration10, v1alpha1.ScalingModeVertical, factor4),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, stringMulFloat64(baseCpu, factor4), stringMulFloat64(baseCpu, factor4), memory, baseMembers),
			),
		},
		{
			name: "Recommends scaling cpu from accepting expression with highest priority",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority3, mockprometheusapi.QueryWillReturnTrue, duration5, duration10, v1alpha1.ScalingModeHorizontal, factor2),
						*newScalingRule(ruleName, priority2, mockprometheusapi.QueryWillReturnTrue, duration5, duration10, v1alpha1.ScalingModeHorizontal, factor4),
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnFalse, duration5, duration10, v1alpha1.ScalingModeHorizontal, factor6),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, baseCpu, baseCpu, memory, baseMembers*factor4),
			),
		},
		{
			name: "getRackRecommendations propagates error returned by Query",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.IncorrectQueryExpr, nil, nil, v1alpha1.ScalingModeHorizontal, factor6),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedStatus: &statusRecommendationsFail,
		},
		{
			name: "getRackRecommendations propagates error returned by RangedQuery",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.IncorrectQueryExpr, duration5, duration10, v1alpha1.ScalingModeHorizontal, factor6),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedStatus: &statusRecommendationsFail,
		},
		{
			name: "no scaling policy",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca:            newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName, nil),
			expectedStatus: &statusRecommendationsFail,
		},
		{
			name: "no rack spec",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{},
				map[string]scyllav1.RackStatus{}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.IncorrectQueryExpr, duration5, duration10, v1alpha1.ScalingModeHorizontal, factor6),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedStatus: &statusRecommendationsFail,
		},
		{
			name: "Recommends scaling requests cpu",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, higherCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, factor4),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequests)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, stringMulFloat64(baseCpu, factor4), higherCpu, memory, baseMembers),
			),
		},
		{
			name: "Recommends scaling requests cpu but cpu limits caps it",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, factor4),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequests)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, baseCpu, higherCpu, memory, baseMembers),
			),
		},
		{
			name: "No scylla cluster",
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeHorizontal, factor2),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedRecommendations: newSingleDcSCRecommendations(
				dcName,
				*newRackRecommendations(rackName, baseCpu, baseCpu, memory, baseMembers*factor2),
			),
			expectedStatus: &statusTargetFetchFail,
		},
		{
			name: "Scylla cluster not ready",
			sc: newSingleDcSc(scName, scNamespace, dcName,
				[]scyllav1.RackSpec{
					*getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
				},
				map[string]scyllav1.RackStatus{
					rackName: *getRackStatus(baseMembers, baseMembers-1),
				}),
			sca: newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName,
				newRackScalingPolicy(rackName,
					[]v1alpha1.ScalingRule{
						*newScalingRule(ruleName, priority1, mockprometheusapi.QueryWillReturnTrue, nil, nil, v1alpha1.ScalingModeHorizontal, factor2),
					},
					minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
					v1alpha1.RackControlledValuesRequestsAndLimits)),
			expectedStatus: &statusTargetNotReady,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.sc != nil {
				err := c.Create(ctx, test.sc)
				require.NoError(t, err, "Couldn't create scylla cluster. Message: '%s'", err)
			}
			if test.sca != nil {
				err := c.Create(ctx, test.sca)
				require.NoError(t, err, "Couldn't create SCA. Message: '%s'", err)
			}

			err := r.RunOnce(ctx)
			require.NoError(t, err, "Couldn't get scylla cluster autoscaler. Message: '%s'", err)

			sca := &v1alpha1.ScyllaClusterAutoscaler{}
			if test.sca != nil {
				err = c.Get(ctx, client.ObjectKey{
					Namespace: test.sca.Namespace,
					Name:      test.sca.Name,
				}, sca)
			}

			if test.expectedStatus != nil {
				require.Equal(t, test.expectedStatus, sca.Status.UpdateStatus)
			} else {
				require.NoError(t, err, "Run Once returned error. Message: '%s'", err)
				if !scsRecommendationsEquivalent(
					sca.Status.Recommendations,
					test.expectedRecommendations) {
					t.Errorf("test \"%s\"\nexpected result \n%v\ngot \n%v",
						test.name,
						test.expectedRecommendations, sca.Status.Recommendations)
				}
			}

			if test.sc != nil {
				err = c.Delete(ctx, test.sc)
				require.NoError(t, err, "Couldn't delete scylla cluster. Message: '%s'", err)
			}
			if test.sca != nil {
				err = c.Delete(ctx, test.sca)
				require.NoError(t, err, "Couldn't delete SCA. Message: '%s'", err)
			}
		})
	}
}
