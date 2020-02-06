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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",priority=1
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"CompiledPolicy\")].status",description=""
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Attester is the Schema for the attesters API
type Attester struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AttesterSpec   `json:"spec,omitempty"`
	Status AttesterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AttesterList contains a list of Attester
type AttesterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Attester `json:"items"`
}

// AttesterSpec defines the desired state of Attester
type AttesterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// PgpSecret defines the name of the secret to use for signing. If the secret doesn't already exist it will be created.
	// +optional
	PgpSecret string `json:"pgpSecret"`
	// Policy defines the Rego policy that the attester will attest adherance to.
	Policy string `json:"policy"`
}

// ConditionStatus represents a condition's status.
// +kubebuilder:validation:Enum=True;False;Unknown
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in
// the condition; "ConditionFalse" means a resource is not in the condition;
// "ConditionUnknown" means kubernetes can't decide if a resource is in the
// condition or not. In the future, we could add other intermediate
// conditions, e.g. ConditionDegraded.
const (
	// ConditionTrue represents the fact that a given condition is true
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse represents the fact that a given condition is false
	ConditionFalse ConditionStatus = "False"

	// ConditionUnknown represents the fact that a given condition is unknown
	ConditionUnknown ConditionStatus = "Unknown"
)

// AttesterStatus defines the observed state of Attester
type AttesterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

    // +optional
	Conditions []AttesterCondition `json:"conditions,omitempty"`
}

// AttesterCondition contains condition information for an Attester.
type AttesterCondition struct {
	// Type of the condition, currently ('Ready').
	Type AttesterConditionType `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	// +optional
	Message string `json:"message,omitempty"`
}

// AttesterConditionType represents an Attester condition value.
type AttesterConditionType string

const (
	// AttesterConditionReady indicates that an attester is ready for use.
    // TODO: Change this definition to what ready should mean
	// This is defined as:
	// - The target secret exists
	// - The target secret contains a certificate that has not expired
	// - The target secret contains a private key valid for the certificate
	// - The commonName and dnsNames attributes match those specified on the Certificate
	AttesterConditionReady AttesterConditionType = "Ready"
    AttesterConditionCompiled AttesterConditionType = "CompiledPolicy"
    AttesterConditionSecret AttesterConditionType = "CreatedSecret"
)

func init() {
	SchemeBuilder.Register(&Attester{}, &AttesterList{})
}
