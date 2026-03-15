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

func newDiagnosticTaskTestRepository(t *testing.T) *Repository {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&DiagnosticTask{},
		&DiagnosticTaskStep{},
		&DiagnosticNodeExecution{},
		&DiagnosticStepLog{},
	); err != nil {
		t.Fatalf("auto migrate diagnostics task models: %v", err)
	}
	return NewRepository(database)
}

func TestDiagnosticTaskRepositoryCreateAndList(t *testing.T) {
	repo := newDiagnosticTaskTestRepository(t)
	ctx := t.Context()
	startedAt := time.Now().UTC().Add(-5 * time.Minute)
	completedAt := startedAt.Add(2 * time.Minute)

	task := &DiagnosticTask{
		ClusterID:     7,
		TriggerSource: DiagnosticTaskSourceInspectionFinding,
		SourceRef: DiagnosticTaskSourceRef{
			InspectionReportID:  11,
			InspectionFindingID: 13,
		},
		Status:        DiagnosticTaskStatusSucceeded,
		CurrentStep:   DiagnosticStepCodeComplete,
		SelectedNodes: DiagnosticTaskNodeTargets{{ClusterNodeID: 1, NodeID: 4, HostID: 4, HostName: "node-a", HostIP: "10.0.0.4", Role: "worker", AgentID: "agent-1", InstallDir: "/opt/seatunnel"}},
		Summary:       "inspection finding bundle created",
		StartedAt:     &startedAt,
		CompletedAt:   &completedAt,
		CreatedBy:     99,
		CreatedByName: "admin",
	}
	if err := repo.CreateDiagnosticTask(ctx, task); err != nil {
		t.Fatalf("create diagnostics task: %v", err)
	}

	steps := []*DiagnosticTaskStep{
		{
			TaskID:      task.ID,
			Code:        DiagnosticStepCodeCollectErrorContext,
			Sequence:    1,
			Title:       "汇总错误上下文",
			Description: "加载来源上下文",
			Status:      DiagnosticTaskStatusSucceeded,
		},
		{
			TaskID:      task.ID,
			Code:        DiagnosticStepCodeRenderHTMLSummary,
			Sequence:    2,
			Title:       "生成诊断报告",
			Description: "生成可离线查看的诊断报告",
			Status:      DiagnosticTaskStatusPending,
		},
	}
	if err := repo.CreateDiagnosticTaskSteps(ctx, steps); err != nil {
		t.Fatalf("create diagnostics task steps: %v", err)
	}

	nodes := []*DiagnosticNodeExecution{
		{
			TaskID:        task.ID,
			TaskStepID:    &steps[0].ID,
			ClusterNodeID: 1,
			NodeID:        4,
			HostID:        4,
			HostName:      "node-a",
			HostIP:        "10.0.0.4",
			Role:          "worker",
			AgentID:       "agent-1",
			InstallDir:    "/opt/seatunnel",
			Status:        DiagnosticTaskStatusSucceeded,
			CurrentStep:   DiagnosticStepCodeCollectErrorContext,
		},
	}
	if err := repo.CreateDiagnosticNodeExecutions(ctx, nodes); err != nil {
		t.Fatalf("create diagnostics node executions: %v", err)
	}

	if err := repo.CreateDiagnosticStepLog(ctx, &DiagnosticStepLog{
		TaskID:          task.ID,
		TaskStepID:      &steps[0].ID,
		NodeExecutionID: &nodes[0].ID,
		StepCode:        DiagnosticStepCodeCollectErrorContext,
		Level:           DiagnosticLogLevelInfo,
		EventType:       DiagnosticLogEventTypeSuccess,
		Message:         "error context collected",
		CommandSummary:  "collect diagnostic evidence",
		Metadata:        DiagnosticLogMetadata{"file_count": 2},
	}); err != nil {
		t.Fatalf("create diagnostics step log: %v", err)
	}

	gotTask, err := repo.GetDiagnosticTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("get diagnostics task by id: %v", err)
	}
	if gotTask.TriggerSource != DiagnosticTaskSourceInspectionFinding {
		t.Fatalf("unexpected task trigger source: %s", gotTask.TriggerSource)
	}
	if gotTask.SourceRef.InspectionFindingID != 13 {
		t.Fatalf("unexpected inspection finding reference: %+v", gotTask.SourceRef)
	}
	if len(gotTask.SelectedNodes) != 1 || gotTask.SelectedNodes[0].HostID != 4 {
		t.Fatalf("unexpected selected nodes snapshot: %+v", gotTask.SelectedNodes)
	}
	if len(gotTask.Steps) != 2 || len(gotTask.NodeExecutions) != 1 {
		t.Fatalf("unexpected preloaded relations: steps=%d nodes=%d", len(gotTask.Steps), len(gotTask.NodeExecutions))
	}

	items, total, err := repo.ListDiagnosticTasks(ctx, &DiagnosticTaskListFilter{
		ClusterID:     7,
		TriggerSource: DiagnosticTaskSourceInspectionFinding,
		Status:        DiagnosticTaskStatusSucceeded,
		Page:          1,
		PageSize:      20,
	})
	if err != nil {
		t.Fatalf("list diagnostics tasks: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 diagnostics task summary, got total=%d len=%d", total, len(items))
	}
	if items[0].CreatedByName != "admin" {
		t.Fatalf("unexpected diagnostics task summary: %+v", items[0])
	}

	logs, logTotal, err := repo.ListDiagnosticStepLogs(ctx, &DiagnosticTaskLogFilter{
		TaskID:   task.ID,
		StepCode: DiagnosticStepCodeCollectErrorContext,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list diagnostics step logs: %v", err)
	}
	if logTotal != 1 || len(logs) != 1 {
		t.Fatalf("expected 1 diagnostics step log, got total=%d len=%d", logTotal, len(logs))
	}
	if logs[0].CommandSummary != "collect diagnostic evidence" {
		t.Fatalf("unexpected diagnostics log: %+v", logs[0])
	}
}
