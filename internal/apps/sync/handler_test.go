package sync

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetJobLogsRejectsLegacyLinesQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newTestSyncService(t)
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/api/v1/sync/jobs/:id/logs", handler.GetJobLogs)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/jobs/33/logs?lines=400", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestGetJobLogsRejectsLegacyAllQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newTestSyncService(t)
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/api/v1/sync/jobs/:id/logs", handler.GetJobLogs)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/jobs/33/logs?all=true", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}
