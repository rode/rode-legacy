package opa

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"go.uber.org/zap"
)

// Client for using OPA
type Client struct {
	logger *zap.SugaredLogger
}

// NewClient to create an opa client
func NewClient(logger *zap.SugaredLogger) *Client {
	return &Client{
		logger,
	}
}

// Evaluate the policy
func (c *Client) Evaluate(moduleName string, moduleBody string, input interface{}) []*Violation {
	ctx := context.TODO()
	compiler, err := ast.CompileModules(map[string]string{
		fmt.Sprintf("%s.rego", moduleName): moduleBody,
	})

	buf := topdown.NewBufferTracer()
	rego := rego.New(
		rego.Query(fmt.Sprintf("data.%s.violation", moduleName)),
		rego.Compiler(compiler),
		rego.Input(input),
		//rego.Tracer(buf),
	)
	rs, err := rego.Eval(ctx)
	if err != nil {
		c.logger.Errorf("Unable to evaluate policy: %v", err)
	}
	topdown.PrettyTrace(os.Stdout, *buf)

	violations := make([]*Violation, 0, 0)
	if len(rs) > 0 {
		for _, v := range rs {
			for _, e := range v.Expressions {
				for _, val := range e.Value.([]interface{}) {
					violations = append(violations, &Violation{
						Raw: val,
					})
				}
			}
		}
	}

	c.logger.Debugf("%v", violations)

	return violations
}

// Violation describes a violation
type Violation struct {
	Raw     interface{}
	Msg     string
	Details map[string]interface{}
}
