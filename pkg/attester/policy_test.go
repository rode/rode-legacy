package attester

import (
	"context"
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("policy", func() {
	var (
		ctx context.Context
		policy string
	)

	BeforeEach(func() {
		ctx = context.Background()

		policy = `
package mytest

violation[{"msg":"v1"}]{
	input.foo = "bar"
}
violation[{"msg": "v2"}] {
	input.a = "z"
}
`
	})

	It("should not return violations when the occurrence is valid", func() {
		c, err := NewPolicy("mytest", policy, true)
		Expect(err).To(BeNil())


		validInput := map[string]string{
			"foo": "no",
			"a":   "no",
		}

		violations := c.Evaluate(ctx, validInput)
		Expect(violations).To(BeEmpty())
	})

	It("should return violations when the occurrence is invalid", func() {
		c, err := NewPolicy("mytest", policy, true)
		Expect(err).To(BeNil())

		invalidInput := map[string]string{
			"foo": "bar",
			"a":   "z",
		}

		violations := c.Evaluate(ctx, invalidInput)
		Expect(violations).ToNot(BeEmpty())
	})

	Context("grafeas", func() {

		It("should return violations when the ECR scan fails", func() {
			violations, err := evalAttestationRego(ctx, failDiscoveryOccurrences)

			Expect(err).To(BeNil())
			Expect(violations).ToNot(BeEmpty())
		})

		It("should not return violations when there are no occurrences ", func() {
			violations, err := evalAttestationRego(ctx, emptyOccurrences)

			Expect(err).To(BeNil())
			Expect(violations).ToNot(BeEmpty())
		})

		It("should not return violations when the ECR scan is successful", func() {
			violations, err := evalAttestationRego(ctx, successDiscoveryOccurrences)

			Expect(err).To(BeNil())
			Expect(violations).To(BeEmpty())
		})

		It("should return violations for high severity violations", func() {
			violations, err := evalAttestationRego(ctx, highVuln)

			Expect(err).To(BeNil())
			Expect(violations).ToNot(BeEmpty())
		})

		It("should not return violations for low severity vulnerabilities", func() {
			violations, err := evalAttestationRego(ctx, lowVuln)

			Expect(err).To(BeNil())
			Expect(violations).To(BeEmpty())
		})
	})
})

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

var attestationRego = `
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

func evalAttestationRego(ctx context.Context, occurrencesJSON string) ([]*Violation, error) {
	c, err := NewPolicy("default_attester", attestationRego, true)
	if err != nil {
		return nil, err
	}
	listOccurrences := make(map[string]interface{})
	err = json.Unmarshal([]byte(occurrencesJSON), &listOccurrences)
	if err != nil {
		return nil, err
	}
	return c.Evaluate(ctx, listOccurrences), nil
}
