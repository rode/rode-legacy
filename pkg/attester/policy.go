package attester

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
)

type policy struct {
	name     string
	module   string
	trace    bool
	compiler *ast.Compiler
}

// Policy is the interface for managing policy
type Policy interface {
	Evaluate(context.Context, interface{}) []*Violation
	Serialize(out io.Writer) error
}

// NewPolicy creates a new policy
func NewPolicy(name string, module string, trace bool) (Policy, error) {
	compiler, err := ast.CompileModules(map[string]string{
		fmt.Sprintf("%s.rego", name): module,
	})
	if err != nil {
		return nil, err
	}
	return &policy{
		name,
		module,
		trace,
		compiler,
	}, nil
}

// ReadPolicy creates a signer from reader
func ReadPolicy(in io.Reader) (Policy, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

// Evaluate the policy
func (p *policy) Evaluate(context context.Context, input interface{}) []*Violation {
	violations := make([]*Violation, 0)

	var tracer *topdown.BufferTracer
	if p.trace {
		tracer = topdown.NewBufferTracer()
	}
	rego := rego.New(
		rego.Query(fmt.Sprintf("data.%s.violation", p.name)),
		rego.Compiler(p.compiler),
		rego.Input(input),
		rego.Tracer(tracer),
	)
	rs, err := rego.Eval(context)
	if err != nil {
		violations = append(violations, NewViolation(err))
	}
	if p.trace {
		topdown.PrettyTrace(os.Stdout, *tracer)
	}

	if len(rs) > 0 {
		for _, v := range rs {
			for _, e := range v.Expressions {
				for _, val := range e.Value.([]interface{}) {
					violations = append(violations, NewViolation(val))
				}
			}
		}
	}

	return violations
}

func (p *policy) Serialize(out io.Writer) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}
