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

// GlobalReplicasIgnoreSpec defines the desired state of GlobalReplicasIgnore
type GlobalReplicasIgnoreSpec struct {
	// IgnoreNamespaces is a list of namespaces to ignore from scaling
	// +optional
	IgnoreNamespaces []string `json:"ignoreNamespaces,omitempty"`

	// IgnoreResources is a list of specific resources to ignore from scaling
	// +optional
	IgnoreResources []IgnoredResource `json:"ignoreResources,omitempty"`

	// IgnoreLabels is a map of labels that, if present on a resource, will cause it to be ignored
	// +optional
	IgnoreLabels map[string]string `json:"ignoreLabels,omitempty"`
}

// IgnoredResource defines a specific resource to ignore
type IgnoredResource struct {
	// Kind of the resource (e.g., "Deployment")
	// +kubebuilder:validation:Enum=Deployment;StatefulSet
	Kind string `json:"kind"`

	// Name of the resource
	Name string `json:"name"`

	// Namespace of the resource
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// GlobalReplicasIgnoreStatus defines the observed state of GlobalReplicasIgnore
type GlobalReplicasIgnoreStatus struct {
	// IgnoredDeployments contains the list of deployments currently being ignored
	// +optional
	IgnoredDeployments []IgnoredDeployment `json:"ignoredDeployments,omitempty"`

	// LastUpdateTime is the last time the status was updated
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// Conditions represent the latest available observations of the ignore's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// IgnoredDeployment contains information about a deployment being ignored
type IgnoredDeployment struct {
	// Name of the deployment
	Name string `json:"name"`

	// Namespace of the deployment
	Namespace string `json:"namespace"`

	// Reason why this deployment is being ignored
	Reason string `json:"reason"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ignored Namespaces",type="string",JSONPath=".spec.ignoreNamespaces"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// GlobalReplicasIgnore is the Schema for the globalreplicasignores API
type GlobalReplicasIgnore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlobalReplicasIgnoreSpec   `json:"spec,omitempty"`
	Status GlobalReplicasIgnoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GlobalReplicasIgnoreList contains a list of GlobalReplicasIgnore
type GlobalReplicasIgnoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalReplicasIgnore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GlobalReplicasIgnore{}, &GlobalReplicasIgnoreList{})
}
