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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:docs-gen:collapse=Imports

// +kubebuilder:object:root=true
// ScyllaClusterAutoscalerList contains a list of ScyllaClusterAutoscaler
type ScyllaClusterAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScyllaClusterAutoscaler `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// ScyllaClusterAutoscaler is the Schema for the ScyllaClusterAutoscalers API
type ScyllaClusterAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ScyllaClusterAutoscalerSpec `json:"spec,omitempty"`

	// +optional
	Status ScyllaClusterAutoscalerStatus `json:"status,omitempty"`
}

// ScyllaClusterAutoscalerSpec defines the desired state of ScyllaClusterAutoscaler
type ScyllaClusterAutoscalerSpec struct {
	TargetRef *TargetRef `json:"targetRef"`

	// +optional
	// +kubebuilder:default:={"updateMode":"Auto"}
	UpdatePolicy *UpdatePolicy `json:"updatePolicy,omitempty"`

	// +optional
	ScalingPolicy *ScalingPolicy `json:"scalingPolicy,omitempty"`
}

type TargetRef struct {
	Namespace string `json:"namespace"`

	Name string `json:"name"`
}

type UpdatePolicy struct {
	// +optional
	// +kubebuilder:default:=Auto
	UpdateMode UpdateMode `json:"updateMode"`
}

// +kubebuilder:validation:Enum=Off;Auto
type UpdateMode string

const (
	UpdateModeOff UpdateMode = "Off"

	UpdateModeOn UpdateMode = "Auto"
)

type ScalingPolicy struct {
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Datacenters []DatacenterScalingPolicy `json:"datacenters,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type DatacenterScalingPolicy struct {
	Name string `json:"name"`

	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	RackScalingPolicies []RackScalingPolicy `json:"racks,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type RackScalingPolicy struct {
	Name string `json:"name"`

	// +optional
	MemberPolicy *RackMemberPolicy `json:"memberPolicy,omitempty"`

	// +optional
	ResourcePolicy *RackResourcePolicy `json:"resourcePolicy,omitempty"`

	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	ScalingRules []ScalingRule `json:"rules,omitempty" patchStrategy:"merge" patchMergeKey:"name"`
}

type RackMemberPolicy struct {
	// +optional
	MinAllowed *int32 `json:"minAllowed,omitempty"`

	// +optional
	MaxAllowed *int32 `json:"maxAllowed,omitempty"`
}

type RackResourcePolicy struct {
	// +optional
	MinAllowedCpu *resource.Quantity `json:"minAllowedCpu,omitempty"`

	// +optional
	MaxAllowedCpu *resource.Quantity `json:"maxAllowedCpu,omitempty"`
}

type ScalingRule struct {
	Name string `json:"name"`

	// +kubebuilder:validation:Minimum=0
	Priority int32 `json:"priority"`

	Expression string `json:"expression"`

	// +optional
	For *string `json:"for"` // TODO add format validation for time.Duration

	ScalingMode ScalingMode `json:"mode"`

	ScalingFactor float64 `json:"factor"`
}

// +kubebuilder:validation:Enum=Horizontal;Vertical
type ScalingMode string

const (
	ScalingModeHorizontal ScalingMode = "Horizontal"

	ScalingModeVertical ScalingMode = "Vertical"
)

// ScyllaClusterAutoscalerStatus defines the observed state of ScyllaClusterAutoscaler
type ScyllaClusterAutoscalerStatus struct {
	// +optional
	Recommendations *ScyllaClusterRecommendations `json:"recommendations,omitempty"`
}

type ScyllaClusterRecommendations struct {
	// +optional
	DatacenterRecommendations []DatacenterRecommendations `json:"datacenterRecommendations,omitempty"`
}

type DatacenterRecommendations struct {
	Name string `json:"name"`

	// +optional
	RackRecommendations []RackRecommendations `json:"rackRecommendations,omitempty"`
}

type RackRecommendations struct {
	Name string `json:"name"`

	// +optional
	Members *RecommendedRackMembers `json:"members,omitempty"`

	// +optional
	Resources *RecommendedRackResources `json:"resources,omitempty"`
}

type RecommendedRackMembers struct {
	Target int32 `json:"target"`
}

type RecommendedRackResources struct {
	TargetCPU resource.Quantity `json:"targetCpu"`
}

func init() {
	SchemeBuilder.Register(&ScyllaClusterAutoscaler{}, &ScyllaClusterAutoscalerList{})
}
