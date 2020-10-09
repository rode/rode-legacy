package attester_test

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/liatrio/rode/pkg/attester"
	"github.com/liatrio/rode/pkg/occurrence"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/golang/mock/gomock"
	attestation "github.com/grafeas/grafeas/proto/v1beta1/attestation_go_proto"
	grafeasCommon "github.com/grafeas/grafeas/proto/v1beta1/common_go_proto"
	discovery "github.com/grafeas/grafeas/proto/v1beta1/discovery_go_proto"
	grafeas "github.com/grafeas/grafeas/proto/v1beta1/grafeas_go_proto"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/liatrio/rode/mocks/pkg/attester_mock"
	"github.com/liatrio/rode/mocks/pkg/eventmanager_mock"
	"github.com/liatrio/rode/mocks/pkg/occurrence_mock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("occurrence", func() {
	var (
		ctx      context.Context
		mockCtrl *gomock.Controller
		log      logr.Logger

		mockOccurrenceCreator *occurrence_mock.MockCreator
		mockOccurrenceLister  *occurrence_mock.MockLister
		mockAttesterLister    *attester_mock.MockLister
		eventManager          *eventmanager_mock.MockEventManager

		grafeasOccurrence *grafeas.Occurrence
		attesterName      string

		attestWrapper occurrence.Creator
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()
		mockOccurrenceCreator = occurrence_mock.NewMockCreator(mockCtrl)
		mockOccurrenceLister = occurrence_mock.NewMockLister(mockCtrl)
		mockAttesterLister = attester_mock.NewMockLister(mockCtrl)

		log = ctrl.Log.WithName("occurences").WithName("GrafeasClient")
		eventManager = eventmanager_mock.NewMockEventManager(mockCtrl)

		attesterName = fmt.Sprintf("attester%s", rand.String(10))
		grafeasOccurrence = createSuccessfulOccurrence(attesterName)
		attestWrapper = attester.NewAttestWrapper(
			log,
			mockOccurrenceCreator,
			mockOccurrenceLister,
			mockAttesterLister,
			eventManager,
		)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	When("CreateOccurrences", func() {
		var (
			attesterList     map[string]attester.Attester
			allOccurrences   []*grafeas.Occurrence
			att              *attester_mock.MockAttester
			attestOccurrence *grafeas.Occurrence
		)

		BeforeEach(func() {
			att = attester_mock.NewMockAttester(mockCtrl)
			att.EXPECT().Name().Return(attesterName).AnyTimes()

			attesterList = make(map[string]attester.Attester)
			attesterList[attesterName] = att

			attestOccurrence = createAttestOccurrence(attesterName)

			mockAttesterLister.
				EXPECT().
				ListAttesters().
				Return(attesterList).
				AnyTimes()

			allOccurrences = []*grafeas.Occurrence{
				grafeasOccurrence,
				createSuccessfulOccurrence("foo"),
			}
		})

		It("should return early if there are no occurrences", func() {
			err := attestWrapper.CreateOccurrences(ctx)

			Expect(err).To(BeNil())
		})

		It("should create the occurrences and publish a message", func() {
			mockOccurrenceCreator.
				EXPECT().
				CreateOccurrences(ctx, grafeasOccurrence)

			mockOccurrenceCreator.
				EXPECT().
				CreateOccurrences(ctx, attestOccurrence)

			mockOccurrenceLister.
				EXPECT().
				ListOccurrences(ctx, attesterName).
				Return(allOccurrences, nil)

			att.
				EXPECT().
				Attest(ctx, gomock.Any()).
				Return(&attester.AttestResponse{
					Attestation: attestOccurrence,
				}, nil)

			eventManager.
				EXPECT().
				Publish(attesterName, attestOccurrence).
				Return(nil)

			err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

			Expect(err).To(BeNil())
		})

		Context("error cases", func() {

			It("should return the error when create occurrence fails", func() {
				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("create occurrence failed"))
				eventManager.
					EXPECT().
					Publish(gomock.Any(), gomock.Any()).
					Times(0)

				err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

				Expect(err).NotTo(BeNil())
			})

			It("should return the error when listing occurrences fails", func() {
				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any())
				mockOccurrenceLister.
					EXPECT().
					ListOccurrences(ctx, attesterName).
					Return(nil, fmt.Errorf("failed to list occurrences"))

				err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

				Expect(err).NotTo(BeNil())
			})

			It("should not create an attestation when the attest returns a violation error", func() {
				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any()).
					Times(1)

				mockOccurrenceLister.
					EXPECT().
					ListOccurrences(ctx, attesterName).
					Return(allOccurrences, nil)

				violationErr := attester.ViolationError{
					Violations: []*attester.Violation{},
				}

				att.
					EXPECT().
					Attest(ctx, gomock.Any()).
					Return(nil, violationErr)

				err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

				Expect(err).To(BeNil())
			})

			It("should return any non-violation errors from the attester", func() {
				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any()).
					Times(1)

				mockOccurrenceLister.
					EXPECT().
					ListOccurrences(ctx, attesterName).
					Return(allOccurrences, nil)

				att.
					EXPECT().
					Attest(ctx, gomock.Any()).
					Return(nil, fmt.Errorf("non-violation error"))

				err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

				Expect(err).NotTo(BeNil())
			})

			It("should return an error if storing the attestation fails", func() {
				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any())

				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("error storing attestation"))

				mockOccurrenceLister.
					EXPECT().
					ListOccurrences(ctx, attesterName).
					Return(allOccurrences, nil)

				att.
					EXPECT().
					Attest(ctx, gomock.Any()).
					Return(&attester.AttestResponse{
						Attestation: attestOccurrence,
					}, nil)

				err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

				Expect(err).NotTo(BeNil())
			})

			It("should return an error if publishing the attestation fails", func() {
				mockOccurrenceCreator.
					EXPECT().
					CreateOccurrences(gomock.Any(), gomock.Any()).
					AnyTimes()

				mockOccurrenceLister.
					EXPECT().
					ListOccurrences(ctx, attesterName).
					Return(allOccurrences, nil)

				att.
					EXPECT().
					Attest(ctx, gomock.Any()).
					Return(&attester.AttestResponse{
						Attestation: attestOccurrence,
					}, nil)

				eventManager.
					EXPECT().
					Publish(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("error publishing to stream"))

				err := attestWrapper.CreateOccurrences(ctx, grafeasOccurrence)

				Expect(err).NotTo(BeNil())
			})
		})
	})
})

func createSuccessfulOccurrence(attesterName string) *grafeas.Occurrence {
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

func createAttestOccurrence(attesterName string) *grafeas.Occurrence {
	return &grafeas.Occurrence{
		Resource: &grafeas.Resource{
			Uri: attesterName,
		},
		NoteName: fmt.Sprintf("projects/rode/notes/%s", attesterName),
		Kind:     grafeasCommon.NoteKind_ATTESTATION,
		Details: &grafeas.Occurrence_Attestation{
			Attestation: &attestation.Details{
				Attestation: &attestation.Attestation{
					Signature: &attestation.Attestation_PgpSignedAttestation{
						PgpSignedAttestation: &attestation.PgpSignedAttestation{
							ContentType: attestation.PgpSignedAttestation_CONTENT_TYPE_UNSPECIFIED,
							Signature:   rand.String(10),
							KeyId: &attestation.PgpSignedAttestation_PgpKeyId{
								PgpKeyId: rand.String(10),
							},
						},
					},
				},
			},
		},
	}
}
