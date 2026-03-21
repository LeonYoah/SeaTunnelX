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

package config

import (
	"strings"
	"testing"
)

func TestValidateConfigContentRejectsInvalidHazelcastYAML(t *testing.T) {
	content := `hazelcast:
  cluster-name: seatunnel
  network:
    port:
      port: 5801
 map:
  engine*:
    map-store:
      enabled: true
`

	err := validateConfigContent(ConfigTypeHazelcast, content)
	if err == nil {
		t.Fatal("expected invalid hazelcast yaml to fail validation")
	}
}

func TestValidateConfigContentRejectsHazelcastWithoutRoot(t *testing.T) {
	content := `map:
  engine*:
    map-store:
      enabled: true
`

	err := validateConfigContent(ConfigTypeHazelcast, content)
	if err == nil {
		t.Fatal("expected hazelcast yaml without top-level hazelcast key to fail validation")
	}
}

func TestValidateConfigContentAcceptsValidHazelcastYAML(t *testing.T) {
	content := `hazelcast:
  cluster-name: seatunnel
  map:
    engine*:
      map-store:
        enabled: true
        initial-mode: EAGER
`

	if err := validateConfigContent(ConfigTypeHazelcast, content); err != nil {
		t.Fatalf("expected valid hazelcast yaml, got error: %v", err)
	}
}

func TestValidateConfigContentAcceptsValidHazelcastClientYAML(t *testing.T) {
	content := `hazelcast-client:
  cluster-name: seatunnel
  network:
    cluster-members:
      - 127.0.0.1:5801
`

	if err := validateConfigContent(ConfigTypeHazelcastClient, content); err != nil {
		t.Fatalf("expected valid hazelcast-client yaml, got error: %v", err)
	}
}

func TestValidateConfigContentRejectsHazelcastClientWithWrongRoot(t *testing.T) {
	content := `hazelcast:
  cluster-name: seatunnel
`

	err := validateConfigContent(ConfigTypeHazelcastClient, content)
	if err == nil {
		t.Fatal("expected hazelcast-client yaml with wrong root to fail validation")
	}
	if !strings.Contains(err.Error(), "hazelcast-client") {
		t.Fatalf("expected error to mention hazelcast-client root, got: %v", err)
	}
}

func TestValidateConfigContentAcceptsValidSeatunnelYAML(t *testing.T) {
	content := `seatunnel:
  engine:
    checkpoint:
      interval: 10000
`

	if err := validateConfigContent(ConfigTypeSeatunnel, content); err != nil {
		t.Fatalf("expected valid seatunnel yaml, got error: %v", err)
	}
}

func TestNormalizeConfigContentFormatsParseableYAML(t *testing.T) {
	content := "hazelcast:\n  cluster-name: seatunnel\n  map: {engine*: {map-store: {enabled: true}}}\n"

	normalized, err := normalizeConfigContent(ConfigTypeHazelcast, content)
	if err != nil {
		t.Fatalf("expected normalize to succeed, got error: %v", err)
	}
	if !strings.Contains(normalized, "hazelcast:") || !strings.Contains(normalized, "map-store:") {
		t.Fatalf("unexpected normalized yaml: %s", normalized)
	}
	if strings.Contains(normalized, "{engine*:") {
		t.Fatalf("expected YAML to be expanded into normalized form, got: %s", normalized)
	}
}

func TestNormalizeConfigContentRejectsInvalidYAML(t *testing.T) {
	content := "hazelcast:\n  cluster-name: seatunnel\n  map\n    broken: true\n"

	if _, err := normalizeConfigContent(ConfigTypeHazelcast, content); err == nil {
		t.Fatal("expected invalid YAML to fail normalization")
	}
}

func TestNormalizeConfigContentRepairsHazelcastMapIndentation(t *testing.T) {
	content := `hazelcast:
    cluster-name: seatunnel
    network:
        port:
            port: 5801
    properties:
        hazelcast.logging.type: log4j2
  map:
    engine*:
      map-store:
        enabled: true
        initial-mode: EAGER
`

	normalized, err := normalizeConfigContent(ConfigTypeHazelcast, content)
	if err != nil {
		t.Fatalf("expected hazelcast smart repair to succeed, got error: %v", err)
	}
	if !strings.Contains(normalized, "  map:\n") {
		t.Fatalf("expected normalized hazelcast map block under hazelcast root, got: %s", normalized)
	}
	if strings.Contains(normalized, "\n map:\n") {
		t.Fatalf("expected malformed indentation to be repaired, got: %s", normalized)
	}
}

func TestNormalizeConfigContentRepairsSeatunnelCheckpointIndentation(t *testing.T) {
	content := `seatunnel:
    engine:
        checkpoint:
            interval: 10000
      storage:
        type: hdfs
        plugin-config:
          namespace: /tmp/seatunnel/checkpoint_snapshot
          storage.type: hdfs
          fs.defaultFS: file:///
`

	normalized, err := normalizeConfigContent(ConfigTypeSeatunnel, content)
	if err != nil {
		t.Fatalf("expected seatunnel smart repair to succeed, got error: %v", err)
	}
	if !strings.Contains(normalized, "  engine:\n") {
		t.Fatalf("expected engine block under seatunnel root, got: %s", normalized)
	}
	if !strings.Contains(normalized, "    checkpoint:\n") {
		t.Fatalf("expected checkpoint block under engine, got: %s", normalized)
	}
	if !strings.Contains(normalized, "      storage:\n") {
		t.Fatalf("expected storage block under checkpoint, got: %s", normalized)
	}
	if !strings.Contains(normalized, "        plugin-config:\n") {
		t.Fatalf("expected plugin-config block under storage, got: %s", normalized)
	}
}
