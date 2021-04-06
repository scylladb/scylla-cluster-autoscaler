/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// The below package contains a CRD of ScyllaClusterAutoscaler API object.
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:docs-gen:collapse=Imports

// +kubebuilder:object:root=true
// ScyllaClusterAutoscalerList contains a list of ScyllaClusterAutoscalers.
type ScyllaClusterAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaClusterAutoscaler `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// ScyllaClusterAutoscaler is the Schema for the ScyllaClusterAutoscalers API.
type ScyllaClusterAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScyllaClusterAutoscalerSpec `json:"spec,omitempty"`

	// +optional
	Status ScyllaClusterAutoscalerStatus `json:"status,omitempty"`
}

// ScyllaClusterAutoscalerSpec defines the desired state of ScyllaClusterAutoscaler.
type ScyllaClusterAutoscalerSpec struct {
	// TargetRef references a ScyllaCluster that is subject to autoscaling.
	// The target is uniquely described by its name and namespace.
	TargetRef *TargetRef `json:"targetRef"`

	// UpdatePolicy describes the rules and limitations of how the target is meant to be updated.
	// If not specified, updateMode is set to "Auto".
	// +optional
	// +kubebuilder:default:={"updateMode":"Auto"}
	UpdatePolicy *UpdatePolicy `json:"updatePolicy,omitempty"`

	// ScalingPolicy determines how each rack is supposed to be scaled.
	// Every rack's policy is described separately.
	// If a rack is not described, it will not undergo autoscaling.
	// +optional
	ScalingPolicy *ScalingPolicy `json:"scalingPolicy,omitempty"`
}

type TargetRef struct {
	Namespace string `json:"namespace"`

	Name string `json:"name"`
}

type UpdatePolicy struct {
	// Determines whether the recommendations are applied to the target. Set to "Auto" by default.
	// +optional
	// +kubebuilder:default:=Auto
	UpdateMode UpdateMode `json:"updateMode"`

	// Describes how long the recommendations is valid for after having been saved in a status.
	// If left blank, recommendations do not expire.
	// +optional
	RecommendationExpirationTime *metav1.Duration `json:"recommendationExpirationTime,omitempty"`

	// Determines the length of a period after updating the ScyllaCluster during which no other recommendations should be applied.
	// If left blank, there is no cooldown period.
	// +optional
	UpdateCooldown *metav1.Duration `json:"updateCooldown,omitempty"`
}

// +kubebuilder:validation:Enum=Off;Auto
type UpdateMode string

const (
	// UpdateModeOff means that the recommendations are provided but never applied.
	UpdateModeOff UpdateMode = "Off"

	// UpdateModeAuto means that the recommendations are applied periodically.
	UpdateModeAuto UpdateMode = "Auto"
)

type ScalingPolicy struct {
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Datacenters []DatacenterScalingPolicy `json:"datacenters,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type DatacenterScalingPolicy struct {
	// Name of a datacenter subject to autoscaling.
	Name string `json:"name"`

	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	RackScalingPolicies []RackScalingPolicy `json:"racks,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type RackScalingPolicy struct {
	// Name of a rack subject to autoscaling.
	Name string `json:"name"`

	// MemberPolicy describes the limitations on scaling the rack's members.
	// +optional
	MemberPolicy *RackMemberPolicy `json:"memberPolicy,omitempty"`

	// ResourcePolicy determines the constraints on scaling the rack's resources.
	// +optional
	ResourcePolicy *RackResourcePolicy `json:"resourcePolicy,omitempty"`

	// ScalingRules are a mechanism allowing for describing how a given rack is meant to be scaled.
	// A single rule is essentially a tuple of a boolean query and the action to be invoked when query evaluates to true
	// at a point or a certain period of time, depending on whether the query is ranged or not.
	// A query is only checked at the time of evaluation.
	// A ranged query is checked against a specified time range with a predetermined frequency and it only evaluates
	// to true if the condition is met at all points in the time series.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ScalingRules []ScalingRule `json:"rules,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type RackMemberPolicy struct {
	// The lowest allowed number of members.
	// The number of rack's members will never go below this value.
	// If not set, there is no minimum.
	// +optional
	MinAllowed *int32 `json:"minAllowed,omitempty"`

	// The highest allowed number of members.
	// The number of rack's members will never go above this value.
	// If not set, there is no maximum.
	// +optional
	MaxAllowed *int32 `json:"maxAllowed,omitempty"`
}

