/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
