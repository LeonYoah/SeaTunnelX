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
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var safeTaskNamePattern = regexp.MustCompile(`^[\p{L}\p{N}._-]+$`)

// Service provides sync studio control-plane behavior.
type Service struct {
	repo                    *Repository
	engineClient            EngineClient
	runtimeResolver         ClusterRuntimeResolver
	configToolClient        ConfigToolClient
	configToolResolver      ConfigToolResolver
	jobIDGenerator          *JobIDGenerator
	agentSender             AgentCommandSender
	executionTargetResolver ExecutionTargetResolver
	clusterLogProvider      ClusterLogProvider
}

// NewService creates a new sync service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo, jobIDGenerator: NewJobIDGenerator()}
}

// SetEngineClient sets the SeaTunnel engine client used by submit/get/cancel flows.
func (s *Service) SetEngineClient(client EngineClient) { s.engineClient = client }

// SetRuntimeResolver sets the runtime endpoint resolver for cluster-backed submissions.
func (s *Service) SetRuntimeResolver(resolver ClusterRuntimeResolver) { s.runtimeResolver = resolver }

// SetConfigToolClient sets the java-proxy config tool client.
func (s *Service) SetConfigToolClient(client ConfigToolClient) { s.configToolClient = client }

// SetConfigToolResolver sets the java-proxy endpoint resolver.
func (s *Service) SetConfigToolResolver(resolver ConfigToolResolver) { s.configToolResolver = resolver }

// SetJobIDGenerator sets the platform job id generator.
func (s *Service) SetJobIDGenerator(generator *JobIDGenerator) { s.jobIDGenerator = generator }

// ListGlobalVariables returns all workspace-wide variables.
func (s *Service) ListGlobalVariables(ctx context.Context) ([]*GlobalVariable, error) {
	return s.repo.ListGlobalVariables(ctx)
}

// ListGlobalVariablesPaginated returns paginated workspace-wide variables.
func (s *Service) ListGlobalVariablesPaginated(ctx context.Context, page, size int) ([]*GlobalVariable, int64, error) {
	return s.repo.ListGlobalVariablesPaginated(ctx, page, size)
}

// CreateGlobalVariable creates one workspace-wide variable.
func (s *Service) CreateGlobalVariable(ctx context.Context, req *CreateGlobalVariableRequest, createdBy uint) (*GlobalVariable, error) {
	key, err := normalizeGlobalVariableKey(req.Key)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetGlobalVariableByKey(ctx, key); err == nil {
		return nil, ErrGlobalVariableKeyDuplicate
	} else if err != nil && !errors.Is(err, ErrGlobalVariableNotFound) {
		return nil, err
	}
	item := &GlobalVariable{
		Key:         key,
		Value:       req.Value,
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   createdBy,
	}
	if err := s.repo.CreateGlobalVariable(ctx, item); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrGlobalVariableKeyDuplicate
		}
		return nil, err
	}
	return item, nil
}

// UpdateGlobalVariable updates one workspace-wide variable.
func (s *Service) UpdateGlobalVariable(ctx context.Context, id uint, req *UpdateGlobalVariableRequest) (*GlobalVariable, error) {
	item, err := s.repo.GetGlobalVariableByID(ctx, id)
	if err != nil {
		return nil, err
	}
	key, err := normalizeGlobalVariableKey(req.Key)
	if err != nil {
		return nil, err
	}
	if other, err := s.repo.GetGlobalVariableByKey(ctx, key); err == nil && other.ID != id {
		return nil, ErrGlobalVariableKeyDuplicate
	} else if err != nil && !errors.Is(err, ErrGlobalVariableNotFound) {
		return nil, err
	}
	item.Key = key
	item.Value = req.Value
	item.Description = strings.TrimSpace(req.Description)
	if err := s.repo.UpdateGlobalVariable(ctx, item); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrGlobalVariableKeyDuplicate
		}
		return nil, err
	}
	return item, nil
}

// DeleteGlobalVariable deletes one workspace-wide variable.
func (s *Service) DeleteGlobalVariable(ctx context.Context, id uint) error {
	return s.repo.DeleteGlobalVariable(ctx, id)
}

// CreateTask creates one workspace node.
func (s *Service) CreateTask(ctx context.Context, req *CreateTaskRequest, createdBy uint) (*Task, error) {
	nodeType, err := normalizeNodeType(req.NodeType)
	if err != nil {
		return nil, err
	}
	mode, err := normalizeTaskMode(req.Mode)
	if err != nil {
		return nil, err
	}
	format, err := normalizeContentFormat(req.ContentFormat)
	if err != nil {
		return nil, err
	}
	parentID, err := s.validateParent(ctx, req.ParentID)
	if err != nil {
		return nil, err
	}
	if nodeType == TaskNodeTypeFile && (parentID == nil || *parentID == 0) {
		return nil, ErrRootFileNotAllowed
	}
	name, err := normalizeTaskName(req.Name)
	if err != nil {
		return nil, err
	}
	definition := cloneJSONMap(req.Definition)
	content := strings.TrimSpace(req.Content)
	jobName := strings.TrimSpace(req.JobName)
	if nodeType == TaskNodeTypeFolder {
		content = ""
		jobName = ""
	} else if content == "" {
		return nil, ErrTaskDefinitionEmpty
	}
	task := &Task{
		ParentID:       parentID,
		NodeType:       nodeType,
		Name:           name,
		Description:    strings.TrimSpace(req.Description),
		ClusterID:      req.ClusterID,
		EngineVersion:  strings.TrimSpace(req.EngineVersion),
		Mode:           mode,
		Status:         TaskStatusDraft,
		ContentFormat:  format,
		Content:        content,
		JobName:        jobName,
		Definition:     definition,
		SortOrder:      req.SortOrder,
		CurrentVersion: 0,
		CreatedBy:      createdBy,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// ListTasks returns paginated workspace nodes.
func (s *Service) ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, int64, error) {
	return s.repo.ListTasks(ctx, filter)
}

// GetTask returns one workspace node.
func (s *Service) GetTask(ctx context.Context, id uint) (*Task, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	return task, nil
}

// GetTaskTree returns nested workspace nodes for the left tree.
func (s *Service) GetTaskTree(ctx context.Context) ([]*TaskTreeNode, error) {
	if err := s.ensureRootFilesNested(ctx); err != nil {
		return nil, err
	}
	tasks, err := s.repo.ListAllTasks(ctx)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		s.applyTaskDefaults(task)
	}
	return buildTaskTree(tasks), nil
}

