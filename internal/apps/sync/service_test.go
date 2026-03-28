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

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type stubConfigToolClient struct {
	webuiResp    *ConfigToolWebUIDAGResponse
	webuiErr     error
	dagResp      *ConfigToolDAGResponse
	dagErr       error
	validateResp *ConfigToolValidateResponse
	validateErr  error
}

func (s *stubConfigToolClient) InspectDAG(ctx context.Context, endpoint string, req *ConfigToolContentRequest) (*ConfigToolDAGResponse, error) {
	return s.dagResp, s.dagErr
}

func (s *stubConfigToolClient) InspectWebUIDAG(ctx context.Context, endpoint string, req *ConfigToolContentRequest) (*ConfigToolWebUIDAGResponse, error) {
	return s.webuiResp, s.webuiErr
}

func (s *stubConfigToolClient) ValidateConfig(ctx context.Context, endpoint string, req *ConfigToolValidateRequest) (*ConfigToolValidateResponse, error) {
	return s.validateResp, s.validateErr
}

func (s *stubConfigToolClient) DeriveSourcePreview(ctx context.Context, endpoint string, req *ConfigToolPreviewRequest) (*ConfigToolPreviewResponse, error) {
	return nil, nil
}

func (s *stubConfigToolClient) DeriveTransformPreview(ctx context.Context, endpoint string, req *ConfigToolPreviewRequest) (*ConfigToolPreviewResponse, error) {
	return nil, nil
}

type stubConfigToolResolver struct {
	endpoint string
	err      error
}

func (s *stubConfigToolResolver) ResolveConfigToolEndpoint(ctx context.Context, clusterID uint, taskDefinition JSONMap) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.endpoint, nil
}

type stubAgentSender struct {
	success bool
	output  string
	err     error
}

func (s *stubAgentSender) SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (bool, string, error) {
	return s.success, s.output, s.err
}

type stubExecutionTargetResolver struct {
	targets []*ExecutionTarget
	err     error
}

func (s *stubExecutionTargetResolver) ResolveExecutionTarget(ctx context.Context, clusterID uint, definition JSONMap) (*ExecutionTarget, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.targets) == 0 {
		return nil, ErrExecutionTargetUnavailable
	}
	return s.targets[0], nil
}

func (s *stubExecutionTargetResolver) ResolveExecutionTargets(ctx context.Context, clusterID uint, definition JSONMap) ([]*ExecutionTarget, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.targets) == 0 {
		return nil, ErrExecutionTargetUnavailable
	}
	return s.targets, nil
}

func newTestSyncService(t *testing.T) *Service {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&Task{}, &TaskVersion{}, &JobInstance{}, &GlobalVariable{}, &PreviewSession{}, &PreviewTable{}, &PreviewRow{}); err != nil {
		t.Fatalf("failed to migrate sync models: %v", err)
	}

	return NewService(NewRepository(database))
}

func uintPtr(value uint) *uint { return &value }

func TestCreateTaskRejectsUnsupportedName(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "root",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}

	_, err = service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "bad name",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1)
	if !errors.Is(err, ErrTaskNameInvalid) {
		t.Fatalf("expected ErrTaskNameInvalid, got %v", err)
	}
}

func TestUpdateTaskRejectsMovingFolderIntoDescendant(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	root, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "root",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create root folder failed: %v", err)
	}
	child, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(root.ID),
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "child",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}

	_, err = service.UpdateTask(ctx, root.ID, &UpdateTaskRequest{
		ParentID:      uintPtr(child.ID),
		NodeType:      string(TaskNodeTypeFolder),
		Name:          root.Name,
		ContentFormat: string(ContentFormatHOCON),
	})
	if !errors.Is(err, ErrTaskParentCycle) {
		t.Fatalf("expected ErrTaskParentCycle, got %v", err)
	}
}

func TestUpdateTaskAllowsMovingFileToFolder(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "folder_a",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	sourceFolder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "folder_b",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create source folder failed: %v", err)
	}
	file, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(sourceFolder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "job_1",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}

	updated, err := service.UpdateTask(ctx, file.ID, &UpdateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          file.Name,
		ContentFormat: string(ContentFormatHOCON),
		Content:       file.Content,
		Definition:    file.Definition,
	})
	if err != nil {
		t.Fatalf("move file failed: %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != folder.ID {
		t.Fatalf("expected parent_id=%d, got %+v", folder.ID, updated.ParentID)
	}
}

