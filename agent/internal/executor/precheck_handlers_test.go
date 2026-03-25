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

package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seatunnel/seatunnelX/agent/internal/installer"
)

func TestHandleSeatunnelXJavaProxyInspectCheckpointAcceptsConfigBackedRequest(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			return
		case "/api/v1/storage/checkpoint/inspect":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":          true,
				"message":     "inspect ok",
				"storageType": "hdfs",
				"path":        "/tmp/checkpoint",
				"fileName":    "checkpoint",
			})
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("SEATUNNELX_JAVA_PROXY_ENDPOINT", server.URL)

	result, err := handleSeatunnelXJavaProxyInspectCheckpoint(context.Background(), map[string]string{
		"install_dir":  t.TempDir(),
		"version":      "2.3.13",
		"path":         "/tmp/checkpoint",
		"storage_type": string(installer.CheckpointStorageLocalFile),
		"namespace":    "/tmp",
	})
	if err != nil {
		t.Fatalf("handle inspect checkpoint returned error: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful result, got %#v", result)
	}
	if received["contentBase64"] != nil {
		t.Fatalf("expected config-backed request without inline content, got %#v", received)
	}
	configMap, ok := received["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected config payload, got %#v", received["config"])
	}
	if configMap["namespace"] != "/tmp/" {
		t.Fatalf("expected normalized namespace /tmp/, got %#v", configMap["namespace"])
	}
	if received["path"] != "/tmp/checkpoint" {
		t.Fatalf("expected path /tmp/checkpoint, got %#v", received["path"])
	}
}

func TestHandleSeatunnelXJavaProxyInspectCheckpointAcceptsBase64Request(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			return
		case "/api/v1/storage/checkpoint/inspect":
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":      true,
				"message": "inspect ok",
				"path":    "/tmp/checkpoint",
			})
			return
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	t.Setenv("SEATUNNELX_JAVA_PROXY_ENDPOINT", server.URL)

	result, err := handleSeatunnelXJavaProxyInspectCheckpoint(context.Background(), map[string]string{
		"install_dir":    t.TempDir(),
		"path":           "/tmp/checkpoint",
		"content_base64": "Y2hlY2twb2ludA==",
		"storage_type":   string(installer.CheckpointStorageLocalFile),
		"namespace":      "/tmp",
	})
	if err != nil {
		t.Fatalf("handle inspect checkpoint returned error: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected successful result, got %#v", result)
	}
	if received["contentBase64"] != "Y2hlY2twb2ludA==" {
		t.Fatalf("expected inline content, got %#v", received["contentBase64"])
	}
}
