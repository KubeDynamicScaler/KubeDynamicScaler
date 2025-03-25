/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ReplicasOverrideSpec defines the desired state of ReplicasOverride
type ReplicasOverrideSpec struct {
	// Selector defines how to find Deployments to scale.
	// Only one of the following selector types should be specified.
	// +optional
	Selector *TargetSelector `json:"selector,omitempty"`

	// DeploymentRef allows direct reference to a specific deployment.
	// +optional
	DeploymentRef *DeploymentReference `json:"deploymentRef,omitempty"`

	// HPARef allows direct reference to a specific HPA.
	// +optional
	HPARef *HPAReference `json:"hpaRef,omitempty"`

	// OverrideType specifies how the scaling should be applied.
	// Valid values are "override" or "additive".
	// +kubebuilder:validation:Enum=override;additive
	// +kubebuilder:default:=override
	OverrideType string `json:"overrideType"`

	// ReplicasPercentage specifies the percentage to scale the replicas.
	// For example: 150 means 150% of the original replicas.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default:=100
	ReplicasPercentage int32 `json:"replicasPercentage"`

	// MinReplicas specifies the minimum number of replicas allowed.
	// If not specified, the global minReplicas from the config will be used.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas specifies the maximum number of replicas allowed.
	// If not specified, the global maxReplicas from the config will be used.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

// TargetSelector defines how to select deployments for scaling
type TargetSelector struct {
	// MatchLabels is a map of {key,value} pairs to select deployments
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// DeploymentReference contains information to select a specific deployment
type DeploymentReference struct {
	// Name of the deployment
	Name string `json:"name"`

	// Namespace of the deployment
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// HPAReference contains information to select a specific HPA
type HPAReference struct {
	// Name of the HPA
	Name string `json:"name"`

	// Namespace of the HPA
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ReplicasOverrideStatus defines the observed state of ReplicasOverride
type ReplicasOverrideStatus struct {
	// AffectedDeployments contains the list of deployments affected by this override
	// +optional
	AffectedDeployments []AffectedDeployment `json:"affectedDeployments,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions represent the latest available observations of the override's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AffectedDeployment contains information about a deployment affected by the override
type AffectedDeployment struct {
	// Name of the deployment
	Name string `json:"name"`

	// Namespace of the deployment
	Namespace string `json:"namespace"`

	// OriginalReplicas is the number of replicas before the override
	OriginalReplicas int32 `json:"originalReplicas"`

	// CurrentReplicas is the current number of replicas after the override
	CurrentReplicas int32 `json:"currentReplicas"`

	// CurrentPercentage is the current percentage applied
	CurrentPercentage int32 `json:"currentPercentage"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.overrideType"
// +kubebuilder:printcolumn:name="Percentage",type="integer",JSONPath=".spec.replicasPercentage"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ReplicasOverride is the Schema for the replicasoverrides API
type ReplicasOverride struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicasOverrideSpec   `json:"spec,omitempty"`
	Status ReplicasOverrideStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReplicasOverrideList contains a list of ReplicasOverride
type ReplicasOverrideList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicasOverride `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReplicasOverride{}, &ReplicasOverrideList{})
}
