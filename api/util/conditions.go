package util

import (
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
)

func SetCollectorCondition(col *rodev1alpha1.Collector, conditionType rodev1alpha1.ConditionType, status rodev1alpha1.ConditionStatus, message string) {
	condition := rodev1alpha1.Condition{
		Type:    conditionType,
		Status:  status,
		Message: message,
	}

	now := metav1.NewTime(clock.RealClock{}.Now())
	condition.LastTransitionTime = &now

	for i, cond := range col.Status.Conditions {
		if cond.Type != condition.Type {
			continue
		}

		if cond.Status == condition.Status {
			condition.LastTransitionTime = cond.LastTransitionTime
		}

		col.Status.Conditions[i] = condition
		return
	}

	col.Status.Conditions = append(col.Status.Conditions, condition)
}

func GetConditionStatus(con Conditioner, conditionType rodev1alpha1.ConditionType) rodev1alpha1.ConditionStatus {
	for _, cond := range con.GetConditions() {
		if cond.Type == conditionType {
			return cond.Status
		}
	}

	return rodev1alpha1.ConditionStatusUnknown
}

type Conditioner interface {
	GetConditions() []rodev1alpha1.Condition
}