func TestCreateTaskRejectsRootFile(t *testing.T) {
	service := newTestSyncService(t)

	_, err := service.CreateTask(context.Background(), &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFile),
		Name:          "root_job",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1)
	if !errors.Is(err, ErrRootFileNotAllowed) {
		t.Fatalf("expected ErrRootFileNotAllowed, got %v", err)
	}
}

func TestCreateTaskRejectsDuplicateNameInSameFolder(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "dup_root",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	if _, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "same_name",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1); err != nil {
		t.Fatalf("create first file failed: %v", err)
	}
	_, err = service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "same_name",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if !errors.Is(err, ErrTaskNameDuplicate) {
		t.Fatalf("expected ErrTaskNameDuplicate, got %v", err)
	}
}

func TestUpdateTaskRejectsDuplicateSiblingName(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "rename_root",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	left, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "left_job",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1)
	if err != nil {
		t.Fatalf("create left file failed: %v", err)
	}
	right, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "right_job",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1)
	if err != nil {
		t.Fatalf("create right file failed: %v", err)
	}
	_, err = service.UpdateTask(ctx, right.ID, &UpdateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          left.Name,
		ContentFormat: string(ContentFormatHOCON),
		Content:       right.Content,
		Definition:    right.Definition,
	})
	if !errors.Is(err, ErrTaskNameDuplicate) {
		t.Fatalf("expected ErrTaskNameDuplicate, got %v", err)
	}
}

func TestDetectTemplateVariablesUsesPlatformSyntaxOnly(t *testing.T) {
	vars := detectTemplateVariables("{{ current_env }} ${seatunnel.builtin} {{job.name}}")
	if len(vars) != 2 {
		t.Fatalf("expected 2 variables, got %v", vars)
	}
	if vars[0] != "current_env" || vars[1] != "job.name" {
		t.Fatalf("unexpected variables: %v", vars)
	}
}

func TestDeleteTaskRemovesDescendantsAndRuntimeArtifacts(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	root, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "root_delete",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create root failed: %v", err)
	}
	file, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(root.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "job_delete",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
	}, 1)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}
	if _, _, err := service.PublishTask(ctx, file.ID, "test", 1); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := service.repo.CreateJobInstance(ctx, &JobInstance{TaskID: file.ID, TaskVersion: 1, RunType: RunTypeRun, Status: JobStatusSuccess}); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}

	if err := service.DeleteTask(ctx, root.ID); err != nil {
		t.Fatalf("delete task failed: %v", err)
	}

	if _, err := service.repo.GetTaskByID(ctx, root.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected root deleted, got %v", err)
	}
	if _, err := service.repo.GetTaskByID(ctx, file.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected child file deleted, got %v", err)
	}
	jobs, total, err := service.repo.ListJobInstances(ctx, &JobFilter{TaskID: file.ID, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("list jobs failed: %v", err)
	}
	if total != 0 || len(jobs) != 0 {
		t.Fatalf("expected job instances deleted, total=%d jobs=%d", total, len(jobs))
	}
}

