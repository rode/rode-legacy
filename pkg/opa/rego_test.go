package opa

import (
	"encoding/json"
	"testing"

	"github.com/liatrio/rode/pkg/common"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestClient_Evaluate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewClient(logger.Sugar(), true)

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

var emptyOccurrences = `{"occurrences": []}`
var successDiscoveryOccurrences = `{"occurrences": [
	{
		"name": "projects/rode/occurrences/353024b2-b849-44d2-a2b1-b12b2f492a3e",
		"resource": {
			"uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
		},
		"noteName": "projects/rode/notes/ecr_events",
		"createTime": "2020-01-15T17:09:10.476349Z",
		"updateTime": "2020-01-15T17:09:10.476349Z",
		"discovered": {
			"discovered": {
				"analysisStatus": "FINISHED_SUCCESS"
			}
		}
	},
	{
		"name": "projects/rode/occurrences/f4347692-a0d0-48e3-bc0d-94efd2a840d6",
		"resource": {
			"uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
		},
		"noteName": "projects/rode/notes/ecr_events"
	}
]}`
var failDiscoveryOccurrences = `{"occurrences": [
	{
		"name": "projects/rode/occurrences/353024b2-b849-44d2-a2b1-b12b2f492a3e",
		"resource": {
			"uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
		},
		"noteName": "projects/rode/notes/ecr_events",
		"createTime": "2020-01-15T17:09:10.476349Z",
		"updateTime": "2020-01-15T17:09:10.476349Z",
		"discovered": {
			"discovered": {
				"analysisStatus": "FINISHED_FAILED"
			}
		}
	},
	{
		"name": "projects/rode/occurrences/f4347692-a0d0-48e3-bc0d-94efd2a840d6",
		"resource": {
			"uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
		},
		"noteName": "projects/rode/notes/ecr_events"
	}
]}`
var highVuln = `{"occurrences": [
		{
			"name": "projects/rode/occurrences/353024b2-b849-44d2-a2b1-b12b2f492a3e",
			"resource": {
				"uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
			},
			"noteName": "projects/rode/notes/ecr_events",
			"createTime": "2020-01-15T17:09:10.476349Z",
			"updateTime": "2020-01-15T17:09:10.476349Z",
			"discovered": {
				"discovered": {
					"analysisStatus": "FINISHED_SUCCESS"
				}
			}
		},
    {
      "name": "projects/rode/occurrences/f4347692-a0d0-48e3-bc0d-94efd2a840d6",
      "resource": {
        "uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
      },
      "noteName": "projects/rode/notes/ecr_events",
      "createTime": "2020-01-15T17:09:10.476821800Z",
      "updateTime": "2020-01-15T17:09:10.476821800Z",
      "vulnerability": {
        "severity": "HIGH",
        "packageIssue": [
          {
            "affectedLocation": {
              "cpeUri": "foo",
              "package": "foo",
              "version": {
                "name": "foo",
                "kind": "NORMAL"
              }
            }
          }
        ]
      }
    }
]}`
var lowVuln = `{"occurrences": [
		{
			"name": "projects/rode/occurrences/353024b2-b849-44d2-a2b1-b12b2f492a3e",
			"resource": {
				"uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
			},
			"noteName": "projects/rode/notes/ecr_events",
			"createTime": "2020-01-15T17:09:10.476349Z",
			"updateTime": "2020-01-15T17:09:10.476349Z",
			"discovered": {
				"discovered": {
					"analysisStatus": "FINISHED_SUCCESS"
				}
			}
		},
    {
      "name": "projects/rode/occurrences/f4347692-a0d0-48e3-bc0d-94efd2a840d6",
      "resource": {
        "uri": "489130170427.dkr.ecr.us-east-1.amazonaws.com/springtrader:foo@sha256:b88ca0f4a7fe2c67fc8bdb67ad405f418ca0be6aec442c79ae32253138da2dd5"
      },
      "noteName": "projects/rode/notes/ecr_events",
      "createTime": "2020-01-15T17:09:10.476821800Z",
      "updateTime": "2020-01-15T17:09:10.476821800Z",
      "vulnerability": {
        "severity": "LOW",
        "packageIssue": [
          {
            "affectedLocation": {
              "cpeUri": "foo",
              "package": "foo",
              "version": {
                "name": "foo",
                "kind": "NORMAL"
              }
            }
          }
        ]
      }
    }
]}`

var attenstationRego = `
package default_attester
violation[{"msg":"analysis failed"}]{
	input.occurrences[_].discovered.discovered.analysisStatus != "FINISHED_SUCCESS"
}
violation[{"msg":"analysis not performed"}]{
	analysisStatus := [s | s := input.occurrences[_].discovered.discovered.analysisStatus]
	count(analysisStatus) = 0
}
violation[{"msg":"critical vulnerability found"}]{
	severityCount("CRITICAL") > 0
}
violation[{"msg":"high vulnerability found"}]{
	severityCount("HIGH") > 0
}
severityCount(severity) = cnt {
	cnt := count([v | v := input.occurrences[_].vulnerability.severity; v == severity])
}
`

func TestClient_EvaluateGrafeas(t *testing.T) {
	assert := assert.New(t)

	res := evalAttenstationRego(failDiscoveryOccurrences)
	assert.NotEmpty(res, "evaluation")

	res = evalAttenstationRego(emptyOccurrences)
	assert.NotEmpty(res, "evaluation")

	res = evalAttenstationRego(successDiscoveryOccurrences)
	assert.Empty(res, "evaluation")

	res = evalAttenstationRego(highVuln)
	assert.NotEmpty(res, "evaluation")

	res = evalAttenstationRego(lowVuln)
	assert.Empty(res, "evaluation")
}

func evalAttenstationRego(occurrencesJSON string) []*common.Violation {
	logger, _ := zap.NewDevelopment()
	c := NewClient(logger.Sugar(), true)
	listOccurrences := make(map[string]interface{})
	err := json.Unmarshal([]byte(occurrencesJSON), &listOccurrences)
	if err != nil {
		panic(err)
	}
	return c.Evaluate("default_attester", attenstationRego, listOccurrences)
}
