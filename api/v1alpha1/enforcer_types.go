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
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen=false
type EnforcerInterface interface {
	metav1.ObjectMetaAccessor
	Attesters() []*EnforcerAttester
	Conditioner
	SetCondition(ConditionType, ConditionStatus, string)
	GetConditionStatus(ConditionType) ConditionStatus
}

// EnforcerSpec defines the desired state of Enforcer
type EnforcerSpec struct {
	Attesters []*EnforcerAttester `json:"attesters"`
}

// EnforcerStatus defines the observed state of Enforcer
type EnforcerStatus struct {
	Conditions []Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Enforcer is the Schema for the enforcers API
type Enforcer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnforcerSpec   `json:"spec,omitempty"`
	Status EnforcerStatus `json:"status,omitempty"`
}

func (in *Enforcer) Attesters() []*EnforcerAttester {
	return in.Spec.Attesters
}

func (in *Enforcer) SetConditions(conditions []Condition) {
	in.Status.Conditions = conditions
}

func (in *Enforcer) GetConditions() []Condition {
	return in.Status.Conditions
}

func (in *Enforcer) SetCondition(conditionType ConditionType, conditionStatus ConditionStatus, message string) {
	SetCondition(in, conditionType, conditionStatus, message)
}

func (in *Enforcer) GetConditionStatus(conditionType ConditionType) ConditionStatus {
	return GetConditionStatus(in, conditionType)
}

// +kubebuilder:object:root=true

// EnforcerList contains a list of Enforcer
type EnforcerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Enforcer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Enforcer{}, &EnforcerList{})
}

type EnforcerAttester struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (in EnforcerAttester) String() string {
	return fmt.Sprintf("%s/%s", in.Namespace, in.Name)
}