func TestBuildTaskDAGPrefersWebUICompatibleDag(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "workspace",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	file, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "demo_job",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
		ClusterID:     11,
	}, 1)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}

	service.SetConfigToolClient(&stubConfigToolClient{
		webuiResp: &ConfigToolWebUIDAGResponse{
			JobID:     "preview",
			JobName:   "Config Preview",
			JobStatus: "CREATED",
			JobDag: ConfigToolWebUIJobDAG{
				JobID: "preview",
				PipelineEdges: map[string][]ConfigToolWebUIDAGEdge{
					"0": []ConfigToolWebUIDAGEdge{{InputVertexID: 1, TargetVertexID: 2}},
				},
				VertexInfoMap: map[string]ConfigToolWebUIDAGVertexInfo{
					"1": {VertexID: 1, Type: "source", ConnectorType: "Source[0]-FakeSource", TablePaths: []string{"fake"}},
					"2": {VertexID: 2, Type: "sink", ConnectorType: "Sink[0]-Console", TablePaths: []string{"fake"}},
				},
			},
			Metrics:     map[string]interface{}{"SourceReceivedCount": "0"},
			Warnings:    []string{"preview warning"},
			SimpleGraph: true,
		},
	})
	service.SetConfigToolResolver(&stubConfigToolResolver{endpoint: "http://127.0.0.1:18080"})

	result, err := service.BuildTaskDAG(ctx, file.ID)
	if err != nil {
		t.Fatalf("BuildTaskDAG returned error: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result.Nodes))
	}
	if len(result.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(result.Edges))
	}
	if result.WebUIJob == nil {
		t.Fatal("expected webui_job to be populated")
	}
	if result.WebUIJob["jobName"] != "Config Preview" {
		t.Fatalf("unexpected jobName: %#v", result.WebUIJob["jobName"])
	}
	if !result.SimpleGraph {
		t.Fatal("expected simple_graph to be true")
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "preview warning" {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
}

func TestValidateTaskUsesConfigToolValidation(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "workspace",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	file, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "demo_job",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
		ClusterID:     11,
	}, 1)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}

	service.SetConfigToolClient(&stubConfigToolClient{
		validateResp: &ConfigToolValidateResponse{
			OK:      true,
			Valid:   true,
			Summary: "Config validation finished.",
			Warnings: []string{
				"connector warning",
			},
			Checks: []ConfigToolValidationCheck{{
				NodeID:        "source-0",
				Kind:          "source",
				ConnectorType: "Source[0]-Jdbc",
				Target:        "jdbc:mysql://127.0.0.1:3307/seatunnel_demo",
				Status:        "success",
				Message:       "Connection succeeded.",
			}},
		},
	})
	service.SetConfigToolResolver(&stubConfigToolResolver{endpoint: "http://127.0.0.1:18080"})

	result, err := service.ValidateTask(ctx, file.ID)
	if err != nil {
		t.Fatalf("ValidateTask returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid result, got %#v", result.Errors)
	}
	if len(result.Checks) != 1 || result.Checks[0].ConnectorType != "Source[0]-Jdbc" {
		t.Fatalf("unexpected checks: %#v", result.Checks)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings from config tool validation")
	}
}

