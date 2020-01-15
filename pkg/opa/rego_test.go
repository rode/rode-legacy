package opa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestClient_Evaluate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewClient(logger.Sugar())

	module := `
package mytest

violation[{"msg":"v1"}]{
	input.foo = "bar"
}
violation[{"msg": "v2"}] {
	input.a = "z"
}
`
	assert := assert.New(t)

	input := map[string]string{
		"foo": "no",
		"a":   "no",
	}

	res := c.Evaluate("mytest", module, input)
	assert.Empty(res, "evaluation")

	input2 := map[string]string{
		"foo": "bar",
		"a":   "z",
	}

	res = c.Evaluate("mytest", module, input2)
	assert.NotEmpty(res, "evaluation")
}
