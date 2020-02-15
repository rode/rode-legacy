package controllers

import (
	"github.com/liatrio/rode/api/util"
	rodev1alpha1 "github.com/liatrio/rode/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func containsFinalizer(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func removeFinalizer(slice []string, str string) []string {
	result := make([]string, 0)

	for _, s := range slice {
		if s == str {
			continue
		}

		result = append(result, s)
	}

	return result
}

func ignoreFinalizerUpdate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObjectMeta := e.MetaOld
			newObjectMeta := e.MetaNew

			// NO enqueue whenever a finalizer is added or removed
			return !containsFinalizer(oldObjectMeta.GetFinalizers(), collectorFinalizerName) != containsFinalizer(newObjectMeta.GetFinalizers(), collectorFinalizerName)
		},
	}
}

func ignoreDelete() predicate.Predicate {
	return predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
}

type conditionerConverter func(o runtime.Object) util.Conditioner

func ignoreConditionStatusUpdateToActive(cc conditionerConverter, ct rodev1alpha1.ConditionType) predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConditionStatus := util.GetConditionStatus(cc(e.ObjectOld), ct)
			newConditionStatus := util.GetConditionStatus(cc(e.ObjectNew), ct)
			if oldConditionStatus != newConditionStatus && newConditionStatus == rodev1alpha1.ConditionStatusTrue {
				// NO enqueue request
				return false
			}
			// ENQUEUE request
			return true
		},
	}
}
