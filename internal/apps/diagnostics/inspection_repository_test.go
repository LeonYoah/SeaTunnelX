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

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newInspectionTestRepository(t *testing.T) *Repository {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&ClusterInspectionReport{}, &ClusterInspectionFinding{}); err != nil {
		t.Fatalf("auto migrate inspection models: %v", err)
	}
	return NewRepository(database)
}

func TestInspectionRepositoryCreateAndListReportsAndFindings(t *testing.T) {
	repo := newInspectionTestRepository(t)
	ctx := t.Context()
	startedAt := time.Now().UTC().Add(-2 * time.Minute)
	finishedAt := startedAt.Add(time.Minute)

	report := &ClusterInspectionReport{
		ClusterID:         7,
		Status:            InspectionReportStatusCompleted,
		TriggerSource:     InspectionTriggerSourceDiagnosticsWorkspace,
		ErrorThreshold:    4,
		RequestedByUserID: 99,
		RequestedBy:       "admin",
		Summary:           "cluster inspection completed",
		FindingTotal:      2,
		CriticalCount:     1,
		WarningCount:      1,
		InfoCount:         0,
		StartedAt:         &startedAt,
		FinishedAt:        &finishedAt,
	}
	if err := repo.CreateInspectionReport(ctx, report); err != nil {
		t.Fatalf("create inspection report: %v", err)
	}

	findings := []*ClusterInspectionFinding{
		{
			ReportID:            report.ID,
			ClusterID:           7,
			Severity:            InspectionFindingSeverityCritical,
			Category:            "error_group",
			CheckCode:           "recent_error_burst",
			CheckName:           "Recent Error Burst",
			Summary:             "recent DEADLINE_EXCEEDED errors exceeded threshold",
			Recommendation:      "open error center and verify Milvus connectivity",
			EvidenceSummary:     "3 related error groups in last 10 minutes",
			RelatedErrorGroupID: 3,
		},
		{
			ReportID:        report.ID,
			ClusterID:       7,
			Severity:        InspectionFindingSeverityWarning,
			Category:        "node_health",
			CheckCode:       "offline_node",
			CheckName:       "Offline Node",
			Summary:         "worker node reported offline heartbeat",
			Recommendation:  "verify agent heartbeat and Seatunnel process state",
			EvidenceSummary: "node #4 heartbeat timeout",
			RelatedNodeID:   4,
			RelatedHostID:   4,
		},
	}
	if err := repo.CreateInspectionFindings(ctx, findings); err != nil {
		t.Fatalf("create inspection findings: %v", err)
	}

	reports, total, err := repo.ListInspectionReports(ctx, &ClusterInspectionReportFilter{
		ClusterID: 7,
		Status:    InspectionReportStatusCompleted,
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("list inspection reports: %v", err)
	}
	if total != 1 || len(reports) != 1 {
		t.Fatalf("expected 1 inspection report, got total=%d len=%d", total, len(reports))
	}
	if reports[0].RequestedBy != "admin" {
		t.Fatalf("expected requested_by admin, got %s", reports[0].RequestedBy)
	}
	if reports[0].ErrorThreshold != 4 {
		t.Fatalf("expected listed error_threshold=4, got %d", reports[0].ErrorThreshold)
	}

	gotReport, err := repo.GetInspectionReportByID(ctx, report.ID)
	if err != nil {
		t.Fatalf("get inspection report: %v", err)
	}
	if gotReport.FindingTotal != 2 || gotReport.CriticalCount != 1 || gotReport.WarningCount != 1 {
		t.Fatalf("unexpected report counters: %+v", gotReport)
	}
	if gotReport.ErrorThreshold != 4 {
		t.Fatalf("expected error_threshold=4, got %d", gotReport.ErrorThreshold)
	}

	reportFindings, totalFindings, err := repo.ListInspectionFindings(ctx, &ClusterInspectionFindingFilter{
		ReportID:  report.ID,
		ClusterID: 7,
		Page:      1,
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("list inspection findings: %v", err)
	}
	if totalFindings != 2 || len(reportFindings) != 2 {
		t.Fatalf("expected 2 findings, got total=%d len=%d", totalFindings, len(reportFindings))
	}

	errorGroupFindings, totalErrorGroupFindings, err := repo.ListInspectionFindings(ctx, &ClusterInspectionFindingFilter{
		ClusterID:           7,
		RelatedErrorGroupID: 3,
		Page:                1,
		PageSize:            20,
	})
	if err != nil {
		t.Fatalf("list findings by related error group: %v", err)
	}
	if totalErrorGroupFindings != 1 || len(errorGroupFindings) != 1 {
		t.Fatalf("expected 1 related error-group finding, got total=%d len=%d", totalErrorGroupFindings, len(errorGroupFindings))
	}
	if errorGroupFindings[0].CheckCode != "recent_error_burst" {
		t.Fatalf("unexpected finding check code: %s", errorGroupFindings[0].CheckCode)
	}
}
