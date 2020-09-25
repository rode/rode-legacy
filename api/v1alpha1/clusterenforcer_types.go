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

type MatchStrategy string

const (
	// IncludeMatchStrategy Enforce attestations in namespaces matching list
	IncludeMatchStrategy MatchStrategy = "Include"
	// ExcludematchStrategy Enforce attestation in namespaces not matching list
	ExcludematchStrategy MatchStrategy = "Exclude"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterEnforcerSpec defines the desired state of ClusterEnforcer
type ClusterEnforcerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Namespaces    []string            `json:"namespaces,omitempty"`
	MatchStrategy MatchStrategy       `json:"matchStrategy,omitempty"`
	Attesters     []*EnforcerAttester `json:"attesters"`
}

// ClusterEnforcerStatus defines the observed state of ClusterEnforcer
type ClusterEnforcerStatus struct {
	Conditions []Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterEnforcer is the Schema for the clusterenforcers API
type ClusterEnforcer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterEnforcerSpec   `json:"spec,omitempty"`
	Status ClusterEnforcerStatus `json:"status,omitempty"`
}

func (ce *ClusterEnforcer) Attesters() []*EnforcerAttester {
	return ce.Spec.Attesters
}

func (ce *ClusterEnforcer) SetConditions(conditions []Condition) {
	ce.Status.Conditions = conditions
}

func (ce *ClusterEnforcer) GetConditions() []Condition {
	return ce.Status.Conditions
}

func (ce *ClusterEnforcer) SetCondition(conditionType ConditionType, conditionStatus ConditionStatus, message string) {
	SetCondition(ce, conditionType, conditionStatus, message)
}

func (ce *ClusterEnforcer) GetConditionStatus(conditionType ConditionType) ConditionStatus {
	return GetConditionStatus(ce, conditionType)
}

func (ce *ClusterEnforcer) EnforcesNamespace(namespace string) bool {
	for _, ceNamespace := range ce.Spec.Namespaces {
		if ceNamespace == namespace {
			return ce.Spec.MatchStrategy == IncludeMatchStrategy
		}
	}
	return ce.Spec.MatchStrategy != IncludeMatchStrategy
}

// +kubebuilder:object:root=true

// ClusterEnforcerList contains a list of ClusterEnforcer
type ClusterEnforcerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterEnforcer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterEnforcer{}, &ClusterEnforcerList{})
}