// UpdateTask updates one workspace node.
func (s *Service) UpdateTask(ctx context.Context, id uint, req *UpdateTaskRequest) (*Task, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task.Status == TaskStatusArchived {
		return nil, ErrTaskArchived
	}
	mode, err := normalizeTaskMode(req.Mode)
	if err != nil {
		return nil, err
	}
	nodeType, err := normalizeNodeType(defaultString(req.NodeType, string(task.NodeType)))
	if err != nil {
		return nil, err
	}
	format, err := normalizeContentFormat(defaultString(req.ContentFormat, string(task.ContentFormat)))
	if err != nil {
		return nil, err
	}
	parentID, err := s.validateParentForTask(ctx, id, nodeType, req.ParentID)
	if err != nil {
		return nil, err
	}
	if nodeType == TaskNodeTypeFile && (parentID == nil || *parentID == 0) {
		return nil, ErrRootFileNotAllowed
	}
	name, err := normalizeTaskName(req.Name)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(req.Content)
	jobName := strings.TrimSpace(req.JobName)
	if nodeType == TaskNodeTypeFolder {
		content = ""
		jobName = ""
	} else if content == "" {
		return nil, ErrTaskDefinitionEmpty
	}
	task.ParentID = parentID
	task.NodeType = nodeType
	task.Name = name
	task.Description = strings.TrimSpace(req.Description)
	task.ClusterID = req.ClusterID
	task.EngineVersion = strings.TrimSpace(req.EngineVersion)
	task.Mode = mode
	task.ContentFormat = format
	task.Content = content
	task.JobName = jobName
	task.SortOrder = req.SortOrder
	if req.Definition == nil {
		task.Definition = JSONMap{}
	} else {
		task.Definition = cloneJSONMap(req.Definition)
	}
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// DeleteTask removes one workspace node and all nested descendants.
func (s *Service) DeleteTask(ctx context.Context, id uint) error {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return err
	}
	s.applyTaskDefaults(task)

	tasks, err := s.repo.ListAllTasks(ctx)
	if err != nil {
		return err
	}
	targetIDs := collectTaskSubtreeIDs(tasks, id)
	if len(targetIDs) == 0 {
		targetIDs = []uint{id}
	}

	return s.repo.Transaction(ctx, func(tx *Repository) error {
		if err := tx.DeleteTaskVersionsByTaskIDs(ctx, targetIDs); err != nil {
			return err
		}
		if err := tx.DeleteJobInstancesByTaskIDs(ctx, targetIDs); err != nil {
			return err
		}
		return tx.DeleteTasksByIDs(ctx, targetIDs)
	})
}

