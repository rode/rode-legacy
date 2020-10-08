package attester_test

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/attester"
	"github.com/liatrio/rode/pkg/occurrence"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/golang/mock/gomock"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/liatrio/rode/mock/pkg/mock_attester"
	"github.com/liatrio/rode/mock/pkg/mock_occurrence"
	"github.com/liatrio/rode/pkg/eventmanager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("occurrence", func() {
	var (
		ctx      context.Context
		mockCtrl *gomock.Controller
		log      logr.Logger

		mockOccurrenceCreator *mock_occurrence.MockCreator
		mockOccurrenceLister  *mock_occurrence.MockLister
		mockAttesterLister    *mock_attester.MockLister
		eventManager          eventmanager.EventManager

		grafeasOccurrence   *grafeas.Occurrence
		attesterName string

		attestWrapper occurrence.Creator
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()
		mockOccurrenceCreator = mock_occurrence.NewMockCreator(mockCtrl)
		mockOccurrenceLister = mock_occurrence.NewMockLister(mockCtrl)
		mockAttesterLister = mock_attester.NewMockLister(mockCtrl)

		log = ctrl.Log.WithName("occurences").WithName("GrafeasClient")
		eventManager = eventmanager.NewEventManagerNone(log)


		attesterName = fmt.Sprintf("attester%s", rand.String(10))
		grafeasOccurrence = createTestOccurrence(attesterName)
		attestWrapper = attester.NewAttestWrapper(
			log,
			mockOccurrenceCreator,
			mockOccurrenceLister,
			mockAttesterLister,
			eventManager,
		)
	})

	FWhen("CreateOccurrences", func() {
		It("should return early if there are no occurrences", func() {
			err := attestWrapper.CreateOccurrences(ctx)

			Expect(err).To(BeNil())
		})

		It("create the occurrences using the delegate", func() {
			mockOccurrenceCreator.
				EXPECT().
				CreateOccurrences(ctx, grafeasOccurrence)
			mockOccurrenceLister.
				EXPECT().
				ListOccurrences(ctx, attesterName)
			mockAttesterLister.EXPECT().ListAttesters()

			err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

			Expect(err).To(BeNil())
		})

		It("should publish verified attestations", func() {
			allOccurrences := []*grafeas.Occurrence{
				grafeasOccurrence,
				createTestOccurrence("foo"),
			}
			//att, err := createAttester(attesterName, "", false)
			//Expect(err).To(BeNil())
			att := mock_attester.NewMockAttester(mockCtrl)

			attesterList := make(map[string]attester.Attester)
			attesterList[attesterName] = att

			mockOccurrenceCreator.EXPECT().CreateOccurrences(gomock.Any(), gomock.Any())
			mockOccurrenceLister.EXPECT().
				ListOccurrences(gomock.Any(), gomock.Any()).
				Return(allOccurrences, nil)
			mockAttesterLister.EXPECT().ListAttesters().Return(attesterList)
			att.EXPECT().Attest(ctx, gomock.Any())

			err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)
			Expect(err).To(BeNil())

		})
	})
})

func createTestOccurrence(attesterName string) *grafeas.Occurrence {
	return &grafeas.Occurrence{
		Resource: &grafeas.Resource{
			Uri: attesterName,
		},
		NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
		Details: &grafeas.Occurrence_Discovered{
			Discovered: &discovery.Details{
				Discovered: &discovery.Discovered{
					AnalysisStatus: discovery.Discovered_FINISHED_SUCCESS,
				},
			},
		},
	}
}