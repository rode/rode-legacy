package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ConditionStatus string

const (
	ConditionStatusUnknown ConditionStatus = "Unknown"
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"
)

type Condition struct {
	Type               ConditionType   `json:"type"`
	Status             ConditionStatus `json:"status"`
	LastTransitionTime *metav1.Time    `json:"lastTransitionTime,omitempty"`
	Message            string          `json:"message,omitempty"`
}

type ConditionType string

const (
	ConditionActive   ConditionType = "Active"
	ConditionCompiled ConditionType = "CompiledPolicy"
	ConditionSecret   ConditionType = "CreatedSecret"
)

type Conditioner interface {
	GetConditions() []Condition
}
