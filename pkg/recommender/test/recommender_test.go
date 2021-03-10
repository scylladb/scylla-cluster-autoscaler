package test

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scylladb/go-log"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/metrics"
	rutil "github.com/scylladb/scylla-operator-autoscaler/pkg/recommender/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
	"testing"
	"time"
)

func TestCalculateCPU(t *testing.T) {

	current := resource.MustParse("9223372036854775807")
	min := resource.MustParse("1001m")
	max := resource.MustParse("10")
	factor := float64(4)
	factor = math.MaxFloat64

	// MilliValue to dzielenie z zaokrąglaniem w góre
	fmt.Println("current.value * factor", resource.NewQuantity(int64(factor*float64(current.Value())), current.Format))
	fmt.Println(
		"cur", "val", current.Value(), "millival", current.MilliValue(), current,
		"\nmin", "val", min.Value(), "millival", min.MilliValue(), min,
		"\nmax", "val", max.Value(), "millival", max.MilliValue(), max)

	tests := []struct {
		name                        string
		current, min, max, expected resource.Quantity
		factor                      float64
	}{
		{
			name:     "simple case",
			current:  resource.MustParse("2"),
			min:      resource.MustParse("1"),
			max:      resource.MustParse("10"),
			factor:   float64(2),
			expected: resource.MustParse("4"),
		},
		{
			name:     "scaled CPU exceeds max",
			current:  resource.MustParse("10"),
			min:      resource.MustParse("1"),
			max:      resource.MustParse("10"),
			factor:   float64(2),
			expected: resource.MustParse("10"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := rutil.CalculateCPU(&test.current, &test.min, &test.max, test.factor)
			if res.Value() != test.expected.Value() && res.MilliValue() != test.expected.MilliValue() {
				t.Errorf("test \"%s\" failed, expected %v, got %v", test.name, test.expected, res)
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := rutil.CalculateMembers(test.current, &test.min, &test.max, test.factor)
			if res != test.expected {
				t.Errorf("test \"%s\" failed, expected %v, got %v", test.name, test.expected, res)
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

	tests := []struct {
		name                    string
		queryFun func() (model.Value, v1.Warnings, error)
		expression string
		expected bool
	}{
		{
			name: "simple case",
			queryFun: func() (model.Value, v1.Warnings, error) {
				res := model.Vector{
					{
						Metric: model.Metric{
							"label_name_1.1": "label_value_1.1",
							"label_name_1.2": "label_value_1.2",
						},
						Value:     model.SampleValue(1),
						Timestamp: model.Time(math.MinInt64),
					},
					{
						Metric: model.Metric{
							"label_name_2.1": "label_value_2.1",
							"label_name_2.2": "label_value_2.2",
						},
						Value:     model.SampleValue(2),
						Timestamp: model.Time(math.MaxInt64),
					},
				}
				return res, []string{}, nil
			},
			expression: "",
			expected: true,
		},
		{
			name: "value 0",
			queryFun: func() (model.Value, v1.Warnings, error) {
				res := model.Vector{
					{
						Metric: model.Metric{
							"label_name_1.1": "label_value_1.1",
							"label_name_1.2": "label_value_1.2",
						},
						Value:     model.SampleValue(0),
						Timestamp: model.Time(math.MinInt64),
					},
				}
				return res, []string{}, nil
			},
			expression: "",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics.Q = test.queryFun

			r := recommender.GetEmptyRecommender(logger)
			r.SetMetricsProviderFakeAPI(metrics.MockApi{})

			res, err := r.QueryMetricsProvider(ctx, test.expression)

			if err != nil {
				t.Errorf("test \"%s\" error, err %v", test.name, err)
			} else if test.expected != res {
				t.Errorf("test \"%s\" expected %v, got %v", test.name, test.expected, res)
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

	tests := []struct {
		name                    string
		rangedQueryFun func() (model.Value, v1.Warnings, error)
		expected bool
		expression string
		duration time.Duration
		argStep *time.Duration
	}{
		{
			name: "simple case",
			rangedQueryFun: func() (model.Value, v1.Warnings, error) {
				return nil, []string{}, errors.New("halkooo QueryRange error")
			},
			expression: "",
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metrics.Q = test.rangedQueryFun

			r := recommender.GetEmptyRecommender(logger)
			r.SetMetricsProviderFakeAPI(metrics.MockApi{})

			res, err := r.RangedQueryMetricsProvider(ctx, test.expression, test.duration, test.argStep)

			if err != nil {
				t.Errorf("test \"%s\" error, err %v", test.name, err)
			} else if test.expected != res {
				t.Errorf("test \"%s\" expected %v, got %v", test.name, test.expected, res)
			}
		})
	}
}

/*namespace := "test_namespace"
scName := "test_scylla_cluster_name"
dcName := "test_datacenter_name"
rName := "test_rack1_name"

ctx := log.WithNewTraceID(context.Background())
r :=

tests := []struct {
	name    string
	sc     *v1.ScyllaCluster
	sca     *v1alpha1.ScyllaClusterAutoscaler
	allowed bool
}{
	{
		name:    "same as old",
		sc:     unit.NewSingleRackCluster(3),
		sca:    simpleScyllaClusterAutoscaler(namespace, scName, dcName, rName,
			int32(1), int32(10),
			resource.MustParse("1"),  resource.MustParse("10"),
			v1alpha1.RackControlledValuesRequestsAndLimits),
		allowed: true,
	},
}

for _, test := range tests {
	t.Run(test.name, func(t *testing.T) {
		targetRef := test.sca.Spec.TargetRef
		sc, err := r.fetchScyllaCluster(ctx, targetRef.Name, targetRef.Namespace)
		if err != nil {
			r.logger.Error(ctx, "fetch referenced ScyllaCluster", "error", err)
			continue
		}

		sca.Status.Recommendations = r.getScyllaClusterRecommendations(ctx, sc, sca.Spec.ScalingPolicy)
		err = r.client.Status().Update(ctx, &sca)
		if err != nil {
			r.logger.Error(ctx, "SCA status update", "error", err)
			continue
		}

		err :=
		if test.allowed {
			require.NoError(t, err, "Wrong value returned from checkTransitions function. Message: '%s'", err)
		} else {
			require.Error(t, err, "Wrong value returned from checkTransitions function. Message: '%s'", err)
		}
	})
}*/
