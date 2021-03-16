package test

import (
	"context"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	rutil "github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
	"testing"
	"time"
)

func TestCalculateCPU(t *testing.T) {

	tests := []struct {
		name                        string
		current, min, max, expected *resource.Quantity
		factor                      float64
		t                           *resource.Quantity
	}{
		{
			name:     "simple case with Value",
			current:  util.ParseQuantity("2"),
			min:      util.ParseQuantity("1"),
			max:      util.ParseQuantity("10"),
			factor:   float64(2),
			expected: util.ParseQuantity("4"),
		},
		{
			name:     "simple case with MilliValue",
			current:  util.ParseQuantity("1001m"),
			min:      util.ParseQuantity("1"),
			max:      util.ParseQuantity("10"),
			factor:   float64(2),
			expected: util.ParseQuantity("2002m"),
		},
		{
			name:     "simple case but with nil max and min values",
			current:  util.ParseQuantity("2"),
			factor:   float64(2),
			expected: util.ParseQuantity("4"),
		},
		{
			name:     "scaled CPU Value exceeds max",
			current:  util.ParseQuantity("10"),
			min:      util.ParseQuantity("1"),
			max:      util.ParseQuantity("10"),
			factor:   float64(2),
			expected: util.ParseQuantity("10"),
		},
		{
			name:     "current MilliValue and scaled Value (Value()*factor) overflow int64",
			current:  util.NewQuantity(math.MaxInt64),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   float64(math.MaxInt64),
			expected: util.NewQuantity(math.MaxInt64 - 1),
		},
		{
			name:     "current MilliValue does not overflow int64 but scaled MilliValue and Value (MilliValue()*factor) do",
			current:  util.NewMilliQuantity(math.MaxInt64),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   float64(math.MaxInt64),
			expected: util.NewQuantity(math.MaxInt64 - 1),
		},
		{
			name:     "current MilliValue does not overflow int64, scaled MilliValue (MilliValue()*factor) does, but scaled value does not",
			current:  util.NewMilliQuantity(math.MaxInt64),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   float64(10),
			expected: resource.NewQuantity(int64(float64(10)*float64(util.NewMilliQuantity(math.MaxInt64).Value())), resource.DecimalSI),
		},
		{
			name:     "current MilliValue does overflow int64 but scaled Value (Value()*factor) doesn't",
			current:  util.NewQuantity(math.MaxInt64 / 100),
			min:      util.ParseQuantity("1"),
			max:      util.NewQuantity(math.MaxInt64 - 1),
			factor:   float64(10),
			expected: resource.NewQuantity(int64(float64(10)*float64(util.NewQuantity(math.MaxInt64/100).Value())), resource.DecimalSI),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := rutil.CalculateCPU(test.current, test.min, test.max, test.factor)
			if res.Value() != test.expected.Value() && res.MilliValue() != test.expected.MilliValue() {
				t.Errorf("test \"%s\" failed, expectedResult %v, got %v", test.name, test.expected.Value(), res.Value())
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
			name:     "simple case",
			current:  2,
			min:      1,
			max:      10,
			factor:   2,
			expected: 4,
		},
		{
			name:     "scaling up capped by max value",
			current:  2,
			min:      1,
			max:      10,
			factor:   100,
			expected: 10,
		},
		{
			name:     "scaling down capped by min value",
			current:  100,
			min:      11,
			max:      20,
			factor:   0.1,
			expected: 11,
		},
		{
			name:     "scaled current value would overflow int32",
			current:  math.MaxInt32 - 1,
			min:      1,
			max:      math.MaxInt32 - 2,
			factor:   2,
			expected: math.MaxInt32 - 2,
		},
		{
			name:     "scaled current value would overflow int32, but nil min and max values",
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
			res := rutil.CalculateMembers(test.current, min, max, test.factor)
			if res != test.expected {
				t.Errorf("test \"%s\" failed, expectedResult %v, got %v", test.name, test.expected, res)
			}

		})
	}
}

