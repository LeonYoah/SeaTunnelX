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

package cluster

import "testing"

func TestParseCheckpointStorageFromYAMLLocalFile(t *testing.T) {
	content := `
seatunnel:
  engine:
    checkpoint:
      storage:
        plugin-config:
          namespace: /tmp/seatunnel/checkpoint_snapshot
          storage.type: hdfs
          fs.defaultFS: file:///tmp/
`

	spec := parseCheckpointStorageFromYAML(content)
	if spec == nil {
		t.Fatal("expected checkpoint storage spec")
	}
	if spec.StorageType != "LOCAL_FILE" {
		t.Fatalf("expected LOCAL_FILE storage type, got %s", spec.StorageType)
	}
	if spec.External {
		t.Fatalf("expected local checkpoint storage to be non-external")
	}
	if spec.Namespace != "/tmp/seatunnel/checkpoint_snapshot" {
		t.Fatalf("unexpected namespace: %s", spec.Namespace)
	}
}

func TestParseIMAPStorageFromYAMLDisabled(t *testing.T) {
	content := `
map:
  engine*:
    map-store:
      enabled: false
`

	spec := parseIMAPStorageFromYAML(content)
	if spec == nil {
		t.Fatal("expected imap storage spec")
	}
	if spec.Enabled {
		t.Fatalf("expected disabled IMAP")
	}
	if spec.StorageType != "DISABLED" {
		t.Fatalf("expected DISABLED storage type, got %s", spec.StorageType)
	}
}

func TestParseIMAPStorageFromYAMLS3(t *testing.T) {
	content := `
map:
  engine*:
    map-store:
      enabled: true
      properties:
        type: hdfs
        namespace: /seatunnel/engine
        storage.type: s3
        s3.bucket: s3a://seatunnel-bucket
        fs.s3a.endpoint: http://127.0.0.1:9000
`

	spec := parseIMAPStorageFromYAML(content)
	if spec == nil {
		t.Fatal("expected imap storage spec")
	}
	if !spec.Enabled {
		t.Fatalf("expected enabled IMAP")
	}
	if spec.StorageType != "S3" {
		t.Fatalf("expected S3 storage type, got %s", spec.StorageType)
	}
	if !spec.External {
		t.Fatalf("expected S3 IMAP to be external")
	}
	if spec.Bucket != "s3a://seatunnel-bucket" {
		t.Fatalf("unexpected bucket: %s", spec.Bucket)
	}
	if spec.Endpoint != "http://127.0.0.1:9000" {
		t.Fatalf("unexpected endpoint: %s", spec.Endpoint)
	}
}

func TestParseIMAPStorageFromYAMLHazelcastRoot(t *testing.T) {
	content := `
hazelcast:
  map:
    engine*:
      map-store:
        enabled: true
        properties:
          type: hdfs
          namespace: /seatunnel/imap/
          storage.type: s3
          s3.bucket: s3a://seatunnel-imap
          fs.defaultFS: s3a://seatunnel-imap
          fs.s3a.endpoint: http://127.0.0.1:19000
`

	spec := parseIMAPStorageFromYAML(content)
	if spec == nil {
		t.Fatal("expected imap storage spec")
	}
	if !spec.Enabled {
		t.Fatalf("expected enabled IMAP")
	}
	if spec.StorageType != "S3" {
		t.Fatalf("expected S3 storage type, got %s", spec.StorageType)
	}
	if spec.Namespace != "/seatunnel/imap/" {
		t.Fatalf("unexpected namespace: %s", spec.Namespace)
	}
	if spec.Endpoint != "http://127.0.0.1:19000" {
		t.Fatalf("unexpected endpoint: %s", spec.Endpoint)
	}
}

func TestRuntimeSpecFromClusterConfigDisabledIMAP(t *testing.T) {
	spec := runtimeSpecFromClusterConfig("imap", map[string]interface{}{
		"storage_type": "DISABLED",
		"namespace":    "/tmp/seatunnel/imap/",
	})
	if spec == nil {
		t.Fatal("expected runtime storage spec")
	}
	if spec.Enabled {
		t.Fatalf("expected disabled IMAP spec")
	}
	if spec.External {
		t.Fatalf("expected disabled IMAP to be non-external")
	}
}

func TestAsStringNilReturnsEmpty(t *testing.T) {
	if got := asString(nil); got != "" {
		t.Fatalf("expected empty string for nil, got %q", got)
	}
}