func TestTestTaskConnectionsReturnsConfigToolChecks(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	folder, err := service.CreateTask(ctx, &CreateTaskRequest{
		NodeType:      string(TaskNodeTypeFolder),
		Name:          "workspace",
		ContentFormat: string(ContentFormatHOCON),
	}, 1)
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	file, err := service.CreateTask(ctx, &CreateTaskRequest{
		ParentID:      uintPtr(folder.ID),
		NodeType:      string(TaskNodeTypeFile),
		Name:          "demo_job",
		ContentFormat: string(ContentFormatHOCON),
		Content:       "env {}",
		Definition:    JSONMap{},
		ClusterID:     11,
	}, 1)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}

	service.SetConfigToolClient(&stubConfigToolClient{
		validateResp: &ConfigToolValidateResponse{
			OK:      false,
			Valid:   false,
			Summary: "Connection test finished.",
			Errors:  []string{"Sink[0]-Jdbc 连接失败: Access denied"},
			Checks: []ConfigToolValidationCheck{{
				NodeID:        "sink-0",
				Kind:          "sink",
				ConnectorType: "Sink[0]-Jdbc",
				Target:        "jdbc:mysql://127.0.0.1:3307/demo2",
				Status:        "failed",
				Message:       "Access denied",
			}},
		},
	})
	service.SetConfigToolResolver(&stubConfigToolResolver{endpoint: "http://127.0.0.1:18080"})

	result, err := service.TestTaskConnections(ctx, file.ID)
	if err != nil {
		t.Fatalf("TestTaskConnections returned error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	if len(result.Errors) != 1 {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
	if len(result.Checks) != 1 || result.Checks[0].Status != "failed" {
		t.Fatalf("unexpected checks: %#v", result.Checks)
	}
}

func TestResolveTaskContentAppliesGlobalAndCustomVariables(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	if _, err := service.CreateGlobalVariable(ctx, &CreateGlobalVariableRequest{
		Key:   "global_env",
		Value: "prod",
	}, 1); err != nil {
		t.Fatalf("create global variable failed: %v", err)
	}

	task := &Task{
		Name:          "demo",
		ContentFormat: ContentFormatHOCON,
		Content:       "env = {{global_env}}\nsource = {{custom_name}}\nkeep = ${seatunnel.native}",
		Definition: JSONMap{
			"custom_variables": map[string]interface{}{
				"custom_name": "orders",
			},
		},
	}
	resolved, err := service.resolveTaskContent(ctx, task)
	if err != nil {
		t.Fatalf("resolve task content failed: %v", err)
	}
	if resolved != "env = prod\nsource = orders\nkeep = ${seatunnel.native}" {
		t.Fatalf("unexpected resolved content: %q", resolved)
	}
}

func TestResolveTaskContentPrefersCustomVariablesAndKeepsComplexValues(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	if _, err := service.CreateGlobalVariable(ctx, &CreateGlobalVariableRequest{
		Key:   "jdbc_url",
		Value: "jdbc://mysql:3306/global",
	}, 1); err != nil {
		t.Fatalf("create global variable failed: %v", err)
	}
	if _, err := service.CreateGlobalVariable(ctx, &CreateGlobalVariableRequest{
		Key:   "query_text",
		Value: `select * from "global.table"`,
	}, 1); err != nil {
		t.Fatalf("create global variable failed: %v", err)
	}

	task := &Task{
		Name:          "complex",
		ContentFormat: ContentFormatHOCON,
		Content:       "url = {{jdbc_url}}\nquery = {{query_text}}",
		Definition: JSONMap{
			"custom_variables": map[string]interface{}{
				"jdbc_url":   "jdbc://mysql:3306/test",
				"query_text": `select * from "aa.test"`,
			},
		},
	}
	resolved, err := service.resolveTaskContent(ctx, task)
	if err != nil {
		t.Fatalf("resolve task content failed: %v", err)
	}
	expected := "url = jdbc://mysql:3306/test\nquery = select * from \"aa.test\""
	if resolved != expected {
		t.Fatalf("unexpected resolved content: %q", resolved)
	}
}

func TestMergeLogChunksPreservesRepeatedLinesAcrossNodes(t *testing.T) {
	got := mergeLogChunks([]string{
		"2026-03-27 15:16:00 INFO start\n2026-03-27 15:16:01 WARN retry",
		"2026-03-27 15:16:01 WARN retry\n2026-03-27 15:16:02 ERROR failed",
	})

	expected := "2026-03-27 15:16:00 INFO start\n2026-03-27 15:16:01 WARN retry\n2026-03-27 15:16:01 WARN retry\n2026-03-27 15:16:02 ERROR failed"
	if got != expected {
		t.Fatalf("unexpected merged logs:\n%s", got)
	}
}

func TestGetTaskTreeAutoMovesRootFilesIntoWorkspaceFolder(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()

	if err := service.repo.CreateTask(ctx, &Task{
		NodeType:      TaskNodeTypeFile,
		Name:          "legacy_root_job",
		ContentFormat: ContentFormatHOCON,
		Content:       "env {}",
		Status:        TaskStatusDraft,
	}); err != nil {
		t.Fatalf("seed root file failed: %v", err)
	}

	tree, err := service.GetTaskTree(ctx)
	if err != nil {
		t.Fatalf("get task tree failed: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("expected exactly one root folder after normalization, got %d", len(tree))
	}
	if tree[0].NodeType != TaskNodeTypeFolder {
		t.Fatalf("expected root node to be folder, got %s", tree[0].NodeType)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Name != "legacy_root_job" {
		t.Fatalf("expected root file moved under workspace folder, got %+v", tree[0].Children)
	}
}

func TestGetJobLogsReturnsEmptyPayloadWhenLevelFilterHasNoMatches(t *testing.T) {
	service := newTestSyncService(t)
	service.SetAgentCommandSender(&stubAgentSender{
		success: true,
		output:  `{"success":true,"message":"{\"logs\":\"\",\"path\":\"/opt/seatunnel/logs/job-177.log\",\"next_offset\":\"128\",\"file_size\":128}"}`,
	})
	service.SetExecutionTargetResolver(&stubExecutionTargetResolver{
		targets: []*ExecutionTarget{{
			AgentID:    "agent-1",
			InstallDir: "/opt/seatunnel",
			HostID:     1,
		}},
	})
	ctx := context.Background()
	if err := service.repo.CreateJobInstance(ctx, &JobInstance{
		TaskID:        1,
		TaskVersion:   1,
		RunType:       RunTypeRun,
		Status:        JobStatusRunning,
		PlatformJobID: "177",
		EngineJobID:   "177",
		SubmitSpec:    JSONMap{"cluster_id": 11, "target_agent_id": "agent-1", "install_dir": "/opt/seatunnel"},
		ResultPreview: JSONMap{},
		ErrorMessage:  "",
		CreatedBy:     1,
	}); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}
	result, err := service.GetJobLogs(ctx, 1, "", 64*1024, "", "error")
	if err != nil {
		t.Fatalf("GetJobLogs returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected logs result, got nil")
	}
	if result.Logs != "" {
		t.Fatalf("expected empty logs, got %q", result.Logs)
	}
}

func TestGetJobLogsTreatsLegacyAgentPayloadWithoutPathAsAvailable(t *testing.T) {
	service := newTestSyncService(t)
	service.SetAgentCommandSender(&stubAgentSender{
		success: true,
		output:  `{"success":true,"message":"{\"logs\":\"\",\"next_offset\":\"128\",\"file_size\":128}"}`,
	})
	service.SetExecutionTargetResolver(&stubExecutionTargetResolver{
		targets: []*ExecutionTarget{{
			AgentID:    "agent-1",
			InstallDir: "/opt/seatunnel",
			HostID:     1,
		}},
	})
	ctx := context.Background()
	if err := service.repo.CreateJobInstance(ctx, &JobInstance{
		TaskID:        1,
		TaskVersion:   1,
		RunType:       RunTypeRun,
		Status:        JobStatusRunning,
		PlatformJobID: "177",
		EngineJobID:   "177",
		SubmitSpec:    JSONMap{"cluster_id": 11, "target_agent_id": "agent-1", "install_dir": "/opt/seatunnel"},
		ResultPreview: JSONMap{},
		CreatedBy:     1,
	}); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}
	result, err := service.GetJobLogs(ctx, 1, "", 64*1024, "", "error")
	if err != nil {
		t.Fatalf("GetJobLogs returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected logs result, got nil")
	}
	if result.NextOffset == "" {
		t.Fatalf("expected next offset to be present for legacy payload")
	}
}

func TestCollectPreviewAppendsRowsIntoPreviewSession(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	now := time.Now()
	instance := &JobInstance{
		TaskID:        7,
		TaskVersion:   1,
		RunType:       RunTypePreview,
		Status:        JobStatusRunning,
		PlatformJobID: "preview-1",
		EngineJobID:   "preview-1",
		SubmitSpec:    JSONMap{},
		ResultPreview: JSONMap{},
		StartedAt:     &now,
		CreatedBy:     1,
	}
	if err := service.repo.CreateJobInstance(ctx, instance); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}
	if err := service.repo.CreatePreviewSession(ctx, &PreviewSession{
		JobInstanceID: instance.ID,
		TaskID:        instance.TaskID,
		PlatformJobID: instance.PlatformJobID,
		EngineJobID:   instance.EngineJobID,
		RowLimit:      3,
		Status:        "collecting",
		StartedAt:     &now,
	}); err != nil {
		t.Fatalf("create preview session failed: %v", err)
	}

	if err := service.CollectPreview(ctx, &PreviewCollectRequest{
		PlatformJobID: "preview-1",
		Dataset:       "seatunnel_demo.users",
		Columns:       []interface{}{"id", "name"},
		Rows: []map[string]interface{}{
			{"id": 1, "name": "a"},
			{"id": 2, "name": "b"},
		},
		RowLimit: 3,
	}); err != nil {
		t.Fatalf("collect preview failed: %v", err)
	}

	snapshot, err := service.GetPreviewSnapshot(ctx, instance.ID, "seatunnel_demo.users")
	if err != nil {
		t.Fatalf("get preview snapshot failed: %v", err)
	}
	if snapshot.TotalRows != 2 {
		t.Fatalf("expected total rows 2, got %d", snapshot.TotalRows)
	}
	if snapshot.SelectedTable == nil || len(snapshot.SelectedTable.Rows) != 2 {
		t.Fatalf("expected two selected table rows, got %+v", snapshot.SelectedTable)
	}
}

