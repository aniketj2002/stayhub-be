package http

import (
	"github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine, deps ServerDeps) {
	v1 := r.Group("/api/v1")

	registerPublicRoutes(r, v1, deps)
	registerGuestRoutes(v1, deps)
	registerHostRoutes(v1, deps)
	registerAdminRoutes(v1, deps)
}