type RackResourcePolicy struct {
	// The smallest allowed resource quantities.
	// Rack's resources will never go below these values.
	// If not set, there is no minimum.
	// +optional
	MinAllowedCpu *resource.Quantity `json:"minAllowedCpu,omitempty"`

	// The largest allowed resource quantities.
	// Rack's resources will never go above these values.
	// If not set, there is no maximum.
	// +optional
	MaxAllowedCpu *resource.Quantity `json:"maxAllowedCpu,omitempty"`

	// Specifies which resource values should be scaled.
	// Defaults to "RequestsAndLimits".
	// +optional
	// +kubebuilder:default:=RequestsAndLimits
	RackControlledValues RackControlledValues `json:"controlledValues"`
}

// +kubebuilder:validation:Enum=Requests;RequestsAndLimits
type RackControlledValues string

const (
	// RackControlledValuesRequests says that only the requests will be scaled.
	RackControlledValuesRequests RackControlledValues = "Requests"

	// RackControlledValuesRequestsAndLimits says that both the requests and the limits will be scaled.
	RackControlledValuesRequestsAndLimits RackControlledValues = "RequestsAndLimits"
)

type ScalingRule struct {
	// A unique name of the scaling rule.
	Name string `json:"name"`

	// Priorities are used to determine which rule is to be applied in case of multiple expressions evaluating to true at once.
	// A rule with the lowest priority is chosen over the others.
	// For triggered rules with equal priority, their top to bottom order of appearance decides.
	// +kubebuilder:validation:Minimum=0
	Priority int32 `json:"priority"`

	// A boolean query to the monitoring service.
	Expression string `json:"expression"`

	// Describes the duration of a ranged query.
	// If not set, the query is not ranged.
	// +optional
	For *metav1.Duration `json:"for"`

	// Specifies the minimal time period between subsequent points in the time series.
	// Only applies for ranged queries.
	// +optional
	Step *metav1.Duration `json:"step"`

	// ScalingMode specifies the direction of scaling.
	ScalingMode ScalingMode `json:"mode"`

	// ScalingFactor describes the factor by which the scaled value will be multiplied.
	ScalingFactor float64 `json:"factor"`
}

// +kubebuilder:validation:Enum=Horizontal;Vertical
type ScalingMode string

const (
	// ScalingModeHorizontal means the target will be scaled horizontally by appending new members.
	ScalingModeHorizontal ScalingMode = "Horizontal"

	// ScalingModeHorizontal means the target will be scaled vertically by making more resources available for its operation.
	ScalingModeVertical ScalingMode = "Vertical"
)

// ScyllaClusterAutoscalerStatus defines the observed state of ScyllaClusterAutoscaler
type ScyllaClusterAutoscalerStatus struct {
	// LastUpdated specifies the timestamp of last saved recommendations.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// LastApplied specifies the timestamp of last applied recommendations.
	// +optional
	LastApplied *metav1.Time `json:"lastApplied,omitempty"`

	// UpdateStatus specifies the result of the latest attempt at preparing and saving recommendations.
	// +optional
	UpdateStatus *UpdateStatus `json:"updateStatus,omitempty"`

	// Latest recommendations for the target.
	// +optional
	Recommendations *ScyllaClusterRecommendations `json:"recommendations,omitempty"`
}

// +kubebuilder:validation:Enum=Ok;TargetFetchFail;TargetNotReady;RecommendationsFail
type UpdateStatus string

const (
	// UpdateStatusOk means that the recommendations were prepared successfully.
	UpdateStatusOk UpdateStatus = "Ok"

	// UpdateStatusTargetFetchFail says that the target ScyllaCluster could not be fetched.
	UpdateStatusTargetFetchFail UpdateStatus = "TargetFetchFail"

	// UpdateStatusTargetNotReady says that the target was reachable but unstable.
	UpdateStatusTargetNotReady UpdateStatus = "TargetNotReady"

	// UpdateStatusRecommendationsFail says that preparing recommendations resulted in an error.
	UpdateStatusRecommendationsFail UpdateStatus = "RecommendationsFail"
)

type ScyllaClusterRecommendations struct {
	// +optional
	DatacenterRecommendations []DatacenterRecommendations `json:"datacenterRecommendations,omitempty"`
}

type DatacenterRecommendations struct {
	// Name of a datacenter.
	Name string `json:"name"`

	// +optional
	RackRecommendations []RackRecommendations `json:"rackRecommendations,omitempty"`
}

type RackRecommendations struct {
	// Name of a rack.
	Name string `json:"name"`

	// Recommended number of members.
	// +optional
	Members *int32 `json:"members,omitempty"`

	// Recommended resources.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ScyllaClusterAutoscaler{}, &ScyllaClusterAutoscalerList{})
}
