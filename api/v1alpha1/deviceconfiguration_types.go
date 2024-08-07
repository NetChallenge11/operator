/*
Copyright 2024.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeviceConfigurationSpec defines the desired state of DeviceConfiguration
type DeviceConfigurationSpec struct {
	// AlgorithmName specifies the name of the partition decision algorithm
	AlgorithmName string `json:"algorithmName"`

	// ClientClusters is a list of client cluster names that are part of this device configuration
	ClientClusters []string `json:"clientClusters"`

	// ModelName specifies the name of the model used for partition computing
	ModelName string `json:"modelName"`
}

// DeviceConfigurationStatus defines the observed state of DeviceConfiguration
type DeviceConfigurationStatus struct {
	// Ready indicates whether the DeviceConfiguration is ready
	Ready bool `json:"ready"`

	// LastUpdated provides the timestamp of the last status update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// CurrentPhase describes the current phase of the DeviceConfiguration
	CurrentPhase string `json:"currentPhase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DeviceConfiguration is the Schema for the deviceconfigurations API
type DeviceConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeviceConfigurationSpec   `json:"spec,omitempty"`
	Status DeviceConfigurationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeviceConfigurationList contains a list of DeviceConfiguration
type DeviceConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeviceConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeviceConfiguration{}, &DeviceConfigurationList{})
}