func TestPrometheusProviderQuery(t *testing.T) {

	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})
	r := recommender.GetEmptyRecommender(logger)
	r.SetMetricsProviderFakeAPI(metrics.MockApi{})

	tests := []struct {
		name           string
		queryFun       func(string, time.Time) (model.Value, v1.Warnings, error)
		queryExpr      string
		expectedResult bool
		errorExpected  bool
	}{
		{
			name:           "Simple case",
			queryFun:       SimpleQueryFunction(),
			expectedResult: true,
		},
		{
			name:           "Query replies negatively",
			queryFun:       SimpleQueryFunction(),
			queryExpr:      queryWillReturnFalse,
			expectedResult: false,
		},
		{
			name:          "Incorrect query expression",
			queryFun:      SimpleQueryFunction(),
			queryExpr:     incorrectQueryExpr,
			errorExpected: true,
		},
		{
			name: "Query returns arbitrary error",
			queryFun: func(string, time.Time) (model.Value, v1.Warnings, error) {
				return nil, v1.Warnings{}, errors.New("Arbitrary error")
			},
			errorExpected: true,
		},
		{
			name: "Query returns unexpected value type",
			queryFun: func(string, time.Time) (model.Value, v1.Warnings, error) {
				return model.Matrix{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
		{
			name: "Query returns empty result vector",
			queryFun: func(string, time.Time) (model.Value, v1.Warnings, error) {
				return model.Vector{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics.Q = test.queryFun
			if test.queryExpr == "" {
				test.queryExpr = queryWillReturnTrue
			}
			res, err := r.ExportedQueryOnMetricsProvider(ctx, test.queryExpr)
			if !test.errorExpected {
				if err != nil {
					t.Errorf("test \"%s\" error, err %v", test.name, err)
				} else if test.expectedResult != res {
					t.Errorf("test \"%s\" expected result %v, got %v", test.name, test.expectedResult, res)
				}
			}
		})
	}
}

func TestPrometheusProviderRangedQuery(t *testing.T) {

	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})
	r := recommender.GetEmptyRecommender(logger)
	r.SetMetricsProviderFakeAPI(metrics.MockApi{})

	tests := []struct {
		name           string
		rangedQueryFun func(string, v1.Range) (model.Value, v1.Warnings, error)
		queryExpr      string
		duration       time.Duration
		argStep        *time.Duration
		expectedResult bool
		errorExpected  bool
	}{
		{
			name:           "Simple case",
			rangedQueryFun: SimpleRangedQueryFunction(),
			expectedResult: true,
		},
		{
			name:           "Ranged query replies negatively",
			rangedQueryFun: SimpleRangedQueryFunction(),
			queryExpr:      queryWillReturnFalse,
			expectedResult: false,
		},
		{
			name:           "Incorrect query expression",
			rangedQueryFun: SimpleRangedQueryFunction(),
			queryExpr:      incorrectQueryExpr,
			errorExpected:  true,
		},
		{
			name: "Ranged query returns arbitrary error",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return nil, v1.Warnings{}, errors.New("Arbitrary error")
			},
			errorExpected: true,
		},
		{
			name: "Ranged query returns unexpected value type",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return model.Vector{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
		{
			name: "Query returns empty result Matrix",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return model.Matrix{}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
		{
			name: "Query returns Matrix containing empty record",
			rangedQueryFun: func(string, v1.Range) (model.Value, v1.Warnings, error) {
				return model.Matrix{{}}, v1.Warnings{}, nil
			},
			errorExpected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics.Qr = test.rangedQueryFun
			if test.queryExpr == "" {
				test.queryExpr = queryWillReturnTrue
			}
			res, err := r.ExportedRangedQueryOnMetricsProvider(ctx, test.queryExpr, test.duration, test.argStep)

			if !test.errorExpected {
				if err != nil {
					t.Errorf("test \"%s\" error, err %v", test.name, err)
				} else if test.expectedResult != res {
					t.Errorf("test \"%s\" expected result %v, got %v", test.name, test.expectedResult, res)
				}
			}
		})
	}
}

func TestGetRackRecommendations(t *testing.T) {
	const (
		rackName          = "rack_0_name"
		ruleName          = "rule_0_name"
		baseMembers       = 3
		baseCpu           = "5"
		memory            = "1Gi"
		minAllowedMembers = 1
		maxAllowedMembers = 100
	)
	var (
		minAllowedCpu = resource.MustParse("1")
		maxAllowedCpu = resource.MustParse("100")
	)
	ctx := log.WithNewTraceID(context.Background())
	atom := zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, _ := log.NewProduction(log.Config{
		Level: atom,
	})
	r := recommender.GetEmptyRecommender(logger)
	r.SetMetricsProviderFakeAPI(metrics.MockApi{})
	metrics.Qr = SimpleRangedQueryFunction()
	metrics.Q = SimpleQueryFunction()

	tests := []struct {
		name           string
		rack           *scyllav1.RackSpec
		scalingPolicy  *v1alpha1.RackScalingPolicy
		expectedResult *v1alpha1.RackRecommendations
		errorExpected  bool
	}{
		{
			name: "Recommend scaling members",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 1, queryWillReturnTrue, nil, nil, v1alpha1.ScalingModeHorizontal, 2.0),
					*getScalingRule(ruleName, 2, queryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, 4.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequestsAndLimits),
			expectedResult: &v1alpha1.RackRecommendations{
				Name:    rackName,
				Members: util.Int32ptr(baseMembers * 2.0),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(baseCpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(baseCpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
		{
			name: "Recommends scaling cpu",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 2, queryWillReturnTrue, nil, nil, v1alpha1.ScalingModeHorizontal, 2.0),
					*getScalingRule(ruleName, 1, queryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, 4.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequestsAndLimits),
			expectedResult: &v1alpha1.RackRecommendations{
				Name:    rackName,
				Members: util.Int32ptr(baseMembers),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20"),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20"),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
		{
			name: "Recommends scaling cpu with duration for, duration step",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 2, queryWillReturnTrue, util.DurationPtr(5), util.DurationPtr(10), v1alpha1.ScalingModeHorizontal, 2.0),
					*getScalingRule(ruleName, 1, queryWillReturnTrue, util.DurationPtr(5), util.DurationPtr(10), v1alpha1.ScalingModeVertical, 4.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequestsAndLimits),
			expectedResult: &v1alpha1.RackRecommendations{
				Name:    rackName,
				Members: util.Int32ptr(baseMembers),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20"),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20"),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
		{
			name: "Recommends scaling cpu from accepting expression with highest priority",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 3, queryWillReturnTrue, util.DurationPtr(5), util.DurationPtr(10), v1alpha1.ScalingModeHorizontal, 2.0),
					*getScalingRule(ruleName, 2, queryWillReturnTrue, util.DurationPtr(5), util.DurationPtr(10), v1alpha1.ScalingModeHorizontal, 4.0),
					*getScalingRule(ruleName, 1, queryWillReturnFalse, util.DurationPtr(5), util.DurationPtr(10), v1alpha1.ScalingModeHorizontal, 6.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequestsAndLimits),
			expectedResult: &v1alpha1.RackRecommendations{
				Name:    rackName,
				Members: util.Int32ptr(baseMembers * 4.0),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(baseCpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(baseCpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
		{
			name: "getRackRecommendations propagates error returned by Query",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 1, incorrectQueryExpr, nil, nil, v1alpha1.ScalingModeHorizontal, 6.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequestsAndLimits),
			errorExpected: true,
		},
		{
			name: "getRackRecommendations propagates error returned by RangedQuery",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 1, incorrectQueryExpr, util.DurationPtr(5), util.DurationPtr(10), v1alpha1.ScalingModeHorizontal, 6.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequestsAndLimits),
			errorExpected: true,
		},
		{
			name:          "no scaling policy",
			rack:          getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: nil,
			errorExpected: true,
		},
		{
			name:          "no rack spec",
			rack:          nil,
			scalingPolicy: nil,
			errorExpected: true,
		},
		{
			name: "Recommends scaling requests cpu",
			rack: getRackSpec(rackName, baseMembers, baseCpu, "40", memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 1, queryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, 4.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequests),
			expectedResult: &v1alpha1.RackRecommendations{
				Name:    rackName,
				Members: util.Int32ptr(baseMembers),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("20"),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("40"),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
		{
			name: "Recommends scaling requests cpu but cpu limits caps it",
			rack: getRackSpec(rackName, baseMembers, baseCpu, baseCpu, memory, memory),
			scalingPolicy: getScalingPolicy(rackName,
				[]v1alpha1.ScalingRule{
					*getScalingRule(ruleName, 1, queryWillReturnTrue, nil, nil, v1alpha1.ScalingModeVertical, 4.0),
				},
				minAllowedMembers, maxAllowedMembers, minAllowedCpu, maxAllowedCpu,
				v1alpha1.RackControlledValuesRequests),
			expectedResult: &v1alpha1.RackRecommendations{
				Name:    rackName,
				Members: util.Int32ptr(baseMembers),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(baseCpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(baseCpu),
						corev1.ResourceMemory: resource.MustParse(memory),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := r.ExportedGetRackRecommendations(ctx, test.rack, test.scalingPolicy)

			if !test.errorExpected {
				if err != nil {
					t.Errorf("test \"%s\" error, err %v", test.name, err)
				} else if !rackRecommendationsEquivalent(*res, *test.expectedResult) {
					t.Errorf("test \"%s\"\nexpected result \n%v\ngot \n%v",
						test.name,
						test.expectedResult, res)
				}
			}
		})
	}
}
