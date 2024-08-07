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

// PartitionDecisionSpec defines the desired state of PartitionDecision
type PartitionDecisionSpec struct {
	// AlgorithmName specifies the name of the partition decision algorithm
	AlgorithmName string `json:"algorithmName"`

	// ClientClusterName is a list of client cluster names targeted by this partition decision
	ClientClusterName []string `json:"clientClusterName"`

	// ModelName specifies the name of the model used for partition computing
	ModelName string `json:"modelName"`

	// ChannelName specifies the event channel used for notifications
	ChannelName string `json:"channelName"`

	// MetricTypes specifies the types of metrics used in partition computing
	MetricTypes string `json:"metricTypes"`
}

// PartitionDecisionStatus defines the observed state of PartitionDecision
type PartitionDecisionStatus struct {
	// Ready indicates whether the PartitionDecision is ready
	Ready bool `json:"ready"`

	// LastUpdated provides the timestamp of the last status update
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// CurrentPhase describes the current phase of the PartitionDecision
	CurrentPhase string `json:"currentPhase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PartitionDecision is the Schema for the partitiondecisions API
type PartitionDecision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PartitionDecisionSpec   `json:"spec,omitempty"`
	Status PartitionDecisionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PartitionDecisionList contains a list of PartitionDecision
type PartitionDecisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PartitionDecision `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PartitionDecision{}, &PartitionDecisionList{})
}