func TestCollectPreviewStopsAtRowLimit(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	now := time.Now()
	instance := &JobInstance{
		TaskID:        8,
		TaskVersion:   1,
		RunType:       RunTypePreview,
		Status:        JobStatusRunning,
		PlatformJobID: "preview-2",
		EngineJobID:   "preview-2",
		SubmitSpec:    JSONMap{},
		ResultPreview: JSONMap{},
		StartedAt:     &now,
		CreatedBy:     1,
	}
	if err := service.repo.CreateJobInstance(ctx, instance); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}
	if err := service.repo.CreatePreviewSession(ctx, &PreviewSession{
		JobInstanceID: instance.ID,
		TaskID:        instance.TaskID,
		PlatformJobID: instance.PlatformJobID,
		EngineJobID:   instance.EngineJobID,
		RowLimit:      1,
		Status:        "collecting",
		StartedAt:     &now,
	}); err != nil {
		t.Fatalf("create preview session failed: %v", err)
	}

	if err := service.CollectPreview(ctx, &PreviewCollectRequest{
		PlatformJobID: "preview-2",
		Dataset:       "seatunnel_demo.users",
		Columns:       []interface{}{"id"},
		Rows: []map[string]interface{}{
			{"id": 1},
			{"id": 2},
		},
		RowLimit: 1,
	}); err != nil {
		t.Fatalf("collect preview failed: %v", err)
	}

	snapshot, err := service.GetPreviewSnapshot(ctx, instance.ID, "seatunnel_demo.users")
	if err != nil {
		t.Fatalf("get preview snapshot failed: %v", err)
	}
	if !snapshot.Truncated {
		t.Fatalf("expected preview snapshot to be truncated")
	}
	if snapshot.TotalRows != 1 {
		t.Fatalf("expected total rows 1, got %d", snapshot.TotalRows)
	}
	job, err := service.GetJob(ctx, instance.ID)
	if err != nil {
		t.Fatalf("get job failed: %v", err)
	}
	if job.Status != JobStatusCanceled {
		t.Fatalf("expected preview job canceled after reaching row limit, got %s", job.Status)
	}
}