// PublishTask snapshots current file definition and marks task as published.
func (s *Service) PublishTask(ctx context.Context, id uint, comment string, createdBy uint) (*Task, *TaskVersion, error) {
	var publishedTask *Task
	var version *TaskVersion
	err := s.repo.Transaction(ctx, func(tx *Repository) error {
		task, err := tx.GetTaskByID(ctx, id)
		if err != nil {
			return err
		}
		s.applyTaskDefaults(task)
		if task.Status == TaskStatusArchived {
			return ErrTaskArchived
		}
		if task.NodeType != TaskNodeTypeFile {
			return ErrTaskNotFile
		}
		version = &TaskVersion{
			TaskID:                task.ID,
			Version:               task.CurrentVersion + 1,
			NameSnapshot:          task.Name,
			DescriptionSnapshot:   task.Description,
			ClusterIDSnapshot:     task.ClusterID,
			EngineVersionSnapshot: task.EngineVersion,
			ModeSnapshot:          task.Mode,
			ContentFormatSnapshot: task.ContentFormat,
			ContentSnapshot:       task.Content,
			JobNameSnapshot:       task.JobName,
			DefinitionSnapshot:    cloneJSONMap(task.Definition),
			Comment:               strings.TrimSpace(comment),
			CreatedBy:             createdBy,
		}
		if err := tx.CreateTaskVersion(ctx, version); err != nil {
			return err
		}
		task.CurrentVersion = version.Version
		task.Status = TaskStatusPublished
		if err := tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		publishedTask = task
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return publishedTask, version, nil
}

// ListTaskVersions returns all immutable snapshots for one file task.
func (s *Service) ListTaskVersions(ctx context.Context, id uint) ([]*TaskVersion, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	return s.repo.ListTaskVersionsByTaskID(ctx, id)
}

// ListTaskVersionsPaginated returns paginated immutable snapshots for one file task.
func (s *Service) ListTaskVersionsPaginated(ctx context.Context, id uint, page, size int) ([]*TaskVersion, int64, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, 0, ErrTaskNotFile
	}
	return s.repo.ListTaskVersionsByTaskIDPaginated(ctx, id, page, size)
}

// RollbackTaskVersion restores one immutable snapshot back to the editable task.
func (s *Service) RollbackTaskVersion(ctx context.Context, id uint, versionID uint) (*Task, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	version, err := s.repo.GetTaskVersionByID(ctx, id, versionID)
	if err != nil {
		return nil, err
	}
	task.Name = version.NameSnapshot
	task.Description = version.DescriptionSnapshot
	task.ClusterID = version.ClusterIDSnapshot
	task.EngineVersion = version.EngineVersionSnapshot
	task.Mode = version.ModeSnapshot
	task.ContentFormat = version.ContentFormatSnapshot
	task.Content = version.ContentSnapshot
	task.JobName = version.JobNameSnapshot
	task.Definition = cloneJSONMap(version.DefinitionSnapshot)
	task.Status = TaskStatusDraft
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

// DeleteTaskVersion removes one immutable snapshot.
func (s *Service) DeleteTaskVersion(ctx context.Context, id uint, versionID uint) error {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return ErrTaskNotFile
	}
	return s.repo.DeleteTaskVersion(ctx, id, versionID)
}

// ValidateTask validates current file content.
func (s *Service) ValidateTask(ctx context.Context, id uint) (*ValidateResult, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	errorsList := []string{}
	warnings := []string{}
	if strings.TrimSpace(task.Content) == "" {
		errorsList = append(errorsList, ErrTaskDefinitionEmpty.Error())
	}
	if task.ClusterID == 0 && taskExecutionMode(task) != "local" {
		warnings = append(warnings, "cluster_id is empty, runtime submission will fail until a target cluster is selected")
	}
	if s.configToolClient != nil && s.configToolResolver != nil {
		endpoint, endpointErr := s.configToolResolver.ResolveConfigToolEndpoint(ctx, task.ClusterID, task.Definition)
		if endpointErr == nil {
			req, buildErr := s.buildConfigToolContentRequest(ctx, task)
			if buildErr == nil {
				validateResp, validateErr := s.configToolClient.ValidateConfig(ctx, endpoint, &ConfigToolValidateRequest{
					ConfigToolContentRequest: *req,
					TestConnection:           false,
				})
				if validateErr != nil {
					return nil, validateErr
				}
				errorsList = append(errorsList, validateResp.Errors...)
				warnings = append(warnings, validateResp.Warnings...)
				return &ValidateResult{
					Valid:        validateResp.Valid && len(errorsList) == 0,
					Errors:       dedupeStrings(errorsList),
					Warnings:     dedupeStrings(warnings),
					Summary:      defaultString(validateResp.Summary, "Studio validation finished."),
					Resolved:     map[string]string{"mode": string(task.Mode), "content_format": string(task.ContentFormat)},
					DetectedVars: detectTemplateVariables(task.Content),
					Checks:       toValidateChecks(validateResp.Checks),
				}, nil
			}
		}
	}
	return &ValidateResult{
		Valid:        len(errorsList) == 0,
		Errors:       errorsList,
		Warnings:     warnings,
		Summary:      "Studio validation finished.",
		Resolved:     map[string]string{"mode": string(task.Mode), "content_format": string(task.ContentFormat)},
		DetectedVars: detectTemplateVariables(task.Content),
	}, nil
}

// TestTaskConnections validates connector connectivity for the task definition.
func (s *Service) TestTaskConnections(ctx context.Context, id uint) (*ValidateResult, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	if s.configToolClient == nil || s.configToolResolver == nil {
		return &ValidateResult{
			Valid:    false,
			Errors:   []string{"sync: config tool resolver is not configured"},
			Summary:  "Connection test failed.",
			Resolved: map[string]string{"mode": string(task.Mode), "content_format": string(task.ContentFormat)},
		}, nil
	}
	endpoint, err := s.configToolResolver.ResolveConfigToolEndpoint(ctx, task.ClusterID, task.Definition)
	if err != nil {
		return nil, err
	}
	req, err := s.buildConfigToolContentRequest(ctx, task)
	if err != nil {
		return nil, err
	}
	validateResp, err := s.configToolClient.ValidateConfig(ctx, endpoint, &ConfigToolValidateRequest{
		ConfigToolContentRequest: *req,
		TestConnection:           true,
	})
	if err != nil {
		return nil, err
	}
	return &ValidateResult{
		Valid:        validateResp.Valid,
		Errors:       append([]string{}, validateResp.Errors...),
		Warnings:     append([]string{}, validateResp.Warnings...),
		Summary:      defaultString(validateResp.Summary, "Connection test finished."),
		Resolved:     map[string]string{"mode": string(task.Mode), "content_format": string(task.ContentFormat)},
		DetectedVars: detectTemplateVariables(task.Content),
		Checks:       toValidateChecks(validateResp.Checks),
	}, nil
}

// BuildTaskDAG returns DAG projection for the task definition.
func (s *Service) BuildTaskDAG(ctx context.Context, id uint) (*DAGResult, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	if s.configToolClient != nil && s.configToolResolver != nil {
		endpoint, err := s.configToolResolver.ResolveConfigToolEndpoint(ctx, task.ClusterID, task.Definition)
		if err == nil {
			req, buildErr := s.buildConfigToolContentRequest(ctx, task)
			if buildErr == nil {
				webuiResp, dagErr := s.configToolClient.InspectWebUIDAG(ctx, endpoint, req)
				if dagErr == nil && webuiResp != nil {
					rawWebUIJob, marshalErr := structToJSONMap(webuiResp)
					if marshalErr == nil {
						return &DAGResult{
							Nodes:       extractWebUIDAGNodes(webuiResp.JobDag.VertexInfoMap),
							Edges:       extractWebUIDAGEdges(webuiResp.JobDag.PipelineEdges),
							WebUIJob:    rawWebUIJob,
							SimpleGraph: webuiResp.SimpleGraph,
							Warnings:    append([]string{}, webuiResp.Warnings...),
						}, nil
					}
				}
				if dagErr != nil {
					return nil, dagErr
				}
			}
		}
	}
	if dag, ok := task.Definition["dag"].(map[string]interface{}); ok {
		return &DAGResult{Nodes: toJSONMapSlice(dag["nodes"]), Edges: toJSONMapSlice(dag["edges"])}, nil
	}
	return &DAGResult{Nodes: []JSONMap{{"id": fmt.Sprintf("task-%d", task.ID), "name": task.Name, "type": "pipeline"}}, Edges: []JSONMap{}}, nil
}

func toValidateChecks(items []ConfigToolValidationCheck) []ValidateCheck {
	if len(items) == 0 {
		return nil
	}
	result := make([]ValidateCheck, 0, len(items))
	for _, item := range items {
		result = append(result, ValidateCheck{
			NodeID:        item.NodeID,
			Kind:          item.Kind,
			ConnectorType: item.ConnectorType,
			Target:        item.Target,
			Status:        item.Status,
			Message:       item.Message,
		})
	}
	return result
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// PreviewTask derives preview config then submits a debug run if possible.
func (s *Service) PreviewTask(ctx context.Context, id uint, createdBy uint) (*JobInstance, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	now := time.Now()
	previewPayload, previewFormat, previewResult, err := s.derivePreviewPayload(ctx, task)
	if err != nil {
		return nil, err
	}
	instance := &JobInstance{
		TaskID:        task.ID,
		TaskVersion:   task.CurrentVersion,
		RunType:       RunTypePreview,
		Status:        JobStatusSuccess,
		SubmitSpec:    JSONMap{"mode": task.Mode, "preview": true, "preview_format": previewFormat},
		ResultPreview: JSONMap{"rows": []map[string]interface{}{}},
		StartedAt:     &now,
		CreatedBy:     createdBy,
	}
	if taskExecutionMode(task) == "local" {
		if len(previewPayload) == 0 {
			return nil, ErrPreviewHTTPSinkEmpty
		}
		instance, err := s.submitLocalTaskInstance(ctx, task, createdBy, RunTypePreview, s.nextJobID(), previewPayload, previewFormat, s.previewJobName(task))
		if err != nil {
			return nil, err
		}
		if previewResult != nil {
			instance.ResultPreview = mergePreviewDeriveMetadata(instance.ResultPreview, previewResult)
			_ = s.repo.UpdateJobInstance(ctx, instance)
		}
		return instance, nil
	}
	if len(previewPayload) == 0 {
		instance.ResultPreview["note"] = "preview placeholder created; configure preview_http_sink to enable engine-backed debug preview"
		instance.FinishedAt = &now
	} else {
		instance.PlatformJobID = s.nextJobID()
		instance.Status = JobStatusRunning
		instance.ResultPreview["note"] = "preview config derived and submitted to engine"
		instance.ResultPreview["payload_bytes"] = len(previewPayload)
		instance.ResultPreview["detected_vars"] = detectTemplateVariables(task.Content)
		if previewResult != nil {
			if previewResult.ContentFormat != "" {
				instance.ResultPreview["content_format"] = previewResult.ContentFormat
			}
			if len(previewResult.Warnings) > 0 {
				instance.ResultPreview["warnings"] = previewResult.Warnings
			}
			if len(previewResult.Graph.Nodes) > 0 {
				instance.ResultPreview["graph"] = map[string]interface{}{"nodes": previewResult.Graph.Nodes, "edges": previewResult.Graph.Edges}
			}
		}
		if s.engineClient != nil && s.runtimeResolver != nil {
			endpoint, err := s.runtimeResolver.ResolveEngineEndpoint(ctx, task.ClusterID, task.Definition)
			if err != nil {
				return nil, err
			}
			resp, err := s.engineClient.Submit(ctx, &EngineSubmitRequest{Endpoint: endpoint, Format: previewFormat, JobID: instance.PlatformJobID, JobName: s.previewJobName(task), Body: previewPayload})
			if err != nil {
				return nil, err
			}
			instance.EngineJobID = strings.TrimSpace(resp.JobID)
			instance.SubmitSpec["engine_api_mode"] = defaultString(resp.APIMode, "v2")
			instance.SubmitSpec["engine_base_url"] = defaultString(resp.EndpointBaseURL, endpoint.BaseURL)
			if strings.EqualFold(defaultString(resp.APIMode, "v2"), "v1") {
				instance.SubmitSpec["engine_legacy_base_url"] = defaultString(resp.EndpointBaseURL, endpoint.LegacyURL)
			} else if endpoint.ContextPath != "" {
				instance.SubmitSpec["engine_context_path"] = endpoint.ContextPath
			}
		} else {
			instance.Status = JobStatusSuccess
			instance.FinishedAt = &now
		}
	}
	if err := s.repo.CreateJobInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

// SubmitTask creates one formal run instance with platform-managed job id.
func (s *Service) SubmitTask(ctx context.Context, id uint, createdBy uint) (*JobInstance, error) {
	task, err := s.repo.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	if task.CurrentVersion == 0 {
		return nil, ErrTaskNotPublished
	}
	if taskExecutionMode(task) == "local" {
		body, format, jobName, buildErr := s.buildSubmitPayload(ctx, task)
		if buildErr != nil {
			return nil, buildErr
		}
		return s.submitLocalTaskInstance(ctx, task, createdBy, RunTypeRun, s.nextJobID(), body, format, jobName)
	}
	return s.submitTaskInstance(ctx, task, createdBy, RunTypeRun, s.nextJobID(), false, nil)
}

// RecoverJob recreates a run using the original platform job id as savepoint recovery source.
func (s *Service) RecoverJob(ctx context.Context, sourceJobID uint, createdBy uint) (*JobInstance, error) {
	source, err := s.repo.GetJobInstanceByID(ctx, sourceJobID)
	if err != nil {
		return nil, err
	}
	if source.TaskID == 0 || strings.TrimSpace(source.PlatformJobID) == "" {
		return nil, ErrRecoverSourceRequired
	}
	task, err := s.repo.GetTaskByID(ctx, source.TaskID)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(task)
	if task.NodeType != TaskNodeTypeFile {
		return nil, ErrTaskNotFile
	}
	if task.CurrentVersion == 0 {
		return nil, ErrTaskNotPublished
	}
	if taskExecutionMode(task) == "local" {
		return nil, ErrLocalSavepointUnsupported
	}
	return s.submitTaskInstance(ctx, task, createdBy, RunTypeRecover, strings.TrimSpace(source.PlatformJobID), true, &source.ID)
}

func (s *Service) submitTaskInstance(ctx context.Context, task *Task, createdBy uint, runType RunType, platformJobID string, startWithSavepoint bool, recoveredFrom *uint) (*JobInstance, error) {
	submitBody, submitFormat, jobName, err := s.buildSubmitPayload(ctx, task)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	instance := &JobInstance{
		TaskID:                  task.ID,
		TaskVersion:             task.CurrentVersion,
		RunType:                 runType,
		PlatformJobID:           platformJobID,
		RecoveredFromInstanceID: recoveredFrom,
		Status:                  JobStatusRunning,
		SubmitSpec: JSONMap{
			"mode":                 task.Mode,
			"engine_version":       task.EngineVersion,
			"cluster_id":           task.ClusterID,
			"format":               submitFormat,
			"job_name":             jobName,
			"platform_job_id":      platformJobID,
			"start_with_savepoint": startWithSavepoint,
		},
		StartedAt: &now,
		CreatedBy: createdBy,
	}
	if s.engineClient != nil && s.runtimeResolver != nil {
		endpoint, err := s.runtimeResolver.ResolveEngineEndpoint(ctx, task.ClusterID, task.Definition)
		if err != nil {
			return nil, err
		}
		if s.executionTargetResolver != nil {
			if target, targetErr := s.executionTargetResolver.ResolveExecutionTarget(ctx, task.ClusterID, task.Definition); targetErr == nil && target != nil {
				instance.SubmitSpec["target_node_id"] = target.NodeID
				instance.SubmitSpec["target_host_id"] = target.HostID
				instance.SubmitSpec["target_agent_id"] = target.AgentID
				instance.SubmitSpec["install_dir"] = target.InstallDir
			}
		}
		resp, err := s.engineClient.Submit(ctx, &EngineSubmitRequest{Endpoint: endpoint, Format: submitFormat, JobID: platformJobID, JobName: jobName, StartWithSavepoint: startWithSavepoint, Body: submitBody})
		if err != nil {
			return nil, err
		}
		instance.EngineJobID = strings.TrimSpace(resp.JobID)
		instance.SubmitSpec["engine_api_mode"] = defaultString(resp.APIMode, "v2")
		instance.SubmitSpec["engine_base_url"] = defaultString(resp.EndpointBaseURL, endpoint.BaseURL)
		if strings.EqualFold(defaultString(resp.APIMode, "v2"), "v1") {
			instance.SubmitSpec["engine_legacy_base_url"] = defaultString(resp.EndpointBaseURL, endpoint.LegacyURL)
		} else if endpoint.ContextPath != "" {
			instance.SubmitSpec["engine_context_path"] = endpoint.ContextPath
		}
	} else {
		instance.EngineJobID = platformJobID
	}
	if err := s.repo.CreateJobInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

// ListJobs returns paginated job instances.
func (s *Service) ListJobs(ctx context.Context, filter *JobFilter) ([]*JobInstance, int64, error) {
	return s.repo.ListJobInstances(ctx, filter)
}

// GetJob returns one job instance and refreshes live runtime state when possible.
func (s *Service) GetJob(ctx context.Context, id uint) (*JobInstance, error) {
	instance, err := s.repo.GetJobInstanceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if submitSpecExecutionMode(instance.SubmitSpec) == "local" {
		return s.refreshLocalJob(ctx, instance)
	}
	if s.engineClient == nil || strings.TrimSpace(instance.EngineJobID) == "" {
		return instance, nil
	}
	if instance.Status == JobStatusSuccess || instance.Status == JobStatusFailed || instance.Status == JobStatusCanceled {
		return instance, nil
	}
	endpoint := endpointFromSubmitSpec(instance.SubmitSpec)
	if endpoint == nil {
		return instance, nil
	}
	info, err := s.engineClient.GetJobInfo(ctx, endpoint, instance.EngineJobID)
	if err != nil {
		return instance, nil
	}
	instance.Status = normalizeJobStatus(info.JobStatus)
	instance.ResultPreview = mergeJobRuntimeInfo(instance.ResultPreview, info)
	if instance.Status == JobStatusSuccess || instance.Status == JobStatusFailed || instance.Status == JobStatusCanceled {
		now := time.Now()
		instance.FinishedAt = &now
		if info.ErrorMsg != nil {
			instance.ErrorMessage = strings.TrimSpace(fmt.Sprintf("%v", info.ErrorMsg))
		}
	}
	if err := s.repo.UpdateJobInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

// CancelJob marks one running/pending job instance as canceled.
func (s *Service) CancelJob(ctx context.Context, id uint, stopWithSavepoint bool) (*JobInstance, error) {
	instance, err := s.repo.GetJobInstanceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if instance.Status == JobStatusSuccess || instance.Status == JobStatusFailed || instance.Status == JobStatusCanceled {
		return nil, ErrJobAlreadyFinished
	}
	if submitSpecExecutionMode(instance.SubmitSpec) == "local" {
		if err := s.stopLocalJob(ctx, instance); err != nil {
			return nil, err
		}
	} else if s.engineClient != nil && strings.TrimSpace(instance.EngineJobID) != "" {
		if endpoint := endpointFromSubmitSpec(instance.SubmitSpec); endpoint != nil {
			if err := s.engineClient.StopJob(ctx, endpoint, instance.EngineJobID, stopWithSavepoint); err != nil {
				return nil, err
			}
		}
	}
	now := time.Now()
	instance.Status = JobStatusCanceled
	instance.FinishedAt = &now
	instance.ResultPreview = mergeJobRuntimeInfo(instance.ResultPreview, &EngineJobInfo{
		JobID:        instance.EngineJobID,
		JobName:      strings.TrimSpace(stringValue(instance.SubmitSpec, "job_name")),
		JobStatus:    "CANCELED",
		FinishedTime: now.Format(time.DateTime),
	})
	if err := s.repo.UpdateJobInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

func (s *Service) validateParent(ctx context.Context, parentID *uint) (*uint, error) {
	if parentID == nil || *parentID == 0 {
		return nil, nil
	}
	parent, err := s.repo.GetTaskByID(ctx, *parentID)
	if err != nil {
		return nil, err
	}
	s.applyTaskDefaults(parent)
	if parent.NodeType != TaskNodeTypeFolder {
		return nil, ErrParentTaskNotFolder
	}
	return parentID, nil
}

func (s *Service) validateParentForTask(ctx context.Context, taskID uint, nodeType TaskNodeType, parentID *uint) (*uint, error) {
	validatedParentID, err := s.validateParent(ctx, parentID)
	if err != nil {
		return nil, err
	}
	if validatedParentID == nil || *validatedParentID == 0 {
		return nil, nil
	}
	if *validatedParentID == taskID {
		return nil, ErrTaskParentCycle
	}
	if nodeType != TaskNodeTypeFolder {
		return validatedParentID, nil
	}
	cursor := validatedParentID
	for cursor != nil && *cursor != 0 {
		if *cursor == taskID {
			return nil, ErrTaskParentCycle
		}
		parent, err := s.repo.GetTaskByID(ctx, *cursor)
		if err != nil {
			return nil, err
		}
		cursor = parent.ParentID
	}
	return validatedParentID, nil
}

func (s *Service) applyTaskDefaults(task *Task) {
	if task == nil {
		return
	}
	if task.NodeType == "" {
		task.NodeType = TaskNodeTypeFile
	}
	if task.ContentFormat == "" {
		task.ContentFormat = ContentFormatHOCON
	}
	if task.Mode == "" {
		task.Mode = TaskModeBatch
	}
	if task.Definition == nil {
		task.Definition = JSONMap{}
	}
}

func (s *Service) nextJobID() string {
	if s.jobIDGenerator == nil {
		s.jobIDGenerator = NewJobIDGenerator()
	}
	return s.jobIDGenerator.NextJobID()
}

func (s *Service) previewJobName(task *Task) string {
	base := strings.TrimSpace(task.JobName)
	if base == "" {
		base = strings.TrimSpace(task.Name)
	}
	if base == "" {
		base = "sync-preview"
	}
	return base + "-preview"
}

func normalizeTaskMode(raw string) (TaskMode, error) {
	switch strings.TrimSpace(raw) {
	case "", string(TaskModeBatch):
		return TaskModeBatch, nil
	case string(TaskModeStreaming):
		return TaskModeStreaming, nil
	default:
		return "", ErrInvalidTaskMode
	}
}

func normalizeTaskName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrTaskNameRequired
	}
	if !safeTaskNamePattern.MatchString(name) {
		return "", ErrTaskNameInvalid
	}
	return name, nil
}

func normalizeNodeType(raw string) (TaskNodeType, error) {
	switch strings.TrimSpace(raw) {
	case "", string(TaskNodeTypeFile):
		return TaskNodeTypeFile, nil
	case string(TaskNodeTypeFolder):
		return TaskNodeTypeFolder, nil
	default:
		return "", ErrInvalidNodeType
	}
}

func normalizeContentFormat(raw string) (ContentFormat, error) {
	switch normalizeSubmitFormat(raw) {
	case "hocon":
		return ContentFormatHOCON, nil
	case "json":
		return ContentFormatJSON, nil
	default:
		return "", ErrInvalidContentFormat
	}
}

func cloneJSONMap(src JSONMap) JSONMap {
	if src == nil {
		return JSONMap{}
	}
	cloned := make(JSONMap, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func buildTaskTree(tasks []*Task) []*TaskTreeNode {
	nodes := make(map[uint]*TaskTreeNode, len(tasks))
	roots := make([]*TaskTreeNode, 0)
	for _, task := range tasks {
		nodes[task.ID] = &TaskTreeNode{ID: task.ID, ParentID: task.ParentID, NodeType: task.NodeType, Name: task.Name, Description: task.Description, ClusterID: task.ClusterID, EngineVersion: task.EngineVersion, Mode: task.Mode, Status: task.Status, ContentFormat: task.ContentFormat, Content: task.Content, JobName: task.JobName, Definition: cloneJSONMap(task.Definition), SortOrder: task.SortOrder, CurrentVersion: task.CurrentVersion, Children: []*TaskTreeNode{}}
	}
	for _, task := range tasks {
		node := nodes[task.ID]
		if task.ParentID != nil {
			if parent, ok := nodes[*task.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	sortTreeNodes(roots)
	return roots
}

func collectTaskSubtreeIDs(tasks []*Task, rootID uint) []uint {
	if len(tasks) == 0 {
		return nil
	}
	childrenByParent := make(map[uint][]uint)
	for _, task := range tasks {
		if task.ParentID != nil {
			childrenByParent[*task.ParentID] = append(childrenByParent[*task.ParentID], task.ID)
		}
	}
	result := make([]uint, 0)
	stack := []uint{rootID}
	seen := make(map[uint]struct{})
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		result = append(result, current)
		stack = append(stack, childrenByParent[current]...)
	}
	return result
}

func sortTreeNodes(nodes []*TaskTreeNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].SortOrder != nodes[j].SortOrder {
			return nodes[i].SortOrder < nodes[j].SortOrder
		}
		if nodes[i].NodeType != nodes[j].NodeType {
			return nodes[i].NodeType < nodes[j].NodeType
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
	for _, node := range nodes {
		if len(node.Children) > 0 {
			sortTreeNodes(node.Children)
		}
	}
}

func toJSONMapSlice(value interface{}) []JSONMap {
	items, ok := value.([]interface{})
	if !ok {
		return []JSONMap{}
	}
	result := make([]JSONMap, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, JSONMap(m))
		}
	}
	return result
}

func toJSONMapSliceFromMaps(items []map[string]interface{}) []JSONMap {
	result := make([]JSONMap, 0, len(items))
	for _, item := range items {
		result = append(result, JSONMap(item))
	}
	return result
}

func structToJSONMap(value interface{}) (JSONMap, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result JSONMap
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func extractWebUIDAGNodes(vertexInfoMap map[string]ConfigToolWebUIDAGVertexInfo) []JSONMap {
	type keyedVertex struct {
		key   string
		value ConfigToolWebUIDAGVertexInfo
	}
	keyed := make([]keyedVertex, 0, len(vertexInfoMap))
	for key, value := range vertexInfoMap {
		keyed = append(keyed, keyedVertex{key: key, value: value})
	}
	sort.SliceStable(keyed, func(i, j int) bool {
		if keyed[i].value.VertexID != keyed[j].value.VertexID {
			return keyed[i].value.VertexID < keyed[j].value.VertexID
		}
		return keyed[i].key < keyed[j].key
	})
	nodes := make([]JSONMap, 0, len(keyed))
	for _, item := range keyed {
		nodes = append(nodes, JSONMap{
			"id":            item.value.VertexID,
			"vertexId":      item.value.VertexID,
			"name":          item.value.ConnectorType,
			"type":          item.value.Type,
			"connectorType": item.value.ConnectorType,
			"tablePaths":    append([]string{}, item.value.TablePaths...),
		})
	}
	return nodes
}

func extractWebUIDAGEdges(pipelineEdges map[string][]ConfigToolWebUIDAGEdge) []JSONMap {
	keys := make([]string, 0, len(pipelineEdges))
	for key := range pipelineEdges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	edges := make([]JSONMap, 0)
	for _, pipelineID := range keys {
		for _, edge := range pipelineEdges[pipelineID] {
			edges = append(edges, JSONMap{
				"pipelineId":     pipelineID,
				"inputVertexId":  edge.InputVertexID,
				"targetVertexId": edge.TargetVertexID,
				"source":         edge.InputVertexID,
				"target":         edge.TargetVertexID,
			})
		}
	}
	return edges
}

func (s *Service) buildSubmitPayload(ctx context.Context, task *Task) ([]byte, string, string, error) {
	if task == nil {
		return nil, "", "", fmt.Errorf("sync: task is required")
	}
	format := normalizeSubmitFormat(string(task.ContentFormat))
	jobName := strings.TrimSpace(task.JobName)
	if jobName == "" {
		jobName = strings.TrimSpace(task.Name)
	}
	resolvedContent, err := s.resolveTaskContent(ctx, task)
	if err != nil {
		return nil, "", "", err
	}
	if raw := strings.TrimSpace(resolvedContent); raw != "" {
		return []byte(raw), format, jobName, nil
	}
	if payload := mapValue(task.Definition, "pipeline_json", "config_json"); payload != nil {
		body, err := jsonMarshal(payload)
		return body, "json", jobName, err
	}
	return nil, "", "", ErrTaskDefinitionEmpty
}

func buildPreviewPayload(definition JSONMap) ([]byte, string) {
	if definition == nil {
		return nil, ""
	}
	if raw := strings.TrimSpace(stringValue(definition, "preview_pipeline_text", "preview_config_text")); raw != "" {
		return []byte(raw), normalizeSubmitFormat(stringValue(definition, "preview_format", "format"))
	}
	if payload := mapValue(definition, "preview_pipeline_json", "preview_config_json"); payload != nil {
		body, err := jsonMarshal(payload)
		if err == nil {
			return body, "json"
		}
	}
	return nil, ""
}

func (s *Service) buildConfigToolContentRequest(ctx context.Context, task *Task) (*ConfigToolContentRequest, error) {
	if task == nil {
		return nil, ErrTaskDefinitionEmpty
	}
	resolvedContent, err := s.resolveTaskContent(ctx, task)
	if err != nil {
		return nil, err
	}
	if raw := strings.TrimSpace(resolvedContent); raw != "" {
		return &ConfigToolContentRequest{Content: raw, ContentFormat: normalizeSubmitFormat(string(task.ContentFormat)), Variables: stringSliceValue(task.Definition, "variables")}, nil
	}
	if path := strings.TrimSpace(stringValue(task.Definition, "file_path", "config_file_path")); path != "" {
		return &ConfigToolContentRequest{FilePath: path, ContentFormat: normalizeSubmitFormat(string(task.ContentFormat)), Variables: stringSliceValue(task.Definition, "variables")}, nil
	}
	return nil, ErrTaskDefinitionEmpty
}

func (s *Service) derivePreviewPayload(ctx context.Context, task *Task) ([]byte, string, *ConfigToolPreviewResponse, error) {
	if payload, format := buildPreviewPayload(task.Definition); len(payload) > 0 {
		return payload, format, nil, nil
	}
	if s.configToolClient == nil || s.configToolResolver == nil {
		return nil, "", nil, nil
	}
	endpoint, err := s.configToolResolver.ResolveConfigToolEndpoint(ctx, task.ClusterID, task.Definition)
	if err != nil {
		return nil, "", nil, nil
	}
	req, err := s.buildConfigToolPreviewRequest(ctx, task)
	if err != nil {
		return nil, "", nil, err
	}
	mode := strings.ToLower(strings.TrimSpace(stringValue(task.Definition, "preview_mode")))
	var resp *ConfigToolPreviewResponse
	switch mode {
	case "", "source":
		resp, err = s.configToolClient.DeriveSourcePreview(ctx, endpoint, req)
	case "transform":
		resp, err = s.configToolClient.DeriveTransformPreview(ctx, endpoint, req)
	default:
		return nil, "", nil, ErrInvalidPreviewMode
	}
	if err != nil {
		return nil, "", nil, err
	}
	if resp == nil || !resp.OK || strings.TrimSpace(resp.Content) == "" {
		return nil, "", nil, fmt.Errorf("sync: preview derive returned empty content")
	}
	return []byte(resp.Content), normalizeSubmitFormat(resp.ContentFormat), resp, nil
}

func (s *Service) buildConfigToolPreviewRequest(ctx context.Context, task *Task) (*ConfigToolPreviewRequest, error) {
	contentReq, err := s.buildConfigToolContentRequest(ctx, task)
	if err != nil {
		return nil, err
	}
	httpSink := mapValue(task.Definition, "preview_http_sink", "http_sink")
	if len(httpSink) == 0 {
		return nil, ErrPreviewHTTPSinkEmpty
	}
	req := &ConfigToolPreviewRequest{ConfigToolContentRequest: *contentReq, OutputFormat: normalizePreviewOutputFormat(stringValue(task.Definition, "preview_output_format")), HttpSink: httpSink, EnvOverrides: mapValue(task.Definition, "preview_env_overrides", "env_overrides"), MetadataFields: mapValue(task.Definition, "preview_metadata_fields", "metadata_fields"), MetadataOutputDataset: strings.TrimSpace(stringValue(task.Definition, "preview_metadata_output_dataset")), SourceNodeID: strings.TrimSpace(stringValue(task.Definition, "preview_source_node_id", "source_node_id")), TransformNodeID: strings.TrimSpace(stringValue(task.Definition, "preview_transform_node_id", "transform_node_id"))}
	if index, ok := intValue(task.Definition, "preview_source_index", "source_index"); ok {
		req.SourceIndex = &index
	}
	if index, ok := intValue(task.Definition, "preview_transform_index", "transform_index"); ok {
		req.TransformIndex = &index
	}
	return req, nil
}

func endpointFromSubmitSpec(spec JSONMap) *EngineEndpoint {
	if spec == nil {
		return nil
	}
	baseURL := strings.TrimSpace(stringValue(spec, "engine_base_url"))
	legacyURL := strings.TrimSpace(stringValue(spec, "engine_legacy_base_url"))
	apiMode := strings.TrimSpace(stringValue(spec, "engine_api_mode"))
	if baseURL == "" && legacyURL == "" {
		return nil
	}
	endpoint := &EngineEndpoint{
		BaseURL:     baseURL,
		ContextPath: strings.TrimSpace(stringValue(spec, "engine_context_path")),
		LegacyURL:   legacyURL,
		APIMode:     apiMode,
	}
	if endpoint.BaseURL == "" && strings.EqualFold(endpoint.APIMode, "v1") {
		endpoint.BaseURL = endpoint.LegacyURL
	}
	if endpoint.LegacyURL == "" && strings.EqualFold(endpoint.APIMode, "v1") {
		endpoint.LegacyURL = endpoint.BaseURL
	}
	return endpoint
}

func mergeJobRuntimeInfo(existing JSONMap, info *EngineJobInfo) JSONMap {
	if existing == nil {
		existing = JSONMap{}
	}
	if info == nil {
		return existing
	}
	if strings.TrimSpace(info.JobStatus) != "" {
		existing["job_status"] = strings.TrimSpace(info.JobStatus)
	}
	if info.JobDag != nil {
		existing["job_dag"] = info.JobDag
	}
	if info.Metrics != nil {
		existing["metrics"] = info.Metrics
	}
	if info.CreateTime != "" {
		existing["create_time"] = info.CreateTime
	}
	if info.FinishedTime != "" {
		existing["finished_time"] = info.FinishedTime
	}
	if strings.TrimSpace(info.JobName) != "" {
		existing["job_name"] = strings.TrimSpace(info.JobName)
	}
	return existing
}

func stringValue(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if str, ok := value.(string); ok {
				return str
			}
		}
	}
	return ""
}

func mapValue(m map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			if obj, ok := value.(map[string]interface{}); ok {
				return obj
			}
		}
	}
	return nil
}

func jsonMarshal(v interface{}) ([]byte, error) { return json.Marshal(v) }

func stringSliceValue(m map[string]interface{}, key string) []string {
	value, ok := m[key]
	if !ok {
		return nil
	}
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if str, ok := item.(string); ok && strings.TrimSpace(str) != "" {
			result = append(result, strings.TrimSpace(str))
		}
	}
	return result
}

func intValue(m map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := m[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case int:
			return v, true
		case float64:
			return int(v), true
		}
	}
	return 0, false
}

func normalizePreviewOutputFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "hocon":
		return "hocon"
	case "json", "":
		return "json"
	default:
		return "json"
	}
}

var platformVariablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

func detectTemplateVariables(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0)
	matches := platformVariablePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name != "" {
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				result = append(result, name)
			}
		}
	}
	sort.Strings(result)
	return result
}

func (s *Service) resolveTaskContent(ctx context.Context, task *Task) (string, error) {
	if task == nil {
		return "", ErrTaskDefinitionEmpty
	}
	content := task.Content
	if strings.TrimSpace(content) == "" {
		return "", nil
	}
	variables, err := s.resolveTaskVariables(ctx, task)
	if err != nil {
		return "", err
	}
	return replaceTemplateVariables(content, variables), nil
}

func (s *Service) resolveTaskVariables(ctx context.Context, task *Task) (map[string]string, error) {
	result := map[string]string{}
	items, err := s.repo.ListGlobalVariables(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.Key) != "" {
			result[strings.TrimSpace(item.Key)] = item.Value
		}
	}
	for key, value := range extractDefinitionVariables(task.Definition, "custom_variables") {
		result[key] = value
	}
	return result, nil
}

func extractDefinitionVariables(definition JSONMap, keys ...string) map[string]string {
	result := map[string]string{}
	for _, key := range keys {
		value := definition[key]
		mapped, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		for rawKey, rawValue := range mapped {
			name := strings.TrimSpace(rawKey)
			if name == "" {
				continue
			}
			result[name] = strings.TrimSpace(fmt.Sprint(rawValue))
		}
	}
	return result
}

