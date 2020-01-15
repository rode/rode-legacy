package common

import (
	"fmt"
)

// PolicyEvaluator runs policy
type PolicyEvaluator interface {
	Evaluate(moduleName string, moduleBody string, input interface{}) []*Violation
}

// Violation describes a violation
type Violation struct {
	Raw     interface{}
	Msg     string
	Details map[string]interface{}
}

// NewViolation creates new violation from raw val
func NewViolation(raw interface{}) *Violation {
	v := new(Violation)
	v.Raw = raw

	if rawError, ok := raw.(error); ok {
		v.Msg = rawError.Error()
	} else if rawMap, ok := raw.(map[string]interface{}); ok {
		rawMsg := rawMap["msg"]
		if rawMsg != nil {
			v.Msg = rawMap["msg"].(string)
		}
		rawDetails := rawMap["details"]
		if rawDetails != nil {
			v.Details = rawMap["details"].(map[string]interface{})
		}
	}

	return v
}

func (v *Violation) String() string {
	return fmt.Sprintf("%s %v", v.Msg, v.Details)
}
