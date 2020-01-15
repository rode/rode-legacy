package opa

import (
	"context"
	"fmt"
	"os"

	"github.com/liatrio/rode/pkg/common"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"go.uber.org/zap"
)

// Client for using OPA
type Client struct {
	logger *zap.SugaredLogger
	trace  bool
}

// NewClient to create an opa client
func NewClient(logger *zap.SugaredLogger, trace bool) *Client {
	return &Client{
		logger,
		trace,
	}
}

// Evaluate the policy
func (c *Client) Evaluate(moduleName string, moduleBody string, input interface{}) []*common.Violation {
	context := context.TODO()
	violations := make([]*common.Violation, 0, 0)
	compiler, err := ast.CompileModules(map[string]string{
		fmt.Sprintf("%s.rego", moduleName): moduleBody,
	})
	if err != nil {
		c.logger.Errorf("Unable to compile module: %v", err)
		violations = append(violations, common.NewViolation(err))
	}

	var tracer *topdown.BufferTracer
	if c.trace {
		tracer = topdown.NewBufferTracer()
	}
	rego := rego.New(
		rego.Query(fmt.Sprintf("data.%s.violation", moduleName)),
		rego.Compiler(compiler),
		rego.Input(input),
		rego.Tracer(tracer),
	)
	rs, err := rego.Eval(context)
	if err != nil {
		c.logger.Errorf("Unable to evaluate policy: %v", err)
		violations = append(violations, common.NewViolation(err))
	}
	if c.trace {
		topdown.PrettyTrace(os.Stdout, *tracer)
	}

	if len(rs) > 0 {
		for _, v := range rs {
			for _, e := range v.Expressions {
				for _, val := range e.Value.([]interface{}) {
					violations = append(violations, common.NewViolation(val))
				}
			}
		}
	}

	c.logger.Debugf("%v", violations)

	return violations
}
