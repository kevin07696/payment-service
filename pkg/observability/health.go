package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status   string            `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Checks   map[string]string `json:"checks"`
}

// HealthChecker manages health checks for the service
type HealthChecker struct {
	dbPool *pgxpool.Pool
}

// NewHealthChecker creates a new HealthChecker
func NewHealthChecker(dbPool *pgxpool.Pool) *HealthChecker {
	return &HealthChecker{
		dbPool: dbPool,
	}
}

// Check performs health checks and returns the status
func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
	checks := make(map[string]string)
	overallStatus := "healthy"

	// Database health check
	if h.dbPool != nil {
		dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		if err := h.dbPool.Ping(dbCtx); err != nil {
			checks["database"] = "unhealthy: " + err.Error()
			overallStatus = "unhealthy"
		} else {
			checks["database"] = "healthy"
		}
	} else {
		checks["database"] = "not configured"
	}

	return HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    checks,
	}
}

// HealthHandler returns an HTTP handler for health checks
func (h *HealthChecker) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := h.Check(r.Context())

		w.Header().Set("Content-Type", "application/json")
		if status.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(status)
	}
}
