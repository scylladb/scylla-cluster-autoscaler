package test

import (
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"time"
)

func rackRecommendationsEquivalent(rec1, rec2 v1alpha1.RackRecommendations) bool {
	return rec1.Name == rec2.Name &&
		*rec1.Members == *rec2.Members &&
		rec1.Resources.Requests.Cpu().Cmp(*rec2.Resources.Requests.Cpu()) == 0 &&
		rec1.Resources.Requests.Memory().Cmp(*rec2.Resources.Requests.Memory()) == 0
}

func getRackSpec(name string, members int32, cpuRequest, cpuLimit, memoryRequest, memoryLimit string) *scyllav1.RackSpec {
	return &scyllav1.RackSpec{
		Name:    name,
		Members: members,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpuRequest),
				corev1.ResourceMemory: resource.MustParse(memoryRequest),
			},
			Limits: func() corev1.ResourceList {
				if cpuLimit != "" && memoryLimit != "" {
					return corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(cpuLimit),
						corev1.ResourceMemory: resource.MustParse(memoryLimit),
					}
				} else {
					return nil
				}
			}(),
		},
	}
}

func getScalingPolicy(rackName string, rules []v1alpha1.ScalingRule, minAllowedMembers, maxAllowedMembers int32, minAllowedCpu, maxAllowedCpu resource.Quantity, controlledValues v1alpha1.RackControlledValues) *v1alpha1.RackScalingPolicy {
	return &v1alpha1.RackScalingPolicy{
		Name: rackName,
		MemberPolicy: &v1alpha1.RackMemberPolicy{
			MinAllowed: &minAllowedMembers,
			MaxAllowed: &maxAllowedMembers,
		},
		ResourcePolicy: &v1alpha1.RackResourcePolicy{
			MinAllowedCpu:        &minAllowedCpu,
			MaxAllowedCpu:        &maxAllowedCpu,
			RackControlledValues: controlledValues,
		},
		ScalingRules: rules,
	}
}

func getScalingRule(name string, priority int32, expression string, durFor, durStep *metav1.Duration, mode v1alpha1.ScalingMode, factor float64) *v1alpha1.ScalingRule {
	return &v1alpha1.ScalingRule{
		Name:          name,
		Priority:      priority,
		Expression:    expression,
		For:           durFor,
		Step:          durStep,
		ScalingMode:   mode,
		ScalingFactor: factor,
	}
}

const (
	queryWillReturnTrue = "queryWillReturnTrue"
	queryWillReturnFalse = "queryWillReturnFalse"
	incorrectQueryExpr = "incorrectQueryExpr"
)

func SimpleQueryFunction() func(string, time.Time) (model.Value, v1.Warnings, error) {
	return func(query string, _ time.Time) (model.Value, v1.Warnings, error) {

		var value int
		if query == incorrectQueryExpr {
			return nil, v1.Warnings{}, errors.New("Incorrect query expression")
		} else if query == queryWillReturnTrue {
			value = 1
		} else if query == queryWillReturnFalse {
			value = 0
		} else {
			panic("Incorrect usage of SimpleQueryFunction in tests. Possible query expressions are: " +
				"\"queryWillReturnTrue\", \"queryWillReturnFalse\", \"incorrectQueryExpr\"")
		}

		res := model.Vector{
			{
				Metric: model.Metric{
					"label_name_1.1": "label_value_1.1",
					"label_name_1.2": "label_value_1.2",
				},
				Value:     model.SampleValue(value),
				Timestamp: model.Time(math.MinInt64),
			},
			{
				Metric: model.Metric{
					"label_name_2.1": "label_value_2.1",
					"label_name_2.2": "label_value_2.2",
				},
				Value:     model.SampleValue(value),
				Timestamp: model.Time(math.MaxInt64),
			},
		}
		return res, v1.Warnings{}, nil
	}
}

func SimpleRangedQueryFunction() func(string, v1.Range) (model.Value, v1.Warnings, error) {
	return func(query string, r v1.Range) (model.Value, v1.Warnings, error) {
		var value int
		if query == incorrectQueryExpr {
			return nil, v1.Warnings{}, errors.New("Incorrect query expression")
		} else if query == queryWillReturnTrue {
			value = 1
		} else if query == queryWillReturnFalse {
			value = 0
		} else {
			panic("Incorrect usage of SimpleQueryFunction in tests. Possible query expressions are: " +
				"\"queryWillReturnTrue\", \"queryWillReturnFalse\", \"incorrectQueryExpr\"")
		}
		res := model.Matrix{
			{
				Metric: model.Metric{
					"label_name_1.1": "label_value_1.1",
					"label_name_1.2": "label_value_1.2",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(math.MaxInt64 - 1),
						Value:     model.SampleValue(value),
					},
					{
						Timestamp: model.Time(math.MaxInt64),
						Value:     model.SampleValue(value),
					},
				},
			},
			{
				Metric: model.Metric{
					"label_name_2.1": "label_value_2.1",
					"label_name_2.2": "label_value_2.2",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(math.MaxInt64 - 1),
						Value:     model.SampleValue(value),
					},
					{
						Timestamp: model.Time(math.MaxInt64),
						Value:     model.SampleValue(value),
					},
				},
			},
		}
		return res, v1.Warnings{}, nil
	}
}
