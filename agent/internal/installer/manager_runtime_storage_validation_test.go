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

package installer

import "testing"

func TestCheckpointConfigValidateAcceptsHDFSHA(t *testing.T) {
	cfg := &CheckpointConfig{
		StorageType:             CheckpointStorageHDFS,
		Namespace:               "/seatunnel/checkpoint/",
		HDFSHAEnabled:           true,
		HDFSNameServices:        "cluster-a",
		HDFSHANamenodes:         "nn1,nn2",
		HDFSNamenodeRPCAddress1: "nn1.example.com:8020",
		HDFSNamenodeRPCAddress2: "nn2.example.com:8020",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected HA checkpoint config to pass validation, got: %v", err)
	}
}

func TestCheckpointConfigValidateRejectsIncompleteHDFSHA(t *testing.T) {
	cfg := &CheckpointConfig{
		StorageType:             CheckpointStorageHDFS,
		Namespace:               "/seatunnel/checkpoint/",
		HDFSHAEnabled:           true,
		HDFSNameServices:        "cluster-a",
		HDFSHANamenodes:         "nn1,nn2",
		HDFSNamenodeRPCAddress1: "nn1.example.com:8020",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected incomplete HA checkpoint config to be rejected")
	}
}

func TestIMAPConfigValidateRejectsIncompleteHDFSHA(t *testing.T) {
	cfg := &IMAPConfig{
		StorageType:             IMAPStorageHDFS,
		Namespace:               "/seatunnel/imap/",
		HDFSHAEnabled:           true,
		HDFSNameServices:        "cluster-a",
		HDFSHANamenodes:         "nn1,nn2",
		HDFSNamenodeRPCAddress1: "nn1.example.com:8020",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected incomplete HA IMAP config to be rejected")
	}
}
