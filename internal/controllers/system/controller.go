package system

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stayhub/backend/internal/config"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/db"
	"github.com/stayhub/backend/internal/platform/redis"
)

type Controller struct {
	cfg   *config.Config
	db    *db.DB
	redis *redis.Client
	clock clock.Clock
}

func NewController(cfg *config.Config, database *db.DB, rdb *redis.Client, clk clock.Clock) *Controller {
	return &Controller{
		cfg:   cfg,
		db:    database,
		redis: rdb,
		clock: clk,
	}
}

func (ctrl *Controller) Healthz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	pgStatus := "ok"
	if err := ctrl.db.Ping(ctx); err != nil {
		pgStatus = "unhealthy: " + err.Error()
	}

	redisStatus := "ok"
	if ctrl.redis != nil {
		if err := ctrl.redis.Ping(ctx); err != nil {
			redisStatus = "unhealthy: " + err.Error()
		}
	}

	healthy := pgStatus == "ok" && redisStatus == "ok"
	code := http.StatusOK
	if !healthy {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":       map[bool]string{true: "ok", false: "degraded"}[healthy],
		"version_mode": ctrl.cfg.VersionMode,
		"timestamp":    ctrl.clock.Now().Format(time.RFC3339),
		"services": gin.H{
			"postgres": pgStatus,
			"redis":    redisStatus,
		},
	})
}

func (ctrl *Controller) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	pgStatus := "ok"
	if err := ctrl.db.Ping(ctx); err != nil {
		pgStatus = "unhealthy: " + err.Error()
	}

	redisStatus := "ok"
	if ctrl.redis != nil {
		if err := ctrl.redis.Ping(ctx); err != nil {
			redisStatus = "unhealthy: " + err.Error()
		}
	}

	ready := pgStatus == "ok" && redisStatus == "ok"
	code := http.StatusOK
	if !ready {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status":       map[bool]string{true: "ready", false: "not_ready"}[ready],
		"version_mode": ctrl.cfg.VersionMode,
		"timestamp":    ctrl.clock.Now().Format(time.RFC3339),
		"checks": gin.H{
			"database": pgStatus,
			"redis":    redisStatus,
		},
	})
}
