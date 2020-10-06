package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
)

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
	ConditionActive      ConditionType = "Active"
	ConditionCompiled    ConditionType = "Policy"
	ConditionSecret      ConditionType = "Key"
	ConditionInitialized ConditionType = "Stream"
	ConditionListener    ConditionType = "Listener"
)

// +k8s:deepcopy-gen=false
type Conditioner interface {
	GetConditions() []Condition
	SetConditions([]Condition)
}

func SetCondition(con Conditioner, conditionType ConditionType, status ConditionStatus, message string) {
	condition := Condition{
		Type:    conditionType,
		Status:  status,
		Message: message,
	}

	now := metav1.NewTime(clock.RealClock{}.Now())
	condition.LastTransitionTime = &now

	conditions := con.GetConditions()
	var result []Condition // nolint: prealloc
	conditionModified := false

	for _, cond := range conditions {
		if cond.Type != condition.Type {
			result = append(result, cond)
			continue
		}

		if cond.Status == condition.Status {
			condition.LastTransitionTime = cond.LastTransitionTime
		}

		result = append(result, condition)

		conditionModified = true
	}

	if !conditionModified {
		result = append(result, condition)
	}

	con.SetConditions(result)
}

func GetConditionStatus(con Conditioner, conditionType ConditionType) ConditionStatus {
	for _, cond := range con.GetConditions() {
		if cond.Type == conditionType {
			return cond.Status
		}
	}

	return ConditionStatusUnknown
}
