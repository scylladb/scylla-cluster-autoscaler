package recommender

import (
	"fmt"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/api/v1alpha1"
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

func newSingleDcSca(scaName, scaNamespace, scName, scNamespace, dcName string, rackScalingPolicy *v1alpha1.RackScalingPolicy) *v1alpha1.ScyllaClusterAutoscaler {
	return &v1alpha1.ScyllaClusterAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scaName,
			Namespace: scaNamespace,
		},
		Spec: v1alpha1.ScyllaClusterAutoscalerSpec{
			TargetRef: &v1alpha1.TargetRef{
				Namespace: scNamespace,
				Name:      scName,
			},
			UpdatePolicy: &v1alpha1.UpdatePolicy{
				UpdateMode: v1alpha1.UpdateModeAuto,
			},
			ScalingPolicy: &v1alpha1.ScalingPolicy{
				Datacenters: []v1alpha1.DatacenterScalingPolicy{
					{
						Name: dcName,
						RackScalingPolicies: []v1alpha1.RackScalingPolicy{
							func() v1alpha1.RackScalingPolicy {
								if rackScalingPolicy != nil {
									return *rackScalingPolicy
								} else {
									return v1alpha1.RackScalingPolicy{}
								}
							}(),
						},
					},
				},
			},
		},
		Status: v1alpha1.ScyllaClusterAutoscalerStatus{
			Recommendations: &v1alpha1.ScyllaClusterRecommendations{
				DatacenterRecommendations: []v1alpha1.DatacenterRecommendations{
					{
						Name:                dcName,
						RackRecommendations: []v1alpha1.RackRecommendations{},
					},
				},
			},
		},
	}
}

func newSingleDcSc(scName, scNamespace, dcName string, racksSpec []scyllav1.RackSpec,
	racksStatus map[string]scyllav1.RackStatus) *scyllav1.ScyllaCluster {

	return &scyllav1.ScyllaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scName,
			Namespace: scNamespace,
		},
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

func scsRecommendationsEquivalent(rec1, rec2 *v1alpha1.ScyllaClusterRecommendations) bool {
	for _, dcRec1 := range rec1.DatacenterRecommendations {
		dcRec2 := findDc(dcRec1.Name, rec2.DatacenterRecommendations)
		if dcRec2 == nil {
			return false
		}
		eq := datacenterRecommendationsEquivalent(dcRec1, *dcRec2)
		if !eq {
			return false
		}
	}
	for _, dcRec2 := range rec2.DatacenterRecommendations {
		dcRec1 := findDc(dcRec2.Name, rec1.DatacenterRecommendations)
		if dcRec1 == nil {
			return false
		}
		eq := datacenterRecommendationsEquivalent(*dcRec1, dcRec2)
		if !eq {
			return false
		}
	}
	return true
}

func datacenterRecommendationsEquivalent(rec1, rec2 v1alpha1.DatacenterRecommendations) bool {
	for _, rackRec1 := range rec1.RackRecommendations {
		rackRec2 := findRack(rackRec1.Name, rec2.RackRecommendations)
		if rackRec2 == nil {
			return false
		}
		eq := rackRecommendationsEquivalent(rackRec1, *rackRec2)
		if !eq {
			return false
		}
	}
	for _, rackRec2 := range rec2.RackRecommendations {
		rackRec1 := findRack(rackRec2.Name, rec1.RackRecommendations)
		if rackRec1 == nil {
			return false
		}
		eq := rackRecommendationsEquivalent(*rackRec1, rackRec2)
		if !eq {
			return false
		}
	}
	return true
}

func rackRecommendationsEquivalent(rec1, rec2 v1alpha1.RackRecommendations) bool {
	return rec1.Name == rec2.Name &&
		*rec1.Members == *rec2.Members &&
		rec1.Resources.Requests.Cpu().Cmp(*rec2.Resources.Requests.Cpu()) == 0 &&
		rec1.Resources.Requests.Memory().Cmp(*rec2.Resources.Requests.Memory()) == 0
}

func findDc(dcName string, dcs []v1alpha1.DatacenterRecommendations) *v1alpha1.DatacenterRecommendations {
	for _, dc := range dcs {
		if dcName == dc.Name {
			return &dc
		}
	}
	return nil
}

func findRack(rackName string, rcs []v1alpha1.RackRecommendations) *v1alpha1.RackRecommendations {
	for _, r := range rcs {
		if rackName == r.Name {
			return &r
		}
	}
	return nil
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

func getRackStatus(statusMembers, statusReadyMembers int32) *scyllav1.RackStatus {
	return &scyllav1.RackStatus{
		Members:      statusMembers,
		ReadyMembers: statusReadyMembers,
	}
}

func newSingleDcSCRecommendations(dcName string, rackRecommendations v1alpha1.RackRecommendations) *v1alpha1.ScyllaClusterRecommendations {
	return &v1alpha1.ScyllaClusterRecommendations{
		DatacenterRecommendations: []v1alpha1.DatacenterRecommendations{
			{
				Name: dcName,
				RackRecommendations: []v1alpha1.RackRecommendations{
					rackRecommendations,
				},
			},
		},
	}
}

func newRackRecommendations(rackName, requestCpu, limitCpu, memory string, members int32) *v1alpha1.RackRecommendations {
	return &v1alpha1.RackRecommendations{
		Name:    rackName,
		Members: util.Int32ptr(members),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(requestCpu),
				corev1.ResourceMemory: resource.MustParse(memory),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(limitCpu),
				corev1.ResourceMemory: resource.MustParse(memory),
			},
		},
	}
}

func newRackScalingPolicy(rackName string, rules []v1alpha1.ScalingRule, minAllowedMembers, maxAllowedMembers int32, minAllowedCpu, maxAllowedCpu resource.Quantity, controlledValues v1alpha1.RackControlledValues) *v1alpha1.RackScalingPolicy {
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

func newScalingRule(name string, priority int32, expression string, durFor, durStep *metav1.Duration, mode v1alpha1.ScalingMode, factor float64) *v1alpha1.ScalingRule {
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

func stringMulFloat64(s string, f2 float64) string {
	f1, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic("In recommender unit tests you tried to parse" + s + "to float64 and it resulted in an error: " + err.Error())
	}
	f3 := f1 * f2
	return fmt.Sprintf("%f", f3)
}
