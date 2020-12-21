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
	autoscaling "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
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
	TargetRef *autoscaling.CrossVersionObjectReference `json:"targetRef"`

	// +optional
	UpdatePolicy *UpdatePolicy `json:"updatePolicy,omitempty"`

	// +optional
	ScalingPolicy *ScalingPolicy `json:"scalingPolicy,omitempty"`
}

type UpdatePolicy struct {
	// +optional
	UpdateMode *UpdateMode `json:"updateMode,omitempty"`
}

type ScalingPolicy struct {
	// +optional
	DataCenterScalingPolicies []DataCenterScalingPolicy `json:"dataCenterScalingPolicies,omitempty"`
}

type DataCenterScalingPolicy struct {
	Name string `json:"name"`
	// +optional
	RackScalingPolicies []RackScalingPolicy `json:"rackScalingPolicies,omitempty"`
}

type RackScalingPolicy struct {
	Name string `json:"name"`
	// +optional
	MinMembers *int `json:"minMembers,omitempty"`

	// +optional
	MaxMembers *int `json:"maxMembers,omitempty"`

	// +optional
	RackResourcePolicy *RackResourcePolicy `json:"resourcePolicy,omitempty"`
}

type RackResourcePolicy struct {
	// +optional
	MinAllowed v1.ResourceList `json:"minAllowed,omitempty"`

	// +optional
	MaxAllowed v1.ResourceList `json:"maxAllowed,omitempty"`
}

// +kubebuilder:validation:Enum=Off;Initial;Auto
type UpdateMode string

const (
	UpdateModeOff UpdateMode = "Off"

	UpdateModeInitial UpdateMode = "Initial"

	UpdateModeAuto UpdateMode = "Auto"
)

// ScyllaClusterAutoscalerStatus defines the observed state of ScyllaClusterAutoscaler
type ScyllaClusterAutoscalerStatus struct {
	// +optional
	Recommendations *ScyllaClusterRecommendations `json:"recommendations,omitempty"`
}

type ScyllaClusterRecommendations struct {
	// +optional
	DataCenterRecommendations []DataCenterRecommendations `json:"dataCenterRecommendations,omitempty"`
}

type DataCenterRecommendations struct {
	Name string `json:"name"`
	// +optional
	RackRecommendations []RackRecommendations `json:"rackRecommendations,omitempty"`
}

type RackRecommendations struct {
	Name string `json:"name"`

	Members *RecommendedRackMembers `json:"members,omitempty"`

	Resources *RecommendedRackResources `json:"resources,omitempty"`
}

type RecommendedRackMembers struct {
	Target int `json:"target,omitempty"`
}

type RecommendedRackResources struct {
	Target v1.ResourceList `json:"target,omitempty"`
}

func init() {
	SchemeBuilder.Register(&ScyllaClusterAutoscaler{}, &ScyllaClusterAutoscalerList{})
}
