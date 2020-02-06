package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ConditionStatus string

const (
	ConditionStatusTrue  ConditionStatus = "True"
	ConditionStatusFalse ConditionStatus = "False"
)

type CollectorCondition struct {
	Type               CollectorConditionType `json:"type"`
	Status             ConditionStatus        `json:"status"`
	LastTransitionTime *metav1.Time           `json:"lastTransitionTime,omitempty"`
	Message            string                 `json:"message,omitempty"`
}

type CollectorConditionType string

const (
	CollectorConditionActive CollectorConditionType = "Active"
)
