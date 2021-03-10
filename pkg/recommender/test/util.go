package test

import (
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func simpleScyllaClusterAutoscaler(namespace, scName, dcName, rName string,
	minAllowedMembers, maxAllowedMembers int32,
	minAllowedCpu, maxAllowedCpu resource.Quantity,
	rackControlledValues v1alpha1.RackControlledValues) *v1alpha1.ScyllaClusterAutoscaler {
	return &v1alpha1.ScyllaClusterAutoscaler{
		Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
			TargetRef: &v1alpha1.TargetRef{
				Namespace: namespace,
				Name:      scName,
			},
			UpdatePolicy: &v1alpha1.UpdatePolicy{
				UpdateMode: v1alpha1.UpdateModeOn,
			},
			ScalingPolicy: &v1alpha1.ScalingPolicy{
				Datacenters: []v1alpha1.DatacenterScalingPolicy{
					{
						Name: dcName,
						RackScalingPolicies: []v1alpha1.RackScalingPolicy{
							{
								Name: rName,
								MemberPolicy: &v1alpha1.RackMemberPolicy{
									MinAllowed: &minAllowedMembers,
									MaxAllowed: &maxAllowedMembers,
								},
								ResourcePolicy: &v1alpha1.RackResourcePolicy{
									MinAllowedCpu:        &minAllowedCpu,
									MaxAllowedCpu:        &maxAllowedCpu,
									RackControlledValues: rackControlledValues,
								},
								ScalingRules: []v1alpha1.ScalingRule{
									{
										Name:          "rule1",
										Priority:      int32(1),
										Expression:    "expression",
										For:           &metav1.Duration{Duration: time.Duration(5)},
										Step:          &metav1.Duration{Duration: time.Duration(10)},
										ScalingMode:   v1alpha1.ScalingModeHorizontal,
										ScalingFactor: 2.0,
									},
									{
										Name:          "rule2",
										Priority:      int32(2),
										Expression:    "expression",
										For:           &metav1.Duration{Duration: time.Duration(5)},
										Step:          &metav1.Duration{Duration: time.Duration(10)},
										ScalingMode:   v1alpha1.ScalingModeHorizontal,
										ScalingFactor: 2.0,
									},
								},
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.ScyllaClusterAutoscalerStatus{
			Recommendations: &v1alpha1.ScyllaClusterRecommendations{
				DatacenterRecommendations: []v1alpha1.DatacenterRecommendations{},
			},
		},
	}
}
