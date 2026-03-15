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

package diagnostics

import "testing"

func TestBuildErrorFingerprintGroupsDeadlineExceededVariants(t *testing.T) {
	message1 := "DEADLINE_EXCEEDED: deadline exceeded after 9.996847312s. Name resolution delay 0.000990884 seconds. [closed=[], open=[[wait_for_ready, buffered_nanos=9999873497, waiting_for_connection]]]"
	message2 := "Failed to initialize connection. Error: DEADLINE_EXCEEDED: deadline exceeded after 9.988364578s. Name resolution delay 0.001774617 seconds. [closed=[], open=[[wait_for_ready, buffered_nanos=9989656015, waiting_for_connection]]]"
	evidence3 := "" +
		"[38.55.133.202]:5801 [seatunnel] [5.1] submit job 1083784734460346369 error org.apache.seatunnel.common.exception.SeaTunnelRuntimeException: ErrorCode:[API-09], ErrorDescription:[Handle save mode failed]\n" +
		"\tat org.apache.seatunnel.engine.server.master.JobMaster.handleSaveMode(JobMaster.java:573)\n" +
		"Caused by: java.lang.RuntimeException: Failed to initialize connection. Error: DEADLINE_EXCEEDED: deadline exceeded after 9.986412457s. Name resolution delay 0.003577568 seconds. [closed=[], open=[[wait_for_ready, buffered_nanos=9997862387, waiting_for_connection]]]"

	fp1, normalized1, _, title1 := BuildErrorFingerprint(message1, message1)
	fp2, normalized2, _, title2 := BuildErrorFingerprint(message2, message2)
	fp3, normalized3, _, title3 := BuildErrorFingerprint(message2, evidence3)

	if fp1 != fp2 || fp2 != fp3 {
		t.Fatalf("expected same fingerprint for deadline exceeded variants, got fp1=%s fp2=%s fp3=%s", fp1, fp2, fp3)
	}
	if title1 == "" || title2 == "" || title3 == "" {
		t.Fatalf("expected non-empty titles, got %q %q %q", title1, title2, title3)
	}
	if normalized1 != normalized2 || normalized2 != normalized3 {
		t.Fatalf("expected same normalized text for deadline exceeded variants")
	}
}

func TestBuildErrorFingerprintIgnoresWrapperNoiseAndUsesRealException(t *testing.T) {
	message := "Fatal Error,"
	evidence := "" +
		"===============================================================================\n" +
		"Fatal Error,\n" +
		"Please submit bug report in https://github.com/apache/seatunnel/issues\n" +
		"Reason:SeaTunnel job executed failed\n" +
		"Exception StackTrace:org.apache.seatunnel.core.starter.exception.CommandExecuteException: SeaTunnel job executed failed\n" +
		"\tat org.apache.seatunnel.core.starter.seatunnel.command.ClientExecuteCommand.execute(ClientExecuteCommand.java:228)\n" +
		"Caused by: java.lang.OutOfMemoryError: Java heap space\n"

	fp, normalized, exceptionClass, title := BuildErrorFingerprint(message, evidence)
	if fp == "" {
		t.Fatal("expected fingerprint for wrapped exception evidence")
	}
	if exceptionClass != "java.lang.OutOfMemoryError" {
		t.Fatalf("expected root exception class, got %q", exceptionClass)
	}
	if title != "java.lang.OutOfMemoryError: Java heap space" {
		t.Fatalf("unexpected title %q", title)
	}
	if normalized == "" {
		t.Fatal("expected normalized text")
	}
}

func TestBuildErrorFingerprintReturnsEmptyForNoiseOnlyEvidence(t *testing.T) {
	message := "Fatal Error,"
	evidence := "" +
		"===============================================================================\n" +
		"Fatal Error,\n" +
		"Please submit bug report in https://github.com/apache/seatunnel/issues\n" +
		"Reason:SeaTunnel job executed failed\n"

	fp, normalized, exceptionClass, title := BuildErrorFingerprint(message, evidence)
	if fp != "" || normalized != "" || exceptionClass != "" || title != "" {
		t.Fatalf("expected noise-only evidence to be ignored, got fp=%q normalized=%q exception=%q title=%q", fp, normalized, exceptionClass, title)
	}
}

func TestBuildErrorFingerprintReturnsEmptyForNoiseOnlyEvidenceWithSeatunnelLogHeader(t *testing.T) {
	message := "Fatal Error,"
	evidence := "" +
		"[] 2026-03-15 00:33:22,900 ERROR [o.a.s.c.s.SeaTunnel           ] [main] - Fatal Error,\n" +
		"===============================================================================\n" +
		"Fatal Error,\n" +
		"Please submit bug report in https://github.com/apache/seatunnel/issues\n" +
		"Reason:SeaTunnel job executed failed\n"

	fp, normalized, exceptionClass, title := BuildErrorFingerprint(message, evidence)
	if fp != "" || normalized != "" || exceptionClass != "" || title != "" {
		t.Fatalf("expected noise-only evidence with log header to be ignored, got fp=%q normalized=%q exception=%q title=%q", fp, normalized, exceptionClass, title)
	}
}
