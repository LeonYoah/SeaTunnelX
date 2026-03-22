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

package seatunnel

import "testing"

func TestParseHDFSHANamenodes(t *testing.T) {
	t.Run("accepts two trimmed names", func(t *testing.T) {
		namenodes, err := ParseHDFSHANamenodes(" nn1 , nn2 ")
		if err != nil {
			t.Fatalf("expected valid namenodes, got error: %v", err)
		}
		if len(namenodes) != 2 || namenodes[0] != "nn1" || namenodes[1] != "nn2" {
			t.Fatalf("unexpected namenodes: %#v", namenodes)
		}
	})

	t.Run("rejects fewer than two names", func(t *testing.T) {
		if _, err := ParseHDFSHANamenodes("nn1"); err == nil {
			t.Fatal("expected error for single namenode")
		}
	})

	t.Run("rejects more than two names", func(t *testing.T) {
		if _, err := ParseHDFSHANamenodes("nn1,nn2,nn3"); err == nil {
			t.Fatal("expected error for more than two namenodes")
		}
	})
}

func TestResolveHDFSHARPCAddresses(t *testing.T) {
	endpoints, err := ResolveHDFSHARPCAddresses("nn1,nn2", "host-1:8020", "host-2:8020")
	if err != nil {
		t.Fatalf("expected valid HA RPC addresses, got error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
	if endpoints[0].Name != "nn1" || endpoints[0].Address != "host-1:8020" {
		t.Fatalf("unexpected first endpoint: %#v", endpoints[0])
	}
	if endpoints[1].Name != "nn2" || endpoints[1].Address != "host-2:8020" {
		t.Fatalf("unexpected second endpoint: %#v", endpoints[1])
	}

	if _, err := ResolveHDFSHARPCAddresses("nn1,nn2", "host-1:8020", ""); err == nil {
		t.Fatal("expected error when second RPC address is missing")
	}
}
