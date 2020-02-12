package util

import (
	rodev1 "github.com/liatrio/rode/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
)

func SetCollectorCondition(col *rodev1.Collector, conditionType rodev1.ConditionType, status rodev1.ConditionStatus, message string) {
	condition := rodev1.Condition{
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
