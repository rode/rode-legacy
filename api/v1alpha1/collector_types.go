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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CollectorAWSConfig defines configuration for ECR type collectors.
type CollectorECRConfig struct {
	// Denotes the name of the AWS SQS queue to collect events from.
	QueueName string `json:"queueName,omitempty"`
}

// CollectorSpec defines the desired state of Collector
type CollectorSpec struct {
	// Type defines the type of collector that this is. Supported values are ecr_event, test
	CollectorType string `json:"type"`
	// Defines configuration for collectors of the ecr_event type.
	// +optional
	ECR CollectorECRConfig `json:"ecr,omitempty"`
}

// CollectorStatus defines the observed state of Collector
type CollectorStatus struct {
	Conditions []Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Active\")].status",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Collector is the Schema for the collectors API
type Collector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CollectorSpec   `json:"spec,omitempty"`
	Status CollectorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CollectorList contains a list of Collector
type CollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Collector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Collector{}, &CollectorList{})
}

func (c *Collector) GetConditions() []Condition {
	return c.Status.Conditions
}