func TestGetPreviewSnapshotReturnsEmptySnapshotWhenSessionNotReady(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	now := time.Now()
	instance := &JobInstance{
		TaskID:        9,
		TaskVersion:   1,
		RunType:       RunTypePreview,
		Status:        JobStatusRunning,
		PlatformJobID: "preview-empty",
		EngineJobID:   "preview-empty",
		SubmitSpec: JSONMap{
			"row_limit":       100,
			"timeout_minutes": 10,
		},
		ResultPreview: JSONMap{},
		StartedAt:     &now,
		CreatedBy:     1,
	}
	if err := service.repo.CreateJobInstance(ctx, instance); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}

	snapshot, err := service.GetPreviewSnapshot(ctx, instance.ID, "")
	if err != nil {
		t.Fatalf("get preview snapshot failed: %v", err)
	}
	if snapshot.EmptyReason != "preview_not_ready" {
		t.Fatalf("expected preview_not_ready, got %q", snapshot.EmptyReason)
	}
	if len(snapshot.Tables) != 0 {
		t.Fatalf("expected no preview tables, got %d", len(snapshot.Tables))
	}
}

func TestGetJobLogsReturnsEmptyResultWhenUnavailable(t *testing.T) {
	service := newTestSyncService(t)
	ctx := context.Background()
	instance := &JobInstance{
		TaskID:        10,
		TaskVersion:   1,
		RunType:       RunTypeRun,
		Status:        JobStatusSuccess,
		PlatformJobID: "run-no-logs",
		EngineJobID:   "run-no-logs",
		SubmitSpec: JSONMap{
			"execution_mode": "cluster",
		},
		ResultPreview: JSONMap{},
		CreatedBy:     1,
	}
	if err := service.repo.CreateJobInstance(ctx, instance); err != nil {
		t.Fatalf("create job instance failed: %v", err)
	}

	result, err := service.GetJobLogs(ctx, instance.ID, "", 64*1024, "", "")
	if err != nil {
		t.Fatalf("get job logs failed: %v", err)
	}
	if result.EmptyReason != "logs_not_ready" {
		t.Fatalf("expected logs_not_ready, got %q", result.EmptyReason)
	}
	if result.Logs != "" {
		t.Fatalf("expected empty logs, got %q", result.Logs)
	}
}