func replaceTemplateVariables(content string, variables map[string]string) string {
	if strings.TrimSpace(content) == "" || len(variables) == 0 {
		return content
	}
	return platformVariablePattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := platformVariablePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		key := strings.TrimSpace(parts[1])
		if value, ok := variables[key]; ok {
			return value
		}
		return match
	})
}

func (s *Service) ensureRootFilesNested(ctx context.Context) error {
	tasks, err := s.repo.ListAllTasks(ctx)
	if err != nil {
		return err
	}
	rootFiles := make([]*Task, 0)
	var targetFolder *Task
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.ParentID == nil && task.NodeType == TaskNodeTypeFolder && targetFolder == nil {
			targetFolder = task
		}
		if task.ParentID == nil && task.NodeType == TaskNodeTypeFile {
			rootFiles = append(rootFiles, task)
		}
	}
	if len(rootFiles) == 0 {
		return nil
	}
	return s.repo.Transaction(ctx, func(tx *Repository) error {
		folder := targetFolder
		if folder == nil {
			folder = &Task{
				NodeType:      TaskNodeTypeFolder,
				Name:          "workspace",
				ContentFormat: ContentFormatHOCON,
				Status:        TaskStatusDraft,
			}
			if err := tx.CreateTask(ctx, folder); err != nil {
				return err
			}
		}
		for _, file := range rootFiles {
			file.ParentID = &folder.ID
			if err := tx.UpdateTask(ctx, file); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeGlobalVariableKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", ErrGlobalVariableKeyRequired
	}
	if !safeTaskNamePattern.MatchString(key) {
		return "", ErrGlobalVariableKeyInvalid
	}
	return key, nil
}

func defaultString(current string, fallback string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return fallback
}
