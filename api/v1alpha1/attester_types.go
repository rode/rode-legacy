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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"CompiledPolicy\")].message",priority=1
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"CompiledPolicy\")].status",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
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

// AttesterStatus defines the observed state of Attester
type AttesterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
}

func init() {
	SchemeBuilder.Register(&Attester{}, &AttesterList{})
}
