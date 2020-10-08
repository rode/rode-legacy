package attester

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/golang/mock/gomock"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/liatrio/rode/mock/pkg/mock_attester"
	"github.com/liatrio/rode/mock/pkg/mock_occurrence"
	"github.com/liatrio/rode/pkg/eventmanager"

	// "github.com/liatrio/rode/mock/pkg/occurrence"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("occurrence", func() {
	var (
		mockCtrl              *gomock.Controller
		mockOccurrenceCreator *mock_occurrence.Creator
		mockOccurrenceLister  *mock_occurrence.MockLister
		mockAttesterLister    *mock_attester.MockLister
		// mockThing *mockthing.MockThing
		// consumer  *Consumer
	)

	// BeforeEach(func() {
	// mockThing = mockthing.NewMockThing(mockCtrl)
	// consumer = NewConsumer(mockThing)
	// })

	When("something happens", func() {
		It("should do something", func() {
			mockCtrl = gomock.NewController(GinkgoT())
			// attesterWrapper := NewAttestWrapper(log, )

			attesterName := fmt.Sprintf("attester%s", rand.String(10))
			noteName := fmt.Sprintf("projects/rode/notes/%s", attesterName)

			occurrences := &grafeas.Occurrence{
				Resource: &grafeas.Resource{
					Uri: attesterName,
				},
				NoteName: noteName,
				Details: &grafeas.Occurrence_Discovered{
					Discovered: &discovery.Details{
						Discovered: &discovery.Discovered{
							AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
						},
					},
				},
			}

			occurrenceCreator := mockOccurrenceCreator.NewMockCreator(mockCtrl)
			occurrenceLister := mockOccurrenceLister.NewMockLister(mockCtrl)
			attesterLister := mockAttesterLister.NewMockLister(mockCtrl)

			// ctrl.SetLogger(zap.New(func(o *zap.Options) {
			// 	o.Development = true
			// }))

			log := ctrl.Log.WithName("occurences").WithName("GrafeasClient")

			eventManager := eventmanager.NewEventManagerNone(log)

			attestWrapper := NewAttestWrapper(
				log,
				occurrenceCreator,
				occurrenceLister,
				attesterLister,
				eventManager,
			)

			err := attestWrapper.CreateOccurrences(ctx, occurrences)
			Expect(err).To(BeNil())

		})
	})
})

// func (a *FakeOccurrenceCreator) CreateOccurrences(ctx context.Context, occurrences ...*grafeas.Occurrence) error {
// 	return nil
// }

// type FakeOccurrenceCreator struct {
// }

// type FakeOccurrenceLister struct {
// }

// func (o *FakeOccurrenceLister) ListOccurrences(ctx context.Context, resourceURI string) ([]*grafeas.Occurrence, error) {
// 	ocName1 := fmt.Sprintf("attester%s", rand.String(10))
// 	noteName1 := fmt.Sprintf("projects/rode/notes/%s", ocName1)
// 	occurrence1 := &grafeas.Occurrence{
// 		Resource: &grafeas.Resource{
// 			Uri: ocName1,
// 		},
// 		NoteName: noteName1,
// 		Details: &grafeas.Occurrence_Discovered{
// 			Discovered: &discovery.Details{
// 				Discovered: &discovery.Discovered{
// 					AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
// 				},
// 			},
// 		},
// 	}
// 	ocName2 := fmt.Sprintf("attester%s", rand.String(10))
// 	noteName2 := fmt.Sprintf("projects/rode/notes/%s", ocName1)
// 	occurrence2 := &grafeas.Occurrence{
// 		Resource: &grafeas.Resource{
// 			Uri: ocName2,
// 		},
// 		NoteName: noteName2,
// 		Details: &grafeas.Occurrence_Discovered{
// 			Discovered: &discovery.Details{
// 				Discovered: &discovery.Discovered{
// 					AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
// 				},
// 			},
// 		},
// 	}

// 	occurrencesList := []*grafeas.Occurrence{occurrence1, occurrence2}
// 	return occurrencesList, nil
// }

// func (o *FakeOccurrenceLister) ListAttestations(ctx context.Context, resourceURI string) ([]*grafeas.Occurrence, error) {
// 	return nil, nil
// }

// type FakeLister struct {
// }

// func (l *FakeLister) ListAttesters() map[string]Attester {
// 	return nil
// }
