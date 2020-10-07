package collector

import (
	"context"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	"github.com/liatrio/rode/pkg/occurrence"
)

type testCollector struct {
	logger      logr.Logger
	testMessage string
}

func NewTestCollector(logger logr.Logger, testMessage string) Collector {
	return &testCollector{
		logger:      logger.WithName("testCollector"),
		testMessage: testMessage,
	}
}

func (t *testCollector) Type() string {
	return "test"
}

func (t *testCollector) Reconcile(ctx context.Context, name types.NamespacedName) error {
	t.logger.Info("reconciling test collector")

	return nil
}

func (t *testCollector) Start(ctx context.Context, stopChan chan interface{}, occurrenceCreator occurrence.Creator) error {
	go func() {
		for range time.NewTicker(60 * time.Second).C {
			select {
			case <-ctx.Done():
				stopChan <- true
				return
			default:
				t.logger.Info("Creating test collector occurrence", "message", t.testMessage)
				occurrences := make([]*grafeas.Occurrence, 0)
				o := &grafeas.Occurrence{
					Name: "test_occurrence",
					Resource: &grafeas.Resource{
						Uri: "nginx:latest",
					},
					NoteName: "projects/rode/notes/testResource",
				}

				dummyDiscoveryDetails := &grafeas.Occurrence_Discovered{
					Discovered: &discovery.Details{
						Discovered: &discovery.Discovered{
							AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
						},
					},
				}

				o.Details = dummyDiscoveryDetails
				occurrences = append(occurrences, o)
				err := occurrenceCreator.CreateOccurrences(ctx, occurrences...)
				if err != nil {
					t.logger.Error(err, "error creating dummy occurrence")
				}
			}
		}

		t.logger.Info("test collector goroutine finished")
	}()

	return nil
}

func (t *testCollector) HandleWebhook(writer http.ResponseWriter, request *http.Request, occurrenceCreator occurrence.Creator) {
	t.logger.Info("got request for test collector")

	writer.WriteHeader(http.StatusOK)
}

func (t *testCollector) Destroy(ctx context.Context) error {
	t.logger.Info("destroying test collector")

	return nil
}
