package controller

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"net/http"

	ginzap "github.com/gin-contrib/zap"
	"github.com/golang/protobuf/jsonpb"
	"github.com/liatrio/rode/pkg/enforcer"
	"github.com/liatrio/rode/pkg/occurrence"

	"github.com/gin-gonic/gin"

	admissionv1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StartAPI creates a new API engine
func StartAPI(ctx context.Context, logger *zap.SugaredLogger, occurrenceLister occurrence.Lister, enf enforcer.Enforcer) error {
	router := gin.New()

	router.Use(ginzap.Ginzap(logger.Desugar(), time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(logger.Desugar(), true))

	logger.Debug("Registering health API")
	routeHealth(router)
	logger.Debug("Registering occurrence API")
	routeOccurrences(router, occurrenceLister)
	logger.Debug("Registering validate API")
	routeValidate(router, enf)

	srv := &http.Server{
		Addr:    ":4000",
		Handler: router,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServeTLS(os.Getenv("TLS_CLIENT_CERT"), os.Getenv("TLS_CLIENT_KEY")); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("listen: %s\n", err)
		}
	}()
	return nil
}

func routeHealth(router gin.IRouter) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "healthy",
		})
	})
}

func routeOccurrences(router gin.IRouter, occurrenceLister occurrence.Lister) {
	marshaler := &jsonpb.Marshaler{}
	router.GET("/occurrences/*resource", func(c *gin.Context) {
		resourceURI := strings.TrimPrefix(c.Param("resource"), "/")
		o, err := occurrenceLister.ListOccurrences(c, resourceURI)
		if err != nil {
			c.AbortWithError(400, err)
		} else {
			c.Stream(func(w io.Writer) bool {
				err := marshaler.Marshal(w, o)
				if err != nil {
					c.Error(err)
				}
				return false
			})
		}
	})
}

func routeValidate(router gin.IRouter, enf enforcer.Enforcer) {
	router.POST("/validate", func(c *gin.Context) {
		var arRequest admissionv1.AdmissionReview
		err := c.BindJSON(&arRequest)

		if arRequest.Request.Kind.Group == "" && arRequest.Request.Kind.Kind == "Pod" {
			namespace := arRequest.Request.Namespace
			pod := corev1.Pod{}
			if err = json.Unmarshal(arRequest.Request.Object.Raw, &pod); err == nil {
				for _, container := range pod.Spec.Containers {
					err = enf.Enforce(c, namespace, container.Image)
					if err != nil {
						break
					}
				}
			}
		}

		var arResponse admissionv1.AdmissionReview
		if err != nil {
			arResponse = admissionv1.AdmissionReview{
				Response: &admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: err.Error(),
					},
				},
			}
		} else {
			arResponse = admissionv1.AdmissionReview{
				Response: &admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Message: "Approved by rode",
					},
				},
			}
		}

		c.JSON(200, &arResponse)
	})
}
