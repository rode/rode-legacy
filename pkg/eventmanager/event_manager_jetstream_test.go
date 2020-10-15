package eventmanager_test

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/liatrio/rode/mocks/pkg/nats_mock"
	"github.com/liatrio/rode/pkg/eventmanager"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

var _ = Describe("jetstream eventmanager", func() {
	var (
		mockCtrl     *gomock.Controller
		log          logr.Logger
		eventManager eventmanager.EventManager

		mockConnectionFactory *nats_mock.MockConnectionFactory
		mockStreamManager     *nats_mock.MockStreamManager
		natsServerUrl         string
		attesterName          string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		natsServerUrl = rand.String(10)
		attesterName = rand.String(10)
		log = ctrl.Log.WithName("JetstreamClient")
		mockConnectionFactory = nats_mock.NewMockConnectionFactory(mockCtrl)
		mockStreamManager = nats_mock.NewMockStreamManager(mockCtrl)

		eventManager = eventmanager.NewJetstreamClient(
			log,
			natsServerUrl,
			nil,
			mockConnectionFactory,
			mockStreamManager,
		)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	When("Initialize()", func() {
		var (
			natsConnection *nats.Conn
			randomError error
		)

		BeforeEach(func() {
			natsConnection = &nats.Conn{}
			randomError = fmt.Errorf(rand.String(10))

			mockStreamManager.EXPECT().WithConnection(natsConnection).AnyTimes()
			mockStreamManager.EXPECT().StreamConnection(gomock.Any()).AnyTimes()
			mockStreamManager.EXPECT().MaxAge(365 * 24 * time.Hour).AnyTimes()
			mockStreamManager.EXPECT().FileStorage().AnyTimes()
		})

		It("should create a connection using the url", func() {
			mockStreamManager.EXPECT().Subjects(gomock.Any()).AnyTimes()
			mockStreamManager.EXPECT().NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockConnectionFactory.
				EXPECT().
				Connect(natsServerUrl).
				Return(natsConnection, nil).
				Times(1)

			err := eventManager.Initialize(attesterName)
			Expect(err).To(BeNil())
		})

		It("should return any error from attempting to create a connection", func() {
			mockConnectionFactory.
				EXPECT().
				Connect(gomock.Any()).
				Return(nil, randomError).
				Times(1)

			err := eventManager.Initialize(attesterName)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(randomError.Error()))
		})

		It("should create a stream for attestations with file storage and a max age of 1 year", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(natsConnection, nil)
			mockStreamManager.
				EXPECT().
				NewStream("ATTESTATION", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Times(1)
			mockStreamManager.EXPECT().NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

			mockStreamManager.
				EXPECT().
				Subjects("ATTESTATION.*").
				Times(1)
			mockStreamManager.EXPECT().Subjects(gomock.Any())

			err := eventManager.Initialize(attesterName)
			Expect(err).To(BeNil())
		})

		It("should create a stream for attester public keys with file storage and a max age of 1 year", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(natsConnection, nil)
			mockStreamManager.EXPECT().NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			mockStreamManager.
				EXPECT().
				NewStream("ATTESTATION_KEY", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Times(1)

			mockStreamManager.EXPECT().Subjects(gomock.Any())
			mockStreamManager.
				EXPECT().
				Subjects("ATTESTATION_KEY.*").
				Times(1)

			err := eventManager.Initialize(attesterName)
			Expect(err).To(BeNil())
		})

		It("should not try to re-create streams when called multiple times", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(natsConnection, nil)
			mockStreamManager.EXPECT().Subjects(gomock.Any()).AnyTimes()
			mockStreamManager.
				EXPECT().
				NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Times(2)

			Expect(eventManager.Initialize(attesterName)).To(BeNil())
			Expect(eventManager.Initialize(attesterName)).To(BeNil())
			Expect(eventManager.Initialize(attesterName)).To(BeNil())
		})
	})
})
