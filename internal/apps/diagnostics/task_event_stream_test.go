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

func newDiagnosticTaskEventService(t *testing.T) *Service {
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
	return NewServiceWithRepository(NewRepository(database), nil, nil, nil)
}

func TestServiceDiagnosticTaskEventStreamReceivesUpdates(t *testing.T) {
	service := newDiagnosticTaskEventService(t)
	ctx := t.Context()

	task := &DiagnosticTask{
		ClusterID:     7,
		TriggerSource: DiagnosticTaskSourceManual,
		SourceRef:     DiagnosticTaskSourceRef{},
		Status:        DiagnosticTaskStatusPending,
		CurrentStep:   DiagnosticStepCodeCollectErrorContext,
		SelectedNodes: DiagnosticTaskNodeTargets{{ClusterNodeID: 1, NodeID: 4, HostID: 4, HostName: "node-a", HostIP: "10.0.0.4", Role: "worker"}},
		Summary:       "diagnostic task pending",
		CreatedBy:     1,
		CreatedByName: "admin",
	}
	if err := service.repo.CreateDiagnosticTask(ctx, task); err != nil {
		t.Fatalf("create diagnostics task: %v", err)
	}
	step := &DiagnosticTaskStep{
		TaskID:   task.ID,
		Code:     DiagnosticStepCodeCollectErrorContext,
		Sequence: 1,
		Title:    "汇总错误上下文",
		Status:   DiagnosticTaskStatusPending,
	}
	if err := service.repo.CreateDiagnosticTaskSteps(ctx, []*DiagnosticTaskStep{step}); err != nil {
		t.Fatalf("create diagnostics step: %v", err)
	}
	node := &DiagnosticNodeExecution{
		TaskID:        task.ID,
		TaskStepID:    &step.ID,
		ClusterNodeID: 1,
		NodeID:        4,
		HostID:        4,
		HostName:      "node-a",
		HostIP:        "10.0.0.4",
		Role:          "worker",
		Status:        DiagnosticTaskStatusPending,
		CurrentStep:   DiagnosticStepCodeCollectErrorContext,
	}
	if err := service.repo.CreateDiagnosticNodeExecutions(ctx, []*DiagnosticNodeExecution{node}); err != nil {
		t.Fatalf("create diagnostics node execution: %v", err)
	}

	events, cancel := service.SubscribeDiagnosticTaskEvents(task.ID)
	defer cancel()

	service.publishDiagnosticTaskEvent(newDiagnosticTaskSnapshotEvent(task))

	task.Status = DiagnosticTaskStatusRunning
	task.Summary = "diagnostic task running"
	if err := service.UpdateDiagnosticTask(ctx, task); err != nil {
		t.Fatalf("UpdateDiagnosticTask returned error: %v", err)
	}

	step.Status = DiagnosticTaskStatusRunning
	step.Message = "collecting error context"
	if err := service.UpdateDiagnosticTaskStep(ctx, step); err != nil {
		t.Fatalf("UpdateDiagnosticTaskStep returned error: %v", err)
	}

	node.Status = DiagnosticTaskStatusRunning
	node.Message = "node execution started"
	if err := service.UpdateDiagnosticNodeExecution(ctx, node); err != nil {
		t.Fatalf("UpdateDiagnosticNodeExecution returned error: %v", err)
	}

	if err := service.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
		TaskID:          task.ID,
		TaskStepID:      &step.ID,
		NodeExecutionID: &node.ID,
		StepCode:        DiagnosticStepCodeCollectErrorContext,
		Level:           DiagnosticLogLevelInfo,
		EventType:       DiagnosticLogEventTypeProgress,
		Message:         "error context collected",
	}); err != nil {
		t.Fatalf("AppendDiagnosticStepLog returned error: %v", err)
	}

	seen := map[DiagnosticTaskEventType]bool{}
	deadline := time.After(5 * time.Second)
	for !(seen[DiagnosticTaskEventTypeSnapshot] &&
		seen[DiagnosticTaskEventTypeTaskUpdated] &&
		seen[DiagnosticTaskEventTypeStepUpdated] &&
		seen[DiagnosticTaskEventTypeNodeUpdated] &&
		seen[DiagnosticTaskEventTypeLogAppended]) {
		select {
		case event := <-events:
			seen[event.EventType] = true
		case <-deadline:
			t.Fatalf("timed out waiting for diagnostics task events: %+v", seen)
		}
	}
}

func TestServiceSubscribeDiagnosticTaskEvents_initializesHubForZeroValueService(t *testing.T) {
	var service Service

	events, cancel := service.SubscribeDiagnosticTaskEvents(99)
	defer cancel()

	service.publishDiagnosticTaskEvent(DiagnosticTaskEvent{
		TaskID:     99,
		EventType:  DiagnosticTaskEventTypeTaskUpdated,
		TaskStatus: DiagnosticTaskStatusRunning,
	})

	select {
	case event := <-events:
		if event.TaskID != 99 {
			t.Fatalf("expected task id 99, got %d", event.TaskID)
		}
		if event.EventType != DiagnosticTaskEventTypeTaskUpdated {
			t.Fatalf("expected task_updated event, got %s", event.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for diagnostics task event from zero-value service")
	}
}

func TestNewDiagnosticsService_initializesTaskEventHub(t *testing.T) {
	service := newDiagnosticTaskEventService(t)
	if service.taskEvents == nil {
		t.Fatal("expected diagnostics service to initialize task event hub")
	}
}
