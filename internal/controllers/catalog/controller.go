package catalog

import (
	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/http/response"
)

type Controller struct {
	catalogService interfaces.CatalogService
}

func NewController(catalogService interfaces.CatalogService) *Controller {
	return &Controller{
		catalogService: catalogService,
	}
}

func (ctrl *Controller) ListAmenities(c *gin.Context) {
	amenities, err := ctrl.catalogService.ListAmenities(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load amenities catalog")
		return
	}

	response.OK(c, gin.H{
		"amenities": amenities,
		"count":     len(amenities),
	})
}

func (ctrl *Controller) ListCancellationPolicies(c *gin.Context) {
	policies, err := ctrl.catalogService.ListCancellationPolicies(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load cancellation policies")
		return
	}

	response.OK(c, gin.H{
		"cancellation_policies": policies,
		"count":                 len(policies),
	})
}
