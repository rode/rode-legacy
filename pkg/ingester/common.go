package ingester

import (
	"github.com/gin-gonic/gin"
)

// Ingester is the interface that all custom ingesters must implement
type Ingester interface {
	Reconcile() error
}

// Router is the alias for Gin engine
type Router = *gin.Engine
