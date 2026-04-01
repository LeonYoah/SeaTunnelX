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

import "testing"

func TestUnwrapPrecheckPayloadReturnsInnerMessage(t *testing.T) {
	raw := `{"success":true,"message":"{\"running\":false,\"status\":\"success\",\"exit_code\":0}"}`
	got := unwrapPrecheckPayload(raw)
	want := `{"running":false,"status":"success","exit_code":0}`
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestUnwrapPrecheckPayloadFallsBackToOriginalOutput(t *testing.T) {
	raw := `{"running":true,"status":"running"}`
	got := unwrapPrecheckPayload(raw)
	if got != raw {
		t.Fatalf("expected raw output, got %s", got)
	}
}
