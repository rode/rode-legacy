package eventmanager_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/liatrio/rode/mocks/pkg/nats_mock"
	"github.com/liatrio/rode/pkg/attester"
	"github.com/liatrio/rode/pkg/eventmanager"
	"github.com/nats-io/jsm.go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

var _ = Describe("jetstream eventmanager", func() {
	var (
		mockCtrl       *gomock.Controller
		log            logr.Logger
		eventManager   eventmanager.EventManager
		mockConnection *nats_mock.MockConnection
		expectedError  error

		mockConnectionFactory *nats_mock.MockConnectionFactory
		mockStreamManager     *nats_mock.MockStreamManager
		natsServerUrl         string
		attesterName          string
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())

		mockConnection = nats_mock.NewMockConnection(mockCtrl)
		expectedError = fmt.Errorf(rand.String(10))

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
		BeforeEach(func() {
			mockStreamManager.EXPECT().WithConnection(mockConnection).AnyTimes()
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
				Return(mockConnection, nil).
				Times(1)

			err := eventManager.Initialize(attesterName)
			Expect(err).To(BeNil())
		})

		It("should return any error from attempting to create a connection", func() {
			mockConnectionFactory.
				EXPECT().
				Connect(gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			err := eventManager.Initialize(attesterName)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(expectedError.Error()))
		})

		It("should create a stream for attesters and attester public keys with file storage and a max age of 1 year", func() {
			var actualStreams []string
			var actualSubjects []string
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(mockConnection, nil)
			mockStreamManager.
				EXPECT().
				NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Do(func(streamName string, _ ...jsm.StreamOption) {
					actualStreams = append(actualStreams, streamName)
				}).AnyTimes()

			mockStreamManager.
				EXPECT().
				Subjects(gomock.Any()).
				Do(func(subjects ...string) {
					actualSubjects = append(actualSubjects, subjects...)
				}).AnyTimes()

			err := eventManager.Initialize(attesterName)
			Expect(err).To(BeNil())
			Expect(len(actualStreams)).To(Equal(2))
			Expect(actualStreams).To(ContainElement("ATTESTATION"))
			Expect(actualStreams).To(ContainElement("ATTESTATION_KEY"))

			Expect(len(actualSubjects)).To(Equal(2))
			Expect(actualSubjects).To(ContainElement("ATTESTATION.*"))
			Expect(actualSubjects).To(ContainElement("ATTESTATION_KEY.*"))
		})

		It("should not try to re-create streams when called multiple times", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(mockConnection, nil)
			mockStreamManager.EXPECT().Subjects(gomock.Any()).AnyTimes()
			mockStreamManager.
				EXPECT().
				NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Times(2)

			Expect(eventManager.Initialize(attesterName)).To(BeNil())
			Expect(eventManager.Initialize(attesterName)).To(BeNil())
			Expect(eventManager.Initialize(attesterName)).To(BeNil())
		})

		It("should return any errors from creating a stream", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(mockConnection, nil)
			mockStreamManager.EXPECT().Subjects(gomock.Any()).AnyTimes()
			mockStreamManager.
				EXPECT().
				NewStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Times(1).
				Return(nil, expectedError)

			err := eventManager.Initialize(attesterName)

			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(expectedError.Error()))
		})
	})

	When("PublishPublicKey", func() {
		var (
			publicKey []byte
		)

		BeforeEach(func() {
			signer, err := attester.NewSigner(attesterName)
			Expect(err).To(BeNil())

			publicKey, err = signer.SerializePublicKey()
			Expect(err).To(BeNil())

			mockStreamManager.EXPECT().WithConnection(mockConnection).AnyTimes()
		})

		It("should create a connection to the NATS server", func() {
			mockStreamManager.EXPECT().LoadStream(gomock.Any(), gomock.Any())
			mockConnection.EXPECT().Publish(gomock.Any(), gomock.Any())
			mockConnectionFactory.
				EXPECT().
				Connect(natsServerUrl).
				Return(mockConnection, nil).
				Times(1)

			err := eventManager.PublishPublicKey(attesterName, publicKey)
			Expect(err).To(BeNil())
		})

		It("should load the ATTESTATION_KEY stream", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(mockConnection, nil)
			mockConnection.EXPECT().Publish(gomock.Any(), gomock.Any())
			mockStreamManager.
				EXPECT().
				LoadStream("ATTESTATION_KEY", gomock.Any()).
				Times(1)

			err := eventManager.PublishPublicKey(attesterName, publicKey)
			Expect(err).To(BeNil())
		})

		It("should return any error from attempting to create a connection", func() {
			mockConnectionFactory.
				EXPECT().
				Connect(gomock.Any()).
				Return(nil, expectedError).
				Times(1)

			err := eventManager.PublishPublicKey(attesterName, publicKey)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal(expectedError.Error()))
		})

		It("should base64 encode the key and publish it", func() {
			mockConnectionFactory.EXPECT().Connect(gomock.Any()).Return(mockConnection, nil)
			mockStreamManager.EXPECT().LoadStream(gomock.Any(), gomock.Any())
			var actualRawMessage []byte
			mockConnection.EXPECT().
				Publish("ATTESTATION_KEY." + attesterName, gomock.Any()).
				Do(func(_ string, data []byte) {
					actualRawMessage = data
			}).
				Times(1)

			err := eventManager.PublishPublicKey(attesterName, publicKey)
			Expect(err).To(BeNil())

			actualMessage := struct {
				Base64PublicKey string
			}{}
			Expect(json.Unmarshal(actualRawMessage, &actualMessage)).To(BeNil())
			Expect(actualMessage.Base64PublicKey).To(Equal(base64.StdEncoding.EncodeToString(publicKey)))
		})
	})
})
