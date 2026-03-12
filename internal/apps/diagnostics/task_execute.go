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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/seatunnel/seatunnelX/internal/apps/cluster"
	"github.com/seatunnel/seatunnelX/internal/apps/monitor"
	monitoringapp "github.com/seatunnel/seatunnelX/internal/apps/monitoring"
	"github.com/seatunnel/seatunnelX/internal/config"
	"github.com/seatunnel/seatunnelX/internal/logger"
)

type diagnosticBundleArtifact struct {
	StepCode   DiagnosticStepCode `json:"step_code"`
	Category   string             `json:"category"`
	Format     string             `json:"format"`
	Status     string             `json:"status"`
	Path       string             `json:"path,omitempty"`
	RemotePath string             `json:"remote_path,omitempty"`
	NodeID     uint               `json:"node_id,omitempty"`
	HostID     uint               `json:"host_id,omitempty"`
	HostName   string             `json:"host_name,omitempty"`
	SizeBytes  int64              `json:"size_bytes,omitempty"`
	Message    string             `json:"message,omitempty"`
}

type diagnosticBundleManifest struct {
	Version       string                      `json:"version"`
	TaskID        uint                        `json:"task_id"`
	ClusterID     uint                        `json:"cluster_id"`
	TriggerSource DiagnosticTaskSourceType    `json:"trigger_source"`
	SourceRef     DiagnosticTaskSourceRef     `json:"source_ref"`
	Options       DiagnosticTaskOptions       `json:"options"`
	Status        DiagnosticTaskStatus        `json:"status"`
	Summary       string                      `json:"summary"`
	CreatedBy     uint                        `json:"created_by"`
	CreatedByName string                      `json:"created_by_name"`
	StartedAt     *time.Time                  `json:"started_at,omitempty"`
	CompletedAt   *time.Time                  `json:"completed_at,omitempty"`
	GeneratedAt   time.Time                   `json:"generated_at"`
	Artifacts     []*diagnosticBundleArtifact `json:"artifacts"`
}

type diagnosticBundleExecutionState struct {
	ErrorGroup       *SeatunnelErrorGroup
	ErrorEvents      []*SeatunnelErrorEvent
	InspectionDetail *ClusterInspectionReportDetailData
	ProcessEvents    []*monitor.ProcessEvent
	AlertSnapshot    []*monitoringapp.AlertInstance
	ClusterSnapshot  *cluster.Cluster
	Artifacts        []*diagnosticBundleArtifact
}

type diagnosticBundleHTMLPayload struct {
	GeneratedAt        time.Time                            `json:"generated_at"`
	Health             diagnosticBundleHTMLHealthSummary    `json:"health"`
	Task               diagnosticBundleHTMLTaskSummary      `json:"task"`
	Cluster            *diagnosticBundleHTMLClusterSummary  `json:"cluster,omitempty"`
	Inspection         *diagnosticBundleHTMLInspectionPanel `json:"inspection,omitempty"`
	ErrorContext       *diagnosticBundleHTMLErrorPanel      `json:"error_context,omitempty"`
	AlertSnapshot      *diagnosticBundleHTMLAlertPanel      `json:"alert_snapshot,omitempty"`
	ProcessEvents      *diagnosticBundleHTMLProcessPanel    `json:"process_events,omitempty"`
	TaskExecution      diagnosticBundleHTMLExecutionPanel   `json:"task_execution"`
	ArtifactGroups     []diagnosticBundleHTMLArtifactGroup  `json:"artifact_groups"`
	SourceTraceability []diagnosticBundleHTMLTraceItem      `json:"source_traceability"`
	Recommendations    []diagnosticBundleHTMLAdvice         `json:"recommendations"`
	PassedChecks       []diagnosticBundleHTMLAdvice         `json:"passed_checks"`
}

type diagnosticBundleHTMLHealthSummary struct {
	Tone    string                           `json:"tone"`
	Title   string                           `json:"title"`
	Summary string                           `json:"summary"`
	Metrics []diagnosticBundleHTMLMetricCard `json:"metrics"`
}

type diagnosticBundleHTMLTaskSummary struct {
	ID            uint                             `json:"id"`
	Status        DiagnosticTaskStatus             `json:"status"`
	TriggerSource DiagnosticTaskSourceType         `json:"trigger_source"`
	Summary       string                           `json:"summary"`
	CreatedBy     string                           `json:"created_by"`
	StartedAt     *time.Time                       `json:"started_at,omitempty"`
	CompletedAt   *time.Time                       `json:"completed_at,omitempty"`
	BundleDir     string                           `json:"bundle_dir"`
	ManifestPath  string                           `json:"manifest_path"`
	IndexPath     string                           `json:"index_path"`
	Options       DiagnosticTaskOptions            `json:"options"`
	SelectedNodes []diagnosticBundleHTMLNodeTarget `json:"selected_nodes"`
}

type diagnosticBundleHTMLClusterSummary struct {
	ID             uint                              `json:"id"`
	Name           string                            `json:"name"`
	Version        string                            `json:"version"`
	Status         string                            `json:"status"`
	DeploymentMode string                            `json:"deployment_mode"`
	InstallDir     string                            `json:"install_dir"`
	NodeCount      int                               `json:"node_count"`
	Nodes          []diagnosticBundleHTMLClusterNode `json:"nodes"`
}

type diagnosticBundleHTMLClusterNode struct {
	Role       string `json:"role"`
	HostID     uint   `json:"host_id"`
	InstallDir string `json:"install_dir"`
	Status     string `json:"status"`
	ProcessPID int    `json:"process_pid"`
}

type diagnosticBundleHTMLInspectionPanel struct {
	Summary         string                        `json:"summary"`
	Status          InspectionReportStatus        `json:"status"`
	RequestedBy     string                        `json:"requested_by"`
	LookbackMinutes int                           `json:"lookback_minutes"`
	CriticalCount   int                           `json:"critical_count"`
	WarningCount    int                           `json:"warning_count"`
	InfoCount       int                           `json:"info_count"`
	StartedAt       *time.Time                    `json:"started_at,omitempty"`
	FinishedAt      *time.Time                    `json:"finished_at,omitempty"`
	Findings        []diagnosticBundleHTMLFinding `json:"findings"`
}

type diagnosticBundleHTMLFinding struct {
	Severity       string `json:"severity"`
	CheckName      string `json:"check_name"`
	CheckCode      string `json:"check_code"`
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
	Evidence       string `json:"evidence"`
}

type diagnosticBundleHTMLErrorPanel struct {
	GroupTitle       string                           `json:"group_title"`
	ExceptionClass   string                           `json:"exception_class"`
	OccurrenceCount  int64                            `json:"occurrence_count"`
	FirstSeenAt      *time.Time                       `json:"first_seen_at,omitempty"`
	LastSeenAt       *time.Time                       `json:"last_seen_at,omitempty"`
	SampleMessage    string                           `json:"sample_message"`
	RecentEventCount int                              `json:"recent_event_count"`
	Events           []diagnosticBundleHTMLErrorEvent `json:"events"`
}

type diagnosticBundleHTMLErrorEvent struct {
	OccurredAt string `json:"occurred_at"`
	Role       string `json:"role"`
	HostLabel  string `json:"host_label"`
	SourceFile string `json:"source_file"`
	JobID      string `json:"job_id"`
	Message    string `json:"message"`
	Evidence   string `json:"evidence"`
}

type diagnosticBundleHTMLAlertPanel struct {
	Total    int                             `json:"total"`
	Critical int                             `json:"critical"`
	Warning  int                             `json:"warning"`
	Firing   int                             `json:"firing"`
	Alerts   []diagnosticBundleHTMLAlertItem `json:"alerts"`
}

type diagnosticBundleHTMLAlertItem struct {
	Name        string `json:"name"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
}

type diagnosticBundleHTMLProcessPanel struct {
	Total  int                                `json:"total"`
	ByType []diagnosticBundleHTMLMetricCard   `json:"by_type"`
	Events []diagnosticBundleHTMLProcessEvent `json:"events"`
}

type diagnosticBundleHTMLProcessEvent struct {
	CreatedAt   string `json:"created_at"`
	EventType   string `json:"event_type"`
	ProcessName string `json:"process_name"`
	NodeLabel   string `json:"node_label"`
	Details     string `json:"details"`
}

type diagnosticBundleHTMLExecutionPanel struct {
	Steps []diagnosticBundleHTMLExecutionStep `json:"steps"`
	Nodes []diagnosticBundleHTMLExecutionNode `json:"nodes"`
}

type diagnosticBundleHTMLExecutionStep struct {
	Sequence    int    `json:"sequence"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Error       string `json:"error"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}

type diagnosticBundleHTMLExecutionNode struct {
	HostLabel   string `json:"host_label"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CurrentStep string `json:"current_step"`
	Message     string `json:"message"`
	Error       string `json:"error"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
}

type diagnosticBundleHTMLArtifactGroup struct {
	Key   string                             `json:"key"`
	Label string                             `json:"label"`
	Items []diagnosticBundleHTMLArtifactView `json:"items"`
}

type diagnosticBundleHTMLArtifactView struct {
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"`
	StepCode      string `json:"step_code"`
	Status        string `json:"status"`
	Format        string `json:"format"`
	HostLabel     string `json:"host_label"`
	LocalPath     string `json:"local_path"`
	RelativePath  string `json:"relative_path"`
	RemotePath    string `json:"remote_path"`
	SizeLabel     string `json:"size_label"`
	Message       string `json:"message"`
	Preview       string `json:"preview"`
	PreviewNote   string `json:"preview_note"`
}

type diagnosticBundleHTMLNodeTarget struct {
	HostLabel   string `json:"host_label"`
	Role        string `json:"role"`
	InstallDir  string `json:"install_dir"`
	ClusterNode string `json:"cluster_node"`
}

type diagnosticBundleHTMLTraceItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type diagnosticBundleHTMLMetricCard struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Note  string `json:"note"`
}

type diagnosticBundleHTMLAdvice struct {
	Title   string `json:"title"`
	Details string `json:"details"`
}

type agentThreadDumpResult struct {
	Status      string    `json:"status"`
	PID         int       `json:"pid"`
	Role        string    `json:"role"`
	InstallDir  string    `json:"install_dir"`
	Tool        string    `json:"tool"`
	OutputPath  string    `json:"output_path"`
	SizeBytes   int64     `json:"size_bytes"`
	Message     string    `json:"message"`
	Content     string    `json:"content"`
	CollectedAt time.Time `json:"collected_at"`
}

type agentJVMDumpResult struct {
	Status         string    `json:"status"`
	PID            int       `json:"pid"`
	Role           string    `json:"role"`
	InstallDir     string    `json:"install_dir"`
	Tool           string    `json:"tool"`
	OutputPath     string    `json:"output_path"`
	SizeBytes      int64     `json:"size_bytes"`
	FreeBytes      int64     `json:"free_bytes"`
	RequiredBytes  int64     `json:"required_bytes"`
	EstimatedBytes int64     `json:"estimated_bytes"`
	Message        string    `json:"message"`
	CollectedAt    time.Time `json:"collected_at"`
}

func (s *Service) StartDiagnosticTask(ctx context.Context, taskID uint) error {
	if s == nil || s.repo == nil {
		return ErrDiagnosticsRepositoryUnavailable
	}
	task, err := s.repo.GetDiagnosticTaskByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status == DiagnosticTaskStatusRunning {
		return nil
	}
	if task.Status == DiagnosticTaskStatusSucceeded {
		return nil
	}
	now := time.Now().UTC()
	if task.StartedAt == nil {
		task.StartedAt = &now
	}
	task.Status = DiagnosticTaskStatusRunning
	task.UpdatedAt = now
	if task.CurrentStep == "" {
		task.CurrentStep = resolveInitialDiagnosticTaskStep(DefaultDiagnosticTaskSteps(), task.Options.Normalize())
	}
	if err := s.UpdateDiagnosticTask(ctx, task); err != nil {
		return err
	}
	go s.executeDiagnosticTask(context.Background(), task.ID)
	return nil
}

func (s *Service) executeDiagnosticTask(ctx context.Context, taskID uint) {
	task, err := s.repo.GetDiagnosticTaskByID(ctx, taskID)
	if err != nil {
		logger.ErrorF(ctx, "[DiagnosticsTask] load task failed: task_id=%d err=%v", taskID, err)
		return
	}
	if err := s.runDiagnosticTask(ctx, task); err != nil {
		logger.ErrorF(ctx, "[DiagnosticsTask] run task failed: task_id=%d err=%v", taskID, err)
	}
}

func (s *Service) runDiagnosticTask(ctx context.Context, task *DiagnosticTask) error {
	if task == nil {
		return ErrDiagnosticTaskNotFound
	}
	stepsByCode := make(map[DiagnosticStepCode]*DiagnosticTaskStep, len(task.Steps))
	for _, step := range task.Steps {
		stepsByCode[step.Code] = &DiagnosticTaskStep{
			ID:          step.ID,
			TaskID:      step.TaskID,
			Code:        step.Code,
			Sequence:    step.Sequence,
			Title:       step.Title,
			Description: step.Description,
			Status:      step.Status,
			Message:     step.Message,
			Error:       step.Error,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
			CreatedAt:   step.CreatedAt,
			UpdatedAt:   step.UpdatedAt,
		}
	}
	nodesByClusterNodeID := make(map[uint]*DiagnosticNodeExecution, len(task.NodeExecutions))
	for _, node := range task.NodeExecutions {
		copyNode := node
		nodesByClusterNodeID[node.ClusterNodeID] = &copyNode
	}

	state := &diagnosticBundleExecutionState{}
	bundleDir := diagnosticTaskBundleDir(task.ID)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return s.failDiagnosticTask(ctx, task, DiagnosticStepCodeCollectConfigSnapshot, fmt.Errorf("create bundle dir: %w", err))
	}
	task.BundleDir = bundleDir
	if err := s.UpdateDiagnosticTask(ctx, task); err != nil {
		return err
	}

	for _, planStep := range DefaultDiagnosticTaskSteps() {
		step := stepsByCode[planStep.Code]
		if step == nil {
			continue
		}
		if step.Status == DiagnosticTaskStatusSkipped {
			continue
		}

		if err := s.beginDiagnosticTaskStep(ctx, task, step); err != nil {
			return err
		}

		stepErr := s.executeDiagnosticPlanStep(ctx, task, step, planStep, nodesByClusterNodeID, state, bundleDir)
		if stepErr != nil {
			if err := s.failDiagnosticTaskStep(ctx, task, step, stepErr); err != nil {
				return err
			}
			if planStep.Required {
				return s.failDiagnosticTask(ctx, task, step.Code, stepErr)
			}
			_ = s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
				TaskID:         task.ID,
				TaskStepID:     uintPtr(step.ID),
				StepCode:       step.Code,
				Level:          DiagnosticLogLevelWarn,
				EventType:      DiagnosticLogEventTypeNote,
				Message:        bilingualText("可选步骤失败，任务继续执行。", "Optional step failed and task will continue."),
				CreatedAt:      time.Now().UTC(),
				CommandSummary: stepErr.Error(),
			})
			continue
		}

		if err := s.finishDiagnosticTaskStep(ctx, task, step, bilingualText("步骤执行完成。", "Step completed.")); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	task.Status = DiagnosticTaskStatusSucceeded
	task.CurrentStep = DiagnosticStepCodeComplete
	task.CompletedAt = &now
	task.UpdatedAt = now
	task.Summary = strings.TrimSpace(task.Summary)
	if task.Summary == "" {
		task.Summary = bilingualText("诊断任务执行完成。", "Diagnostic bundle task completed.")
	}
	if err := s.UpdateDiagnosticTask(ctx, task); err != nil {
		return err
	}
	if task.ManifestPath != "" {
		if err := writeDiagnosticBundleManifestFile(task.ManifestPath, task, state.Artifacts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) executeDiagnosticPlanStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, planStep DiagnosticPlanStep, nodesByClusterNodeID map[uint]*DiagnosticNodeExecution, state *diagnosticBundleExecutionState, bundleDir string) error {
	switch step.Code {
	case DiagnosticStepCodeCollectErrorContext:
		return s.executeCollectErrorContextStep(ctx, task, step, state, bundleDir)
	case DiagnosticStepCodeCollectProcessEvents:
		return s.executeCollectProcessEventsStep(ctx, task, step, state, bundleDir)
	case DiagnosticStepCodeCollectAlertSnapshot:
		return s.executeCollectAlertSnapshotStep(ctx, task, step, state, bundleDir)
	case DiagnosticStepCodeCollectConfigSnapshot:
		return s.executeCollectConfigSnapshotStep(ctx, task, step, state, bundleDir)
	case DiagnosticStepCodeCollectLogSample:
		return s.executeCollectLogSampleStep(ctx, task, step, nodesByClusterNodeID, state, bundleDir)
	case DiagnosticStepCodeCollectThreadDump:
		return s.executeCollectThreadDumpStep(ctx, task, step, nodesByClusterNodeID, state, bundleDir)
	case DiagnosticStepCodeCollectJVMDump:
		return s.executeCollectJVMDumpStep(ctx, task, step, nodesByClusterNodeID, state, bundleDir)
	case DiagnosticStepCodeAssembleManifest:
		return s.executeAssembleManifestStep(ctx, task, step, state, bundleDir)
	case DiagnosticStepCodeRenderHTMLSummary:
		return s.executeRenderHTMLSummaryStep(ctx, task, step, state, bundleDir)
	case DiagnosticStepCodeComplete:
		return s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
			TaskID:     task.ID,
			TaskStepID: uintPtr(step.ID),
			StepCode:   step.Code,
			Level:      DiagnosticLogLevelInfo,
			EventType:  DiagnosticLogEventTypeSuccess,
			Message:    bilingualText("诊断任务执行完成。", "Diagnostic task completed."),
			CreatedAt:  time.Now().UTC(),
		})
	default:
		return nil
	}
}

func (s *Service) executeCollectErrorContextStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir string) error {
	payload := map[string]interface{}{
		"trigger_source": task.TriggerSource,
		"source_ref":     task.SourceRef,
	}
	if task.SourceRef.ErrorGroupID > 0 {
		group, err := s.repo.GetErrorGroupByID(ctx, task.SourceRef.ErrorGroupID)
		if err != nil {
			return err
		}
		state.ErrorGroup = group
		events, _, err := s.repo.ListErrorEvents(ctx, &SeatunnelErrorEventFilter{
			ErrorGroupID: group.ID,
			Page:         1,
			PageSize:     100,
		})
		if err != nil {
			return err
		}
		state.ErrorEvents = events
		payload["error_group"] = group
		payload["error_events"] = events
	}
	if task.SourceRef.InspectionReportID > 0 {
		detail, err := s.GetInspectionReportDetail(ctx, task.SourceRef.InspectionReportID)
		if err != nil {
			return err
		}
		state.InspectionDetail = detail
		payload["inspection_detail"] = detail
	}
	return s.writeDiagnosticJSONArtifact(ctx, task, step, state, bundleDir, "error-context.json", payload, &diagnosticBundleArtifact{
		StepCode: step.Code,
		Category: "error_context",
		Format:   "json",
		Status:   "created",
	})
}

func (s *Service) executeCollectProcessEventsStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir string) error {
	if s.monitorService == nil {
		state.ProcessEvents = []*monitor.ProcessEvent{}
	} else {
		events, err := s.monitorService.ListClusterEvents(ctx, task.ClusterID, 200)
		if err != nil {
			return err
		}
		state.ProcessEvents = events
	}
	return s.writeDiagnosticJSONArtifact(ctx, task, step, state, bundleDir, "process-events.json", state.ProcessEvents, &diagnosticBundleArtifact{
		StepCode: step.Code,
		Category: "process_events",
		Format:   "json",
		Status:   "created",
	})
}

func (s *Service) executeCollectAlertSnapshotStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir string) error {
	alerts := make([]*monitoringapp.AlertInstance, 0)
	if s.monitoringService != nil {
		data, err := s.monitoringService.ListAlertInstances(ctx, &monitoringapp.AlertInstanceFilter{
			ClusterID: fmt.Sprintf("%d", task.ClusterID),
			Page:      1,
			PageSize:  200,
		})
		if err != nil {
			return err
		}
		if data != nil {
			alerts = data.Alerts
		}
	}
	if task.SourceRef.AlertID != "" {
		filtered := make([]*monitoringapp.AlertInstance, 0, len(alerts))
		for _, alert := range alerts {
			if alert != nil && strings.TrimSpace(alert.AlertID) == strings.TrimSpace(task.SourceRef.AlertID) {
				filtered = append(filtered, alert)
			}
		}
		alerts = filtered
	}
	state.AlertSnapshot = alerts
	return s.writeDiagnosticJSONArtifact(ctx, task, step, state, bundleDir, "alert-snapshot.json", alerts, &diagnosticBundleArtifact{
		StepCode: step.Code,
		Category: "alert_snapshot",
		Format:   "json",
		Status:   "created",
	})
}

func (s *Service) executeCollectConfigSnapshotStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir string) error {
	if s.clusterService == nil {
		return fmt.Errorf("cluster service is unavailable")
	}
	clusterInfo, err := s.clusterService.Get(ctx, task.ClusterID)
	if err != nil {
		return err
	}
	state.ClusterSnapshot = clusterInfo
	return s.writeDiagnosticJSONArtifact(ctx, task, step, state, bundleDir, "config-snapshot.json", clusterInfo, &diagnosticBundleArtifact{
		StepCode: step.Code,
		Category: "config_snapshot",
		Format:   "json",
		Status:   "created",
	})
}

func (s *Service) executeCollectLogSampleStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, nodesByClusterNodeID map[uint]*DiagnosticNodeExecution, state *diagnosticBundleExecutionState, bundleDir string) error {
	if s.agentSender == nil {
		return fmt.Errorf("agent sender is unavailable")
	}
	logDir := filepath.Join(bundleDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	var successCount int
	var errs []string
	for _, selected := range sortedDiagnosticTaskTargets(task.SelectedNodes) {
		node := nodesByClusterNodeID[selected.ClusterNodeID]
		if node == nil {
			continue
		}
		if err := s.beginDiagnosticNodeStep(ctx, step, node, bilingualText("正在采集日志样本。", "Collecting log samples.")); err != nil {
			return err
		}
		candidates := buildDiagnosticLogCandidates(selected, state.ErrorEvents)
		var nodeSuccess bool
		for _, candidate := range candidates {
			success, output, err := s.agentSender.SendCommand(ctx, selected.AgentID, "get_logs", map[string]string{
				"log_file": candidate,
				"mode":     "tail",
				"lines":    fmt.Sprintf("%d", task.Options.LogSampleLines),
			})
			if err != nil || !success {
				detail := resolveDiagnosticCommandFailure(output, err, "日志样本采集失败。", "Failed to collect log sample.")
				_ = s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
					TaskID:          task.ID,
					TaskStepID:      uintPtr(step.ID),
					NodeExecutionID: uintPtr(node.ID),
					StepCode:        step.Code,
					Level:           DiagnosticLogLevelWarn,
					EventType:       DiagnosticLogEventTypeNote,
					Message:         bilingualText(fmt.Sprintf("从 %s 采集日志样本失败：%s", candidate, detail), fmt.Sprintf("Failed to collect log sample from %s: %s", candidate, detail)),
					CommandSummary:  candidate,
					CreatedAt:       time.Now().UTC(),
				})
				continue
			}
			fileName := fmt.Sprintf("host-%d-%s.log", selected.HostID, filepath.Base(candidate))
			localPath := filepath.Join(logDir, fileName)
			if err := os.WriteFile(localPath, []byte(output), 0o644); err != nil {
				return err
			}
			state.Artifacts = append(state.Artifacts, &diagnosticBundleArtifact{
				StepCode:  step.Code,
				Category:  "log_sample",
				Format:    "log",
				Status:    "created",
				Path:      localPath,
				NodeID:    selected.NodeID,
				HostID:    selected.HostID,
				HostName:  selected.HostName,
				SizeBytes: int64(len(output)),
				Message:   candidate,
			})
			_ = s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
				TaskID:          task.ID,
				TaskStepID:      uintPtr(step.ID),
				NodeExecutionID: uintPtr(node.ID),
				StepCode:        step.Code,
				Level:           DiagnosticLogLevelInfo,
				EventType:       DiagnosticLogEventTypeSuccess,
				Message:         bilingualText(fmt.Sprintf("已从 %s 采集日志样本", candidate), fmt.Sprintf("Collected log sample from %s", candidate)),
				CommandSummary:  candidate,
				CreatedAt:       time.Now().UTC(),
			})
			nodeSuccess = true
			successCount++
			break
		}
		if nodeSuccess {
			if err := s.finishDiagnosticNodeStep(ctx, step, node, DiagnosticTaskStatusSucceeded, bilingualText("日志样本采集完成。", "Log sample collected.")); err != nil {
				return err
			}
			continue
		}
		errs = append(errs, fmt.Sprintf("host=%d", selected.HostID))
		if err := s.finishDiagnosticNodeStep(ctx, step, node, DiagnosticTaskStatusFailed, bilingualText("未采集到日志样本。", "No log sample collected.")); err != nil {
			return err
		}
	}
	if successCount == 0 {
		return formatDiagnosticAllNodesFailed("全部节点都未采集到日志样本", "No log samples collected on any node", errs)
	}
	return nil
}

func (s *Service) executeCollectThreadDumpStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, nodesByClusterNodeID map[uint]*DiagnosticNodeExecution, state *diagnosticBundleExecutionState, bundleDir string) error {
	if s.agentSender == nil {
		return fmt.Errorf("agent sender is unavailable")
	}
	outputDir := filepath.Join(bundleDir, "thread-dumps")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	var successCount int
	var errs []string
	for _, selected := range sortedDiagnosticTaskTargets(task.SelectedNodes) {
		node := nodesByClusterNodeID[selected.ClusterNodeID]
		if node == nil {
			continue
		}
		if err := s.beginDiagnosticNodeStep(ctx, step, node, bilingualText("正在采集线程栈。", "Collecting thread dump.")); err != nil {
			return err
		}
		success, output, err := s.agentSender.SendCommand(ctx, selected.AgentID, "thread_dump", map[string]string{
			"install_dir": selected.InstallDir,
			"role":        selected.Role,
		})
		if err != nil || !success {
			detail := resolveDiagnosticCommandFailure(output, err, "线程栈采集失败。", "Thread dump failed.")
			errs = append(errs, fmt.Sprintf("host=%d: %s", selected.HostID, detail))
			_ = s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
				TaskID:          task.ID,
				TaskStepID:      uintPtr(step.ID),
				NodeExecutionID: uintPtr(node.ID),
				StepCode:        step.Code,
				Level:           DiagnosticLogLevelError,
				EventType:       DiagnosticLogEventTypeFailed,
				Message:         detail,
				CommandSummary:  selected.Role,
				CreatedAt:       time.Now().UTC(),
			})
			if finishErr := s.finishDiagnosticNodeStep(ctx, step, node, DiagnosticTaskStatusFailed, detail); finishErr != nil {
				return finishErr
			}
			continue
		}
		var result agentThreadDumpResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			return err
		}
		localPath := filepath.Join(outputDir, fmt.Sprintf("thread-dump-host-%d-%s.txt", selected.HostID, normalizeDiagnosticFileRole(selected.Role)))
		if err := os.WriteFile(localPath, []byte(result.Content), 0o644); err != nil {
			return err
		}
		state.Artifacts = append(state.Artifacts, &diagnosticBundleArtifact{
			StepCode:   step.Code,
			Category:   "thread_dump",
			Format:     "txt",
			Status:     result.Status,
			Path:       localPath,
			RemotePath: result.OutputPath,
			NodeID:     selected.NodeID,
			HostID:     selected.HostID,
			HostName:   selected.HostName,
			SizeBytes:  result.SizeBytes,
			Message:    result.Tool,
		})
		if err := s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
			TaskID:          task.ID,
			TaskStepID:      uintPtr(step.ID),
			NodeExecutionID: uintPtr(node.ID),
			StepCode:        step.Code,
			Level:           DiagnosticLogLevelInfo,
			EventType:       DiagnosticLogEventTypeSuccess,
			Message:         bilingualText(fmt.Sprintf("线程栈已保存到 %s", localPath), fmt.Sprintf("Thread dump collected to %s", localPath)),
			CommandSummary:  result.Tool,
			CreatedAt:       time.Now().UTC(),
			Metadata: DiagnosticLogMetadata{
				"remote_path": result.OutputPath,
			},
		}); err != nil {
			return err
		}
		if err := s.finishDiagnosticNodeStep(ctx, step, node, DiagnosticTaskStatusSucceeded, bilingualText("线程栈采集完成。", "Thread dump collected.")); err != nil {
			return err
		}
		successCount++
	}
	if successCount == 0 {
		return formatDiagnosticAllNodesFailed("全部节点线程栈采集失败", "Thread dump failed on all nodes", errs)
	}
	return nil
}

func (s *Service) executeCollectJVMDumpStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, nodesByClusterNodeID map[uint]*DiagnosticNodeExecution, state *diagnosticBundleExecutionState, bundleDir string) error {
	if s.agentSender == nil {
		return fmt.Errorf("agent sender is unavailable")
	}
	outputDir := filepath.Join(bundleDir, "jvm-dumps")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	var successCount int
	var skippedCount int
	var errs []string
	for _, selected := range sortedDiagnosticTaskTargets(task.SelectedNodes) {
		node := nodesByClusterNodeID[selected.ClusterNodeID]
		if node == nil {
			continue
		}
		if err := s.beginDiagnosticNodeStep(ctx, step, node, bilingualText("正在采集 JVM Dump。", "Collecting JVM dump.")); err != nil {
			return err
		}
		success, output, err := s.agentSender.SendCommand(ctx, selected.AgentID, "jvm_dump", map[string]string{
			"install_dir": selected.InstallDir,
			"role":        selected.Role,
			"min_free_mb": fmt.Sprintf("%d", task.Options.JVMDumpMinFreeMB),
		})
		if err != nil || !success {
			detail := resolveDiagnosticCommandFailure(output, err, "JVM Dump 采集失败。", "JVM dump failed.")
			errs = append(errs, fmt.Sprintf("host=%d: %s", selected.HostID, detail))
			_ = s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
				TaskID:          task.ID,
				TaskStepID:      uintPtr(step.ID),
				NodeExecutionID: uintPtr(node.ID),
				StepCode:        step.Code,
				Level:           DiagnosticLogLevelError,
				EventType:       DiagnosticLogEventTypeFailed,
				Message:         detail,
				CommandSummary:  selected.Role,
				CreatedAt:       time.Now().UTC(),
			})
			if finishErr := s.finishDiagnosticNodeStep(ctx, step, node, DiagnosticTaskStatusFailed, detail); finishErr != nil {
				return finishErr
			}
			continue
		}
		var result agentJVMDumpResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			return err
		}
		localPath := filepath.Join(outputDir, fmt.Sprintf("jvm-dump-host-%d-%s.json", selected.HostID, normalizeDiagnosticFileRole(selected.Role)))
		if err := os.WriteFile(localPath, []byte(output), 0o644); err != nil {
			return err
		}
		status := DiagnosticTaskStatusSucceeded
		if result.Status == "skipped" {
			status = DiagnosticTaskStatusSkipped
			skippedCount++
		} else {
			successCount++
		}
		state.Artifacts = append(state.Artifacts, &diagnosticBundleArtifact{
			StepCode:   step.Code,
			Category:   "jvm_dump",
			Format:     "json",
			Status:     result.Status,
			Path:       localPath,
			RemotePath: result.OutputPath,
			NodeID:     selected.NodeID,
			HostID:     selected.HostID,
			HostName:   selected.HostName,
			SizeBytes:  result.SizeBytes,
			Message:    result.Message,
		})
		if err := s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
			TaskID:          task.ID,
			TaskStepID:      uintPtr(step.ID),
			NodeExecutionID: uintPtr(node.ID),
			StepCode:        step.Code,
			Level:           DiagnosticLogLevelInfo,
			EventType:       DiagnosticLogEventTypeNote,
			Message:         result.Message,
			CommandSummary:  result.Tool,
			CreatedAt:       time.Now().UTC(),
			Metadata: DiagnosticLogMetadata{
				"remote_path":    result.OutputPath,
				"free_bytes":     result.FreeBytes,
				"required_bytes": result.RequiredBytes,
			},
		}); err != nil {
			return err
		}
		if err := s.finishDiagnosticNodeStep(ctx, step, node, status, result.Message); err != nil {
			return err
		}
	}
	if successCount == 0 && skippedCount == 0 {
		return formatDiagnosticAllNodesFailed("全部节点 JVM Dump 采集失败", "JVM dump failed on all nodes", errs)
	}
	return nil
}

func (s *Service) executeAssembleManifestStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir string) error {
	manifestPath := filepath.Join(bundleDir, "manifest.json")
	manifestArtifact := &diagnosticBundleArtifact{
		StepCode: step.Code,
		Category: "manifest",
		Format:   "json",
		Status:   "created",
		Path:     manifestPath,
		Message:  bilingualText("诊断包 Manifest", "Diagnostic bundle manifest"),
	}
	if err := writeDiagnosticBundleManifestFile(manifestPath, task, append(cloneDiagnosticArtifacts(state.Artifacts), manifestArtifact)); err != nil {
		return err
	}
	fileInfo, err := os.Stat(manifestPath)
	if err != nil {
		return err
	}
	manifestArtifact.SizeBytes = fileInfo.Size()
	task.ManifestPath = manifestPath
	if err := s.UpdateDiagnosticTask(ctx, task); err != nil {
		return err
	}
	state.Artifacts = append(state.Artifacts, manifestArtifact)
	return nil
}

func (s *Service) executeRenderHTMLSummaryStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir string) error {
	indexPath := filepath.Join(bundleDir, "index.html")
	htmlArtifact := &diagnosticBundleArtifact{
		StepCode: step.Code,
		Category: "diagnostic_report",
		Format:   "html",
		Status:   "created",
		Path:     indexPath,
		Message:  bilingualText("离线诊断报告", "Offline diagnostic report"),
	}
	payload := buildDiagnosticBundleHTMLPayload(task, state, bundleDir, append(cloneDiagnosticArtifacts(state.Artifacts), htmlArtifact))
	tmpl, err := template.New("diagnostic-summary").Funcs(template.FuncMap{
		"formatTime": func(value interface{}) string {
			switch typed := value.(type) {
			case *time.Time:
				return formatDiagnosticBundleTime(typed)
			case time.Time:
				return formatDiagnosticBundleTimeValue(typed)
			default:
				return "-"
			}
		},
		"statusClass": func(status interface{}) string {
			return diagnosticHTMLStatusClass(fmt.Sprint(status))
		},
		"toneClass": func(tone interface{}) string {
			return diagnosticHTMLToneClass(fmt.Sprint(tone))
		},
	}).Parse(diagnosticBundleHTMLTemplate)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, payload); err != nil {
		return err
	}
	if err := os.WriteFile(indexPath, buffer.Bytes(), 0o644); err != nil {
		return err
	}
	task.IndexPath = indexPath
	if err := s.UpdateDiagnosticTask(ctx, task); err != nil {
		return err
	}
	htmlArtifact.SizeBytes = int64(buffer.Len())
	state.Artifacts = append(state.Artifacts, htmlArtifact)
	if task.ManifestPath != "" {
		if err := writeDiagnosticBundleManifestFile(task.ManifestPath, task, state.Artifacts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) beginDiagnosticTaskStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep) error {
	now := time.Now().UTC()
	task.Status = DiagnosticTaskStatusRunning
	task.CurrentStep = step.Code
	task.UpdatedAt = now
	if task.StartedAt == nil {
		task.StartedAt = &now
	}
	if err := s.UpdateDiagnosticTask(ctx, task); err != nil {
		return err
	}
	step.Status = DiagnosticTaskStatusRunning
	step.StartedAt = &now
	step.CompletedAt = nil
	step.Message = step.Description
	return s.UpdateDiagnosticTaskStep(ctx, step)
}

func (s *Service) finishDiagnosticTaskStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, message string) error {
	now := time.Now().UTC()
	step.Status = DiagnosticTaskStatusSucceeded
	step.Message = message
	step.Error = ""
	step.CompletedAt = &now
	return s.UpdateDiagnosticTaskStep(ctx, step)
}

func (s *Service) failDiagnosticTaskStep(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, stepErr error) error {
	now := time.Now().UTC()
	step.Status = DiagnosticTaskStatusFailed
	step.Error = stepErr.Error()
	step.Message = stepErr.Error()
	step.CompletedAt = &now
	return s.UpdateDiagnosticTaskStep(ctx, step)
}

func (s *Service) failDiagnosticTask(ctx context.Context, task *DiagnosticTask, failureStep DiagnosticStepCode, taskErr error) error {
	now := time.Now().UTC()
	task.Status = DiagnosticTaskStatusFailed
	task.FailureStep = failureStep
	task.FailureReason = taskErr.Error()
	task.CompletedAt = &now
	task.UpdatedAt = now
	return s.UpdateDiagnosticTask(ctx, task)
}

func (s *Service) beginDiagnosticNodeStep(ctx context.Context, step *DiagnosticTaskStep, node *DiagnosticNodeExecution, message string) error {
	now := time.Now().UTC()
	node.TaskStepID = uintPtr(step.ID)
	node.CurrentStep = step.Code
	node.Status = DiagnosticTaskStatusRunning
	node.Message = message
	node.Error = ""
	node.StartedAt = &now
	return s.UpdateDiagnosticNodeExecution(ctx, node)
}

func (s *Service) finishDiagnosticNodeStep(ctx context.Context, step *DiagnosticTaskStep, node *DiagnosticNodeExecution, status DiagnosticTaskStatus, message string) error {
	now := time.Now().UTC()
	node.TaskStepID = uintPtr(step.ID)
	node.CurrentStep = step.Code
	node.Status = status
	node.Message = message
	if status != DiagnosticTaskStatusFailed {
		node.Error = ""
	}
	node.CompletedAt = &now
	return s.UpdateDiagnosticNodeExecution(ctx, node)
}

func (s *Service) writeDiagnosticJSONArtifact(ctx context.Context, task *DiagnosticTask, step *DiagnosticTaskStep, state *diagnosticBundleExecutionState, bundleDir, fileName string, payload interface{}, artifact *diagnosticBundleArtifact) error {
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(bundleDir, fileName)
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return err
	}
	artifact.Path = path
	artifact.SizeBytes = int64(len(bytes))
	state.Artifacts = append(state.Artifacts, artifact)
	return s.AppendDiagnosticStepLog(ctx, &DiagnosticStepLog{
		TaskID:         task.ID,
		TaskStepID:     uintPtr(step.ID),
		StepCode:       step.Code,
		Level:          DiagnosticLogLevelInfo,
		EventType:      DiagnosticLogEventTypeSuccess,
		Message:        bilingualText(fmt.Sprintf("已生成 %s", fileName), fmt.Sprintf("Created %s", fileName)),
		CommandSummary: path,
		CreatedAt:      time.Now().UTC(),
	})
}

func writeDiagnosticBundleManifestFile(path string, task *DiagnosticTask, artifacts []*diagnosticBundleArtifact) error {
	manifest := buildDiagnosticBundleManifest(task, artifacts)
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func buildDiagnosticBundleManifest(task *DiagnosticTask, artifacts []*diagnosticBundleArtifact) *diagnosticBundleManifest {
	if task == nil {
		return nil
	}
	return &diagnosticBundleManifest{
		Version:       "v1",
		TaskID:        task.ID,
		ClusterID:     task.ClusterID,
		TriggerSource: task.TriggerSource,
		SourceRef:     task.SourceRef,
		Options:       task.Options.Normalize(),
		Status:        task.Status,
		Summary:       task.Summary,
		CreatedBy:     task.CreatedBy,
		CreatedByName: task.CreatedByName,
		StartedAt:     task.StartedAt,
		CompletedAt:   task.CompletedAt,
		GeneratedAt:   time.Now().UTC(),
		Artifacts:     cloneDiagnosticArtifacts(artifacts),
	}
}

func cloneDiagnosticArtifacts(src []*diagnosticBundleArtifact) []*diagnosticBundleArtifact {
	if len(src) == 0 {
		return []*diagnosticBundleArtifact{}
	}
	result := make([]*diagnosticBundleArtifact, 0, len(src))
	for _, item := range src {
		if item == nil {
			continue
		}
		copyItem := *item
		result = append(result, &copyItem)
	}
	return result
}

func buildDiagnosticBundleHTMLPayload(task *DiagnosticTask, state *diagnosticBundleExecutionState, bundleDir string, artifacts []*diagnosticBundleArtifact) *diagnosticBundleHTMLPayload {
	payload := &diagnosticBundleHTMLPayload{
		GeneratedAt:        time.Now().UTC(),
		Task:               buildDiagnosticBundleHTMLTaskSummary(task),
		SourceTraceability: buildDiagnosticBundleHTMLTraceItems(task),
		TaskExecution:      buildDiagnosticBundleHTMLExecutionPanel(task),
		ArtifactGroups:     buildDiagnosticBundleHTMLArtifactGroups(bundleDir, artifacts),
	}
	payload.Health = buildDiagnosticBundleHTMLHealthSummary(task, state, artifacts)
	if state == nil {
		payload.Recommendations = buildDiagnosticBundleHTMLRecommendations(task, state)
		payload.PassedChecks = buildDiagnosticBundleHTMLPassedChecks(task, state, artifacts)
		return payload
	}
	payload.Cluster = buildDiagnosticBundleHTMLClusterSummary(state.ClusterSnapshot)
	payload.Inspection = buildDiagnosticBundleHTMLInspectionPanel(state.InspectionDetail)
	payload.ErrorContext = buildDiagnosticBundleHTMLErrorPanel(state.ErrorGroup, state.ErrorEvents)
	payload.AlertSnapshot = buildDiagnosticBundleHTMLAlertPanel(state.AlertSnapshot)
	payload.ProcessEvents = buildDiagnosticBundleHTMLProcessPanel(state.ProcessEvents)
	payload.Recommendations = buildDiagnosticBundleHTMLRecommendations(task, state)
	payload.PassedChecks = buildDiagnosticBundleHTMLPassedChecks(task, state, artifacts)
	return payload
}

func buildDiagnosticBundleHTMLTaskSummary(task *DiagnosticTask) diagnosticBundleHTMLTaskSummary {
	summary := diagnosticBundleHTMLTaskSummary{}
	if task == nil {
		return summary
	}
	selectedNodes := make([]diagnosticBundleHTMLNodeTarget, 0, len(task.SelectedNodes))
	for _, node := range sortedDiagnosticTaskTargets(task.SelectedNodes) {
		selectedNodes = append(selectedNodes, diagnosticBundleHTMLNodeTarget{
			HostLabel:   resolveDiagnosticHostLabel(node.HostName, node.HostID, node.HostIP),
			Role:        normalizeDiagnosticDisplayText(node.Role),
			InstallDir:  normalizeDiagnosticDisplayText(node.InstallDir),
			ClusterNode: fmt.Sprintf("#%d", node.ClusterNodeID),
		})
	}
	return diagnosticBundleHTMLTaskSummary{
		ID:            task.ID,
		Status:        task.Status,
		TriggerSource: task.TriggerSource,
		Summary:       normalizeDiagnosticDisplayText(task.Summary),
		CreatedBy:     firstNonEmptyString(strings.TrimSpace(task.CreatedByName), fmt.Sprintf("%d", task.CreatedBy)),
		StartedAt:     task.StartedAt,
		CompletedAt:   task.CompletedAt,
		BundleDir:     normalizeDiagnosticDisplayText(task.BundleDir),
		ManifestPath:  normalizeDiagnosticDisplayText(task.ManifestPath),
		IndexPath:     normalizeDiagnosticDisplayText(task.IndexPath),
		Options:       task.Options.Normalize(),
		SelectedNodes: selectedNodes,
	}
}

func buildDiagnosticBundleHTMLTraceItems(task *DiagnosticTask) []diagnosticBundleHTMLTraceItem {
	if task == nil {
		return nil
	}
	items := []diagnosticBundleHTMLTraceItem{
		{Label: bilingualText("触发来源", "Trigger Source"), Value: normalizeDiagnosticDisplayText(string(task.TriggerSource))},
	}
	if task.SourceRef.ErrorGroupID > 0 {
		items = append(items, diagnosticBundleHTMLTraceItem{
			Label: bilingualText("错误组", "Error Group"),
			Value: fmt.Sprintf("#%d", task.SourceRef.ErrorGroupID),
		})
	}
	if task.SourceRef.InspectionReportID > 0 {
		items = append(items, diagnosticBundleHTMLTraceItem{
			Label: bilingualText("巡检报告", "Inspection Report"),
			Value: fmt.Sprintf("#%d", task.SourceRef.InspectionReportID),
		})
	}
	if task.SourceRef.InspectionFindingID > 0 {
		items = append(items, diagnosticBundleHTMLTraceItem{
			Label: bilingualText("巡检发现", "Inspection Finding"),
			Value: fmt.Sprintf("#%d", task.SourceRef.InspectionFindingID),
		})
	}
	if alertID := strings.TrimSpace(task.SourceRef.AlertID); alertID != "" {
		items = append(items, diagnosticBundleHTMLTraceItem{
			Label: bilingualText("告警 ID", "Alert ID"),
			Value: alertID,
		})
	}
	return items
}

func buildDiagnosticBundleHTMLClusterSummary(snapshot *cluster.Cluster) *diagnosticBundleHTMLClusterSummary {
	if snapshot == nil {
		return nil
	}
	nodes := make([]diagnosticBundleHTMLClusterNode, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		nodes = append(nodes, diagnosticBundleHTMLClusterNode{
			Role:       normalizeDiagnosticDisplayText(string(node.Role)),
			HostID:     node.HostID,
			InstallDir: normalizeDiagnosticDisplayText(node.InstallDir),
			Status:     normalizeDiagnosticDisplayText(string(node.Status)),
			ProcessPID: node.ProcessPID,
		})
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].HostID != nodes[j].HostID {
			return nodes[i].HostID < nodes[j].HostID
		}
		return nodes[i].Role < nodes[j].Role
	})
	return &diagnosticBundleHTMLClusterSummary{
		ID:             snapshot.ID,
		Name:           normalizeDiagnosticDisplayText(snapshot.Name),
		Version:        normalizeDiagnosticDisplayText(snapshot.Version),
		Status:         normalizeDiagnosticDisplayText(string(snapshot.Status)),
		DeploymentMode: normalizeDiagnosticDisplayText(string(snapshot.DeploymentMode)),
		InstallDir:     normalizeDiagnosticDisplayText(snapshot.InstallDir),
		NodeCount:      len(snapshot.Nodes),
		Nodes:          nodes,
	}
}

func buildDiagnosticBundleHTMLInspectionPanel(detail *ClusterInspectionReportDetailData) *diagnosticBundleHTMLInspectionPanel {
	if detail == nil || detail.Report == nil {
		return nil
	}
	findings := make([]diagnosticBundleHTMLFinding, 0, len(detail.Findings))
	for _, finding := range detail.Findings {
		if finding == nil {
			continue
		}
		findings = append(findings, diagnosticBundleHTMLFinding{
			Severity:       normalizeDiagnosticDisplayText(string(finding.Severity)),
			CheckName:      normalizeDiagnosticDisplayText(firstNonEmptyString(finding.CheckName, finding.CheckCode)),
			CheckCode:      normalizeDiagnosticDisplayText(finding.CheckCode),
			Summary:        normalizeDiagnosticDisplayText(finding.Summary),
			Recommendation: normalizeDiagnosticDisplayText(finding.Recommendation),
			Evidence:       normalizeDiagnosticDisplayText(finding.EvidenceSummary),
		})
	}
	return &diagnosticBundleHTMLInspectionPanel{
		Summary:         normalizeDiagnosticDisplayText(detail.Report.Summary),
		Status:          detail.Report.Status,
		RequestedBy:     normalizeDiagnosticDisplayText(detail.Report.RequestedBy),
		LookbackMinutes: firstNonZeroInt(detail.Report.LookbackMinutes, defaultInspectionLookbackMinutes),
		CriticalCount:   detail.Report.CriticalCount,
		WarningCount:    detail.Report.WarningCount,
		InfoCount:       detail.Report.InfoCount,
		StartedAt:       detail.Report.StartedAt,
		FinishedAt:      detail.Report.FinishedAt,
		Findings:        findings,
	}
}

func buildDiagnosticBundleHTMLErrorPanel(group *SeatunnelErrorGroup, events []*SeatunnelErrorEvent) *diagnosticBundleHTMLErrorPanel {
	if group == nil && len(events) == 0 {
		return nil
	}
	panel := &diagnosticBundleHTMLErrorPanel{
		Events: make([]diagnosticBundleHTMLErrorEvent, 0, len(events)),
	}
	if group != nil {
		panel.GroupTitle = normalizeDiagnosticDisplayText(group.Title)
		panel.ExceptionClass = normalizeDiagnosticDisplayText(group.ExceptionClass)
		panel.OccurrenceCount = group.OccurrenceCount
		panel.FirstSeenAt = &group.FirstSeenAt
		panel.LastSeenAt = &group.LastSeenAt
		panel.SampleMessage = normalizeDiagnosticDisplayText(group.SampleMessage)
	}
	for _, event := range events {
		if event == nil {
			continue
		}
		panel.Events = append(panel.Events, diagnosticBundleHTMLErrorEvent{
			OccurredAt: formatDiagnosticBundleTimeValue(event.OccurredAt),
			Role:       normalizeDiagnosticDisplayText(event.Role),
			HostLabel:  resolveDiagnosticHostLabel("", event.HostID, ""),
			SourceFile: normalizeDiagnosticDisplayText(event.SourceFile),
			JobID:      normalizeDiagnosticDisplayText(event.JobID),
			Message:    normalizeDiagnosticDisplayText(event.Message),
			Evidence:   normalizeDiagnosticDisplayText(event.Evidence),
		})
	}
	panel.RecentEventCount = len(panel.Events)
	return panel
}

func buildDiagnosticBundleHTMLAlertPanel(alerts []*monitoringapp.AlertInstance) *diagnosticBundleHTMLAlertPanel {
	if len(alerts) == 0 {
		return nil
	}
	panel := &diagnosticBundleHTMLAlertPanel{
		Alerts: make([]diagnosticBundleHTMLAlertItem, 0, len(alerts)),
	}
	for _, alert := range alerts {
		if alert == nil {
			continue
		}
		item := diagnosticBundleHTMLAlertItem{
			Name:        normalizeDiagnosticDisplayText(alert.AlertName),
			Severity:    normalizeDiagnosticDisplayText(string(alert.Severity)),
			Status:      normalizeDiagnosticDisplayText(string(alert.Status)),
			Summary:     normalizeDiagnosticDisplayText(alert.Summary),
			Description: normalizeDiagnosticDisplayText(alert.Description),
		}
		switch alert.Severity {
		case monitoringapp.AlertSeverityCritical:
			panel.Critical++
		default:
			panel.Warning++
		}
		if alert.Status == monitoringapp.AlertDisplayStatusFiring {
			panel.Firing++
		}
		panel.Alerts = append(panel.Alerts, item)
	}
	panel.Total = len(panel.Alerts)
	return panel
}

func buildDiagnosticBundleHTMLProcessPanel(events []*monitor.ProcessEvent) *diagnosticBundleHTMLProcessPanel {
	if len(events) == 0 {
		return nil
	}
	panel := &diagnosticBundleHTMLProcessPanel{
		Total:  len(events),
		Events: make([]diagnosticBundleHTMLProcessEvent, 0, len(events)),
	}
	typeCounter := make(map[string]int)
	for _, event := range events {
		if event == nil {
			continue
		}
		eventType := normalizeDiagnosticDisplayText(string(event.EventType))
		typeCounter[eventType]++
		panel.Events = append(panel.Events, diagnosticBundleHTMLProcessEvent{
			CreatedAt:   formatDiagnosticBundleTimeValue(event.CreatedAt),
			EventType:   eventType,
			ProcessName: normalizeDiagnosticDisplayText(event.ProcessName),
			NodeLabel:   resolveDiagnosticHostLabel("", event.HostID, fmt.Sprintf("node-%d", event.NodeID)),
			Details:     normalizeDiagnosticDisplayText(event.Details),
		})
	}
	typeKeys := make([]string, 0, len(typeCounter))
	for key := range typeCounter {
		typeKeys = append(typeKeys, key)
	}
	sort.Strings(typeKeys)
	panel.ByType = make([]diagnosticBundleHTMLMetricCard, 0, len(typeKeys))
	for _, key := range typeKeys {
		panel.ByType = append(panel.ByType, diagnosticBundleHTMLMetricCard{
			Label: key,
			Value: fmt.Sprintf("%d", typeCounter[key]),
		})
	}
	return panel
}

func buildDiagnosticBundleHTMLExecutionPanel(task *DiagnosticTask) diagnosticBundleHTMLExecutionPanel {
	panel := diagnosticBundleHTMLExecutionPanel{
		Steps: []diagnosticBundleHTMLExecutionStep{},
		Nodes: []diagnosticBundleHTMLExecutionNode{},
	}
	if task == nil {
		return panel
	}
	steps := make([]DiagnosticTaskStep, 0, len(task.Steps))
	steps = append(steps, task.Steps...)
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].Sequence != steps[j].Sequence {
			return steps[i].Sequence < steps[j].Sequence
		}
		return steps[i].ID < steps[j].ID
	})
	for _, step := range steps {
		panel.Steps = append(panel.Steps, diagnosticBundleHTMLExecutionStep{
			Sequence:    step.Sequence,
			Code:        string(step.Code),
			Title:       normalizeDiagnosticDisplayText(step.Title),
			Status:      normalizeDiagnosticDisplayText(string(step.Status)),
			Message:     normalizeDiagnosticDisplayText(step.Message),
			Error:       normalizeDiagnosticDisplayText(step.Error),
			StartedAt:   formatDiagnosticBundleTime(step.StartedAt),
			CompletedAt: formatDiagnosticBundleTime(step.CompletedAt),
		})
	}
	nodes := make([]DiagnosticNodeExecution, 0, len(task.NodeExecutions))
	nodes = append(nodes, task.NodeExecutions...)
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].HostID != nodes[j].HostID {
			return nodes[i].HostID < nodes[j].HostID
		}
		return nodes[i].Role < nodes[j].Role
	})
	for _, node := range nodes {
		panel.Nodes = append(panel.Nodes, diagnosticBundleHTMLExecutionNode{
			HostLabel:   resolveDiagnosticHostLabel(node.HostName, node.HostID, node.HostIP),
			Role:        normalizeDiagnosticDisplayText(node.Role),
			Status:      normalizeDiagnosticDisplayText(string(node.Status)),
			CurrentStep: normalizeDiagnosticDisplayText(string(node.CurrentStep)),
			Message:     normalizeDiagnosticDisplayText(node.Message),
			Error:       normalizeDiagnosticDisplayText(node.Error),
			StartedAt:   formatDiagnosticBundleTime(node.StartedAt),
			CompletedAt: formatDiagnosticBundleTime(node.CompletedAt),
		})
	}
	return panel
}

func buildDiagnosticBundleHTMLHealthSummary(task *DiagnosticTask, state *diagnosticBundleExecutionState, artifacts []*diagnosticBundleArtifact) diagnosticBundleHTMLHealthSummary {
	summary := diagnosticBundleHTMLHealthSummary{
		Tone:  "neutral",
		Title: bilingualText("诊断报告已生成", "Diagnostic report generated"),
		Summary: bilingualText(
			"可通过本报告快速判断当前健康信号、关键问题与诊断证据。",
			"Use this report to quickly assess health signals, key issues, and diagnostic evidence.",
		),
		Metrics: []diagnosticBundleHTMLMetricCard{},
	}
	clusterLabel := "-"
	if task != nil && task.ClusterID > 0 {
		clusterLabel = fmt.Sprintf("#%d", task.ClusterID)
	}
	selectedNodes := 0
	if task != nil {
		selectedNodes = len(task.SelectedNodes)
	}
	summary.Metrics = append(summary.Metrics,
		diagnosticBundleHTMLMetricCard{Label: bilingualText("集群", "Cluster"), Value: clusterLabel},
		diagnosticBundleHTMLMetricCard{Label: bilingualText("采集节点", "Selected Nodes"), Value: fmt.Sprintf("%d", selectedNodes)},
		diagnosticBundleHTMLMetricCard{Label: bilingualText("产物数量", "Artifacts"), Value: fmt.Sprintf("%d", len(artifacts))},
	)

	if state != nil && state.InspectionDetail != nil && state.InspectionDetail.Report != nil {
		report := state.InspectionDetail.Report
		switch {
		case report.Status == InspectionReportStatusFailed || report.CriticalCount > 0:
			summary.Tone = "critical"
			summary.Title = bilingualText("集群存在严重风险", "Cluster requires immediate attention")
			summary.Summary = normalizeDiagnosticDisplayText(firstNonEmptyString(report.Summary, report.ErrorMessage))
		case report.WarningCount > 0:
			summary.Tone = "warning"
			summary.Title = bilingualText("集群存在待排查问题", "Cluster has issues to investigate")
			summary.Summary = normalizeDiagnosticDisplayText(report.Summary)
		default:
			summary.Tone = "healthy"
			summary.Title = bilingualText("巡检未发现明显异常", "Inspection found no critical issue")
			summary.Summary = normalizeDiagnosticDisplayText(report.Summary)
		}
		summary.Metrics = append(summary.Metrics,
			diagnosticBundleHTMLMetricCard{Label: bilingualText("严重发现", "Critical Findings"), Value: fmt.Sprintf("%d", report.CriticalCount)},
			diagnosticBundleHTMLMetricCard{Label: bilingualText("告警发现", "Warning Findings"), Value: fmt.Sprintf("%d", report.WarningCount)},
			diagnosticBundleHTMLMetricCard{Label: bilingualText("信息发现", "Info Findings"), Value: fmt.Sprintf("%d", report.InfoCount)},
		)
	} else {
		errorCount := 0
		alertCount := 0
		if state != nil {
			errorCount = len(state.ErrorEvents)
			alertCount = len(state.AlertSnapshot)
		}
		if errorCount > 0 {
			summary.Tone = "warning"
			summary.Title = bilingualText("诊断报告包含错误证据", "Diagnostic report includes error evidence")
		}
		if alertCount > 0 {
			summary.Tone = "warning"
		}
		summary.Metrics = append(summary.Metrics,
			diagnosticBundleHTMLMetricCard{Label: bilingualText("错误事件", "Error Events"), Value: fmt.Sprintf("%d", errorCount)},
			diagnosticBundleHTMLMetricCard{Label: bilingualText("活动告警", "Active Alerts"), Value: fmt.Sprintf("%d", alertCount)},
		)
	}
	return summary
}

func buildDiagnosticBundleHTMLArtifactGroups(bundleDir string, artifacts []*diagnosticBundleArtifact) []diagnosticBundleHTMLArtifactGroup {
	if len(artifacts) == 0 {
		return []diagnosticBundleHTMLArtifactGroup{}
	}
	groupMap := make(map[string][]diagnosticBundleHTMLArtifactView)
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		view := buildDiagnosticBundleHTMLArtifactView(bundleDir, artifact)
		groupMap[artifact.Category] = append(groupMap[artifact.Category], view)
	}
	order := []string{
		"error_context",
		"config_snapshot",
		"alert_snapshot",
		"process_events",
		"log_sample",
		"thread_dump",
		"jvm_dump",
		"manifest",
		"html_summary",
		"diagnostic_report",
	}
	seen := make(map[string]struct{}, len(order))
	result := make([]diagnosticBundleHTMLArtifactGroup, 0, len(groupMap))
	for _, key := range order {
		items, ok := groupMap[key]
		if !ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, diagnosticBundleHTMLArtifactGroup{
			Key:   key,
			Label: resolveDiagnosticArtifactCategoryLabel(key),
			Items: items,
		})
	}
	remaining := make([]string, 0)
	for key := range groupMap {
		if _, ok := seen[key]; ok {
			continue
		}
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		result = append(result, diagnosticBundleHTMLArtifactGroup{
			Key:   key,
			Label: resolveDiagnosticArtifactCategoryLabel(key),
			Items: groupMap[key],
		})
	}
	return result
}

func buildDiagnosticBundleHTMLRecommendations(task *DiagnosticTask, state *diagnosticBundleExecutionState) []diagnosticBundleHTMLAdvice {
	items := make([]diagnosticBundleHTMLAdvice, 0, 6)
	appendAdvice := func(title, details string) {
		title = strings.TrimSpace(title)
		details = strings.TrimSpace(details)
		if title == "" && details == "" {
			return
		}
		items = append(items, diagnosticBundleHTMLAdvice{
			Title:   normalizeDiagnosticDisplayText(title),
			Details: normalizeDiagnosticDisplayText(details),
		})
	}

	if state != nil && state.InspectionDetail != nil {
		for _, finding := range state.InspectionDetail.Findings {
			if finding == nil {
				continue
			}
			appendAdvice(
				firstNonEmptyString(finding.CheckName, finding.Summary),
				firstNonEmptyString(finding.Recommendation, finding.EvidenceSummary),
			)
			if len(items) >= 4 {
				break
			}
		}
	}
	if len(items) == 0 && state != nil && state.ErrorGroup != nil {
		appendAdvice(
			bilingualText("优先排查错误组根因", "Prioritize the root cause of the error group"),
			firstNonEmptyString(state.ErrorGroup.SampleMessage, state.ErrorGroup.Title),
		)
	}
	if len(items) == 0 && state != nil && len(state.AlertSnapshot) > 0 {
		alert := state.AlertSnapshot[0]
		if alert != nil {
			appendAdvice(
				bilingualText("先处理活动告警", "Address active alerts first"),
				firstNonEmptyString(alert.Summary, alert.Description, alert.AlertName),
			)
		}
	}
	if len(items) == 0 {
		appendAdvice(
			bilingualText("复核诊断证据完整性", "Review diagnostic evidence completeness"),
			bilingualText("优先查看诊断报告中的日志样本、配置快照与执行过程，再决定是否需要重新采集或升级为更深层排查。", "Review log samples, config snapshots, and execution evidence in this report before deciding whether deeper collection is required."),
		)
	}
	if task != nil && task.Options.IncludeJVMDump {
		appendAdvice(
			bilingualText("复查 JVM Dump 元数据", "Review JVM dump metadata"),
			bilingualText("当前 MVP 仅登记 JVM Dump 远端路径与元数据，如需 hprof 二进制回传请走后续增强能力。", "The MVP records JVM dump metadata and remote paths only. Binary HPROF upload is deferred to a later enhancement."),
		)
	}
	return items
}

func buildDiagnosticBundleHTMLPassedChecks(task *DiagnosticTask, state *diagnosticBundleExecutionState, artifacts []*diagnosticBundleArtifact) []diagnosticBundleHTMLAdvice {
	items := make([]diagnosticBundleHTMLAdvice, 0, 8)
	appendAdvice := func(title, details string) {
		title = strings.TrimSpace(title)
		details = strings.TrimSpace(details)
		if title == "" && details == "" {
			return
		}
		items = append(items, diagnosticBundleHTMLAdvice{
			Title:   normalizeDiagnosticDisplayText(title),
			Details: normalizeDiagnosticDisplayText(details),
		})
	}

	artifactCountByCategory := make(map[string]int)
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		artifactCountByCategory[strings.TrimSpace(artifact.Category)]++
	}
	if artifactCountByCategory["config_snapshot"] > 0 {
		appendAdvice(
			bilingualText("配置快照已采集", "Configuration snapshot collected"),
			bilingualText("报告已附带集群配置与运行环境快照。", "The report contains configuration and runtime snapshots."),
		)
	}
	if artifactCountByCategory["log_sample"] > 0 {
		appendAdvice(
			bilingualText("日志样本已落盘", "Log sample collected"),
			bilingualText("至少一个节点已采集到用于排查的日志样本。", "At least one node produced a log sample for investigation."),
		)
	}
	if artifactCountByCategory["thread_dump"] > 0 {
		appendAdvice(
			bilingualText("线程栈已采集", "Thread dump collected"),
			bilingualText("可直接打开线程栈产物定位阻塞、卡死或线程热点。", "Thread dump artifacts are available for blocking and hotspot analysis."),
		)
	}
	if task != nil && !task.Options.IncludeJVMDump {
		appendAdvice(
			bilingualText("JVM Dump 按策略跳过", "JVM dump intentionally skipped"),
			bilingualText("当前任务未开启 JVM Dump，不视为任务失败。", "JVM dump was disabled by task option and is not treated as a failure."),
		)
	}
	if state != nil && state.InspectionDetail != nil && state.InspectionDetail.Report != nil && state.InspectionDetail.Report.CriticalCount == 0 {
		appendAdvice(
			bilingualText("巡检未发现严重问题", "No critical inspection findings"),
			bilingualText("本次巡检上下文中没有 critical 级别发现项。", "The inspection context contains no critical findings."),
		)
	}
	if state != nil && len(state.AlertSnapshot) == 0 {
		appendAdvice(
			bilingualText("未检测到活动告警", "No active alerts detected"),
			bilingualText("告警快照为空，说明当前上下文没有关联的 firing 告警。", "The alert snapshot is empty, which means no firing alerts were associated with this context."),
		)
	}
	return items
}

func buildDiagnosticBundleHTMLArtifactView(bundleDir string, artifact *diagnosticBundleArtifact) diagnosticBundleHTMLArtifactView {
	relativePath := resolveDiagnosticBundleRelativePath(bundleDir, artifact.Path)
	preview, previewNote := readDiagnosticArtifactPreview(artifact)
	return diagnosticBundleHTMLArtifactView{
		Category:      artifact.Category,
		CategoryLabel: resolveDiagnosticArtifactCategoryLabel(artifact.Category),
		StepCode:      normalizeDiagnosticDisplayText(string(artifact.StepCode)),
		Status:        normalizeDiagnosticDisplayText(artifact.Status),
		Format:        normalizeDiagnosticDisplayText(artifact.Format),
		HostLabel:     resolveDiagnosticHostLabel(artifact.HostName, artifact.HostID, ""),
		LocalPath:     normalizeDiagnosticDisplayText(artifact.Path),
		RelativePath:  normalizeDiagnosticDisplayText(relativePath),
		RemotePath:    normalizeDiagnosticDisplayText(artifact.RemotePath),
		SizeLabel:     formatDiagnosticBytes(artifact.SizeBytes),
		Message:       normalizeDiagnosticDisplayText(artifact.Message),
		Preview:       preview,
		PreviewNote:   previewNote,
	}
}

func resolveDiagnosticArtifactCategoryLabel(category string) string {
	switch strings.TrimSpace(category) {
	case "error_context":
		return bilingualText("错误上下文", "Error Context")
	case "process_events":
		return bilingualText("进程事件", "Process Events")
	case "alert_snapshot":
		return bilingualText("告警快照", "Alert Snapshot")
	case "config_snapshot":
		return bilingualText("配置快照", "Config Snapshot")
	case "log_sample":
		return bilingualText("日志样本", "Log Sample")
	case "thread_dump":
		return bilingualText("线程栈", "Thread Dump")
	case "jvm_dump":
		return bilingualText("JVM Dump", "JVM Dump")
	case "manifest":
		return bilingualText("Manifest", "Manifest")
	case "html_summary":
		return bilingualText("诊断报告", "Diagnostic Report")
	case "diagnostic_report":
		return bilingualText("诊断报告", "Diagnostic Report")
	default:
		return normalizeDiagnosticDisplayText(category)
	}
}

func resolveDiagnosticBundleRelativePath(bundleDir, path string) string {
	trimmedBundle := strings.TrimSpace(bundleDir)
	trimmedPath := strings.TrimSpace(path)
	if trimmedBundle == "" || trimmedPath == "" {
		return ""
	}
	relative, err := filepath.Rel(trimmedBundle, trimmedPath)
	if err != nil {
		return trimmedPath
	}
	if strings.HasPrefix(relative, "..") {
		return trimmedPath
	}
	return filepath.ToSlash(relative)
}

func readDiagnosticArtifactPreview(artifact *diagnosticBundleArtifact) (string, string) {
	if artifact == nil {
		return "", ""
	}
	category := strings.TrimSpace(artifact.Category)
	if category == "diagnostic_report" || category == "html_summary" {
		return "", bilingualText("当前文件即离线诊断报告首页。", "This file is the offline diagnostic report itself.")
	}
	path := strings.TrimSpace(artifact.Path)
	if path == "" {
		return "", ""
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", bilingualText("产物预览读取失败。", "Failed to read artifact preview.")
	}
	content := string(bytes)
	if strings.TrimSpace(content) == "" {
		return "", bilingualText("产物文件为空。", "Artifact file is empty.")
	}
	const maxPreviewRunes = 6000
	preview, truncated := truncateDiagnosticText(content, maxPreviewRunes)
	if truncated {
		return preview, bilingualText("仅展示前 6000 个字符，完整内容请打开对应文件。", "Preview shows the first 6000 characters only. Open the file for full content.")
	}
	return preview, bilingualText("已展示完整产物内容。", "Showing full artifact content.")
}

func truncateDiagnosticText(value string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return "", false
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value, false
	}
	return string(runes[:maxRunes]), true
}

func resolveDiagnosticHostLabel(hostName string, hostID uint, fallback string) string {
	if trimmed := strings.TrimSpace(hostName); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(fallback); trimmed != "" {
		return trimmed
	}
	if hostID > 0 {
		return fmt.Sprintf("#%d", hostID)
	}
	return "-"
}

func normalizeDiagnosticDisplayText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func formatDiagnosticBundleTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "-"
	}
	return formatDiagnosticBundleTimeValue(*value)
}

func formatDiagnosticBundleTimeValue(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04:05 MST")
}

func formatDiagnosticBytes(size int64) string {
	if size <= 0 {
		return "-"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value = value / 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func diagnosticHTMLStatusClass(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "completed", "healthy", "created":
		return "status-ok"
	case "failed", "critical":
		return "status-critical"
	case "running", "warning":
		return "status-warn"
	case "skipped":
		return "status-skip"
	default:
		return "status-neutral"
	}
}

func diagnosticHTMLToneClass(tone string) string {
	switch strings.ToLower(strings.TrimSpace(tone)) {
	case "healthy":
		return "tone-healthy"
	case "warning":
		return "tone-warning"
	case "critical":
		return "tone-critical"
	default:
		return "tone-neutral"
	}
}

func diagnosticTaskBundleDir(taskID uint) string {
	baseDir := strings.TrimSpace(config.GetStorageConfig().BaseDir)
	if baseDir == "" {
		baseDir = "./data/storage"
	}
	return filepath.Join(baseDir, "diagnostics", "tasks", fmt.Sprintf("%d", taskID))
}

func buildDiagnosticLogCandidates(target DiagnosticTaskNodeTarget, errorEvents []*SeatunnelErrorEvent) []string {
	candidates := make([]string, 0, 4)
	seen := make(map[string]struct{})
	for _, event := range errorEvents {
		if event == nil {
			continue
		}
		if event.HostID != target.HostID {
			continue
		}
		if path := strings.TrimSpace(event.SourceFile); path != "" {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				candidates = append(candidates, path)
			}
		}
	}
	// target.InstallDir 是远端 Seatunnel 安装路径，通常运行在 Linux 上，这里必须使用 POSIX 路径拼接。
	defaultLog := path.Join(target.InstallDir, "logs", diagnosticDefaultLogFile(target.Role))
	if _, ok := seen[defaultLog]; !ok {
		candidates = append(candidates, defaultLog)
	}
	return candidates
}

func diagnosticDefaultLogFile(role string) string {
	switch strings.TrimSpace(role) {
	case "master":
		return "seatunnel-engine-master.log"
	case "worker":
		return "seatunnel-engine-worker.log"
	default:
		return "seatunnel-engine-server.log"
	}
}

func sortedDiagnosticTaskTargets(targets DiagnosticTaskNodeTargets) []DiagnosticTaskNodeTarget {
	items := make([]DiagnosticTaskNodeTarget, 0, len(targets))
	items = append(items, targets...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].HostID != items[j].HostID {
			return items[i].HostID < items[j].HostID
		}
		return items[i].Role < items[j].Role
	})
	return items
}

func normalizeDiagnosticFileRole(role string) string {
	role = strings.TrimSpace(role)
	if role == "" || role == "master/worker" {
		return "hybrid"
	}
	return strings.ReplaceAll(role, "/", "-")
}

const diagnosticBundleHTMLTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>SeaTunnelX Diagnostic Report</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f4f7fb;
      --panel: #ffffff;
      --panel-soft: #f8fbff;
      --border: #d9e2ec;
      --border-strong: #c6d3e1;
      --muted: #64748b;
      --text: #0f172a;
      --primary: #2563eb;
      --ok: #10b981;
      --ok-soft: #ecfdf5;
      --warn: #f59e0b;
      --warn-soft: #fff7ed;
      --critical: #ef4444;
      --critical-soft: #fef2f2;
      --neutral: #3b82f6;
      --neutral-soft: #eff6ff;
      --skip: #94a3b8;
      --skip-soft: #f8fafc;
      --code-bg: #0f172a;
      --code-text: #e2e8f0;
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body {
      margin: 0;
      padding: 28px;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--text);
      background: var(--bg);
      line-height: 1.6;
    }
    h1, h2, h3, h4, p { margin: 0; }
    a { color: var(--primary); text-decoration: none; }
    a:hover { text-decoration: underline; }
    .page {
      max-width: 1360px;
      margin: 0 auto;
      display: flex;
      flex-direction: column;
      gap: 18px;
    }
    .hero {
      background: var(--panel);
      border: 1px solid var(--border-strong);
      border-radius: 16px;
      padding: 24px 28px;
    }
    .hero-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.35fr) minmax(320px, 0.65fr);
      gap: 24px;
      align-items: start;
    }
    .hero-badges {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-bottom: 14px;
    }
    .hero-kicker {
      color: var(--primary);
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.08em;
      text-transform: uppercase;
      margin-bottom: 8px;
    }
    .hero-title {
      font-size: 32px;
      line-height: 1.2;
      margin-bottom: 10px;
    }
    .hero-summary {
      color: var(--muted);
      max-width: 840px;
      font-size: 15px;
    }
    .hero-side {
      border: 1px solid var(--border);
      border-radius: 14px;
      background: var(--panel-soft);
      padding: 18px;
    }
    .hero-side.tone-healthy {
      background: linear-gradient(180deg, var(--ok-soft) 0, #fff 100%);
      border-color: rgba(16,185,129,0.28);
    }
    .hero-side.tone-warning {
      background: linear-gradient(180deg, var(--warn-soft) 0, #fff 100%);
      border-color: rgba(245,158,11,0.28);
    }
    .hero-side.tone-critical {
      background: linear-gradient(180deg, var(--critical-soft) 0, #fff 100%);
      border-color: rgba(239,68,68,0.28);
    }
    .hero-side.tone-neutral {
      background: linear-gradient(180deg, var(--neutral-soft) 0, #fff 100%);
      border-color: rgba(59,130,246,0.24);
    }
    .side-label,
    .focus-label,
    .panel-label,
    .subsection-label {
      color: var(--muted);
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.04em;
      text-transform: uppercase;
      margin-bottom: 10px;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      padding: 4px 10px;
      border-radius: 999px;
      border: 1px solid var(--border);
      font-size: 12px;
      font-weight: 600;
      background: #fff;
      color: var(--text);
    }
    .status-ok { background: var(--ok-soft); border-color: rgba(16,185,129,0.32); }
    .status-warn { background: var(--warn-soft); border-color: rgba(245,158,11,0.32); }
    .status-critical { background: var(--critical-soft); border-color: rgba(239,68,68,0.32); }
    .status-neutral { background: var(--neutral-soft); border-color: rgba(59,130,246,0.24); }
    .status-skip { background: var(--skip-soft); border-color: rgba(148,163,184,0.36); }
    .metric-grid,
    .stat-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }
    .metric-card,
    .stat-card {
      border: 1px solid var(--border);
      border-radius: 12px;
      background: #fff;
      padding: 14px;
      min-height: 96px;
    }
    .metric-card .label,
    .stat-card .label {
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 8px;
    }
    .metric-card .value,
    .stat-card .value {
      font-size: 24px;
      font-weight: 700;
      line-height: 1.15;
    }
    .metric-card .note,
    .stat-card .note {
      color: var(--muted);
      font-size: 12px;
      margin-top: 8px;
      line-height: 1.5;
    }
    .report-nav {
      position: sticky;
      top: 12px;
      z-index: 20;
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      padding: 10px 12px;
      border: 1px solid rgba(198,211,225,0.9);
      border-radius: 14px;
      background: rgba(255,255,255,0.92);
      backdrop-filter: blur(10px);
    }
    .report-nav a {
      display: inline-flex;
      align-items: center;
      padding: 8px 12px;
      border-radius: 999px;
      background: #f8fbff;
      color: #1e293b;
      font-size: 13px;
      font-weight: 600;
    }
    .report-nav a:hover {
      background: #eef4ff;
      text-decoration: none;
    }
    .section {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: 16px;
      padding: 22px 24px;
    }
    .section-heading {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 18px;
    }
    .section-lead {
      margin-top: 6px;
      color: var(--muted);
      font-size: 14px;
    }
    .grid-2 {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
      gap: 18px;
    }
    .focus-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.1fr) minmax(0, 0.9fr) minmax(0, 1fr);
      gap: 16px;
    }
    .focus-panel,
    .detail-panel {
      border: 1px solid var(--border);
      border-radius: 14px;
      background: #fff;
      padding: 18px;
    }
    .focus-panel.tone-healthy {
      background: linear-gradient(180deg, var(--ok-soft) 0, #fff 100%);
      border-color: rgba(16,185,129,0.28);
    }
    .focus-panel.tone-warning {
      background: linear-gradient(180deg, var(--warn-soft) 0, #fff 100%);
      border-color: rgba(245,158,11,0.28);
    }
    .focus-panel.tone-critical {
      background: linear-gradient(180deg, var(--critical-soft) 0, #fff 100%);
      border-color: rgba(239,68,68,0.28);
    }
    .focus-panel.tone-neutral {
      background: linear-gradient(180deg, var(--neutral-soft) 0, #fff 100%);
      border-color: rgba(59,130,246,0.24);
    }
    .focus-panel h3,
    .detail-title {
      font-size: 20px;
      line-height: 1.35;
      margin-bottom: 10px;
    }
    .focus-panel p,
    .detail-panel p {
      color: var(--muted);
    }
    .panel-note {
      margin-top: 12px;
      color: var(--muted);
      font-size: 13px;
    }
    .detail-columns {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
      gap: 18px;
    }
    .dl {
      display: flex;
      flex-direction: column;
      gap: 0;
    }
    .dl-row {
      display: grid;
      grid-template-columns: 180px minmax(0, 1fr);
      gap: 14px;
      padding: 10px 0;
      border-bottom: 1px solid #edf2f7;
    }
    .dl-row:last-child { border-bottom: none; }
    .dl-term {
      color: var(--muted);
      font-size: 13px;
    }
    .dl-value {
      word-break: break-word;
      font-size: 14px;
    }
    .subsection + .subsection {
      margin-top: 18px;
      padding-top: 18px;
      border-top: 1px solid #edf2f7;
    }
    .list {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .entry {
      padding: 14px 0;
      border-bottom: 1px solid #edf2f7;
    }
    .entry:first-child { padding-top: 0; }
    .entry:last-child {
      border-bottom: none;
      padding-bottom: 0;
    }
    .entry-header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
      flex-wrap: wrap;
      margin-bottom: 8px;
    }
    .entry-title {
      font-weight: 700;
      line-height: 1.5;
    }
    .muted {
      color: var(--muted);
    }
    .small { font-size: 12px; }
    .callout {
      margin-top: 14px;
      padding: 14px 16px;
      border-radius: 12px;
      background: #f8fbff;
      border: 1px solid var(--border);
    }
    .callout.critical {
      background: var(--critical-soft);
      border-color: rgba(239,68,68,0.28);
    }
    .callout.warn {
      background: var(--warn-soft);
      border-color: rgba(245,158,11,0.28);
    }
    .list-clean {
      margin: 0;
      padding-left: 18px;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }
    .list-clean li { color: #1e293b; }
    .table-wrap {
      border: 1px solid var(--border);
      border-radius: 14px;
      overflow: hidden;
      margin-top: 14px;
      background: #fff;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
    }
    th, td {
      padding: 12px 14px;
      text-align: left;
      border-bottom: 1px solid #edf2f7;
      vertical-align: top;
      word-break: break-word;
    }
    th {
      background: #f8fbff;
      color: var(--muted);
      font-weight: 600;
    }
    tr:last-child td { border-bottom: none; }
    .artifact-group + .artifact-group {
      margin-top: 22px;
      padding-top: 22px;
      border-top: 1px solid #edf2f7;
    }
    .artifact-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
      gap: 14px;
    }
    .artifact-card {
      border: 1px solid var(--border);
      border-radius: 14px;
      background: #fff;
      padding: 16px;
      display: flex;
      flex-direction: column;
      gap: 12px;
    }
    .artifact-meta {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
    }
    .artifact-meta .meta-item {
      border: 1px solid #edf2f7;
      border-radius: 10px;
      padding: 10px 12px;
      background: #fafcff;
    }
    .meta-item .label {
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 6px;
    }
    .meta-item .value {
      font-size: 13px;
      word-break: break-word;
    }
    code.inline {
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
      background: #eff6ff;
      color: #1d4ed8;
      padding: 2px 6px;
      border-radius: 6px;
      word-break: break-all;
    }
    pre {
      margin: 0;
      background: var(--code-bg);
      color: var(--code-text);
      padding: 16px;
      border-radius: 12px;
      overflow: auto;
      line-height: 1.6;
      font-size: 12px;
      max-height: 420px;
      white-space: pre-wrap;
      word-break: break-word;
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
    }
    details {
      border: 1px dashed var(--border);
      border-radius: 12px;
      padding: 12px 14px;
      background: #f8fafc;
    }
    details summary {
      cursor: pointer;
      font-weight: 600;
    }
    .empty {
      border: 1px dashed var(--border);
      border-radius: 12px;
      padding: 18px;
      color: var(--muted);
      background: #fafcff;
    }
    @media (max-width: 1120px) {
      .hero-grid,
      .focus-grid {
        grid-template-columns: 1fr;
      }
    }
    @media (max-width: 900px) {
      body { padding: 16px; }
      .section, .hero { padding: 18px; }
      .dl-row {
        grid-template-columns: 1fr;
        gap: 6px;
      }
      .metric-grid,
      .stat-grid,
      .artifact-meta {
        grid-template-columns: 1fr;
      }
    }
  </style>
</head>
<body>
  <div class="page">
    <header class="hero">
      <div class="hero-grid">
        <div class="hero-main">
          <div class="hero-badges">
            <span class="badge {{statusClass .Task.Status}}">{{.Task.Status}}</span>
            <span class="badge {{statusClass .Health.Tone}}">{{.Health.Title}}</span>
            <span class="badge">Task #{{.Task.ID}}</span>
            <span class="badge">Generated {{formatTime .GeneratedAt}}</span>
          </div>
          <div class="hero-kicker">SeaTunnelX</div>
          <h1 class="hero-title">诊断报告 / Diagnostic Report</h1>
          <p class="hero-summary">{{.Health.Summary}}</p>
        </div>
        <aside class="hero-side {{toneClass .Health.Tone}}">
          <div class="side-label">核心指标 / Key Signals</div>
          <div class="metric-grid">
            {{range .Health.Metrics}}
            <div class="metric-card">
              <div class="label">{{.Label}}</div>
              <div class="value">{{.Value}}</div>
              {{if .Note}}<div class="note">{{.Note}}</div>{{end}}
            </div>
            {{end}}
          </div>
        </aside>
      </div>
    </header>

    <nav class="report-nav">
      <a href="#focus">核心结论</a>
      <a href="#evidence">关键证据</a>
      <a href="#overview">任务概览</a>
      <a href="#execution">执行过程</a>
      <a href="#artifacts">诊断产物</a>
      <a href="#appendix">集群附录</a>
    </nav>

    <section class="section" id="focus">
      <div class="section-heading">
        <div>
          <h2>核心结论 / Executive Summary</h2>
          <p class="section-lead">建议先看当前结论、影响范围和下一步动作，再进入错误明细与产物。</p>
        </div>
      </div>
      <div class="focus-grid">
        <article class="focus-panel {{toneClass .Health.Tone}}">
          <div class="focus-label">当前结论 / Current Assessment</div>
          <h3>{{.Health.Title}}</h3>
          <p>{{.Health.Summary}}</p>
          {{if .ErrorContext}}
          <div class="panel-note">最近关联错误组：{{.ErrorContext.GroupTitle}}</div>
          {{else if .Inspection}}
          <div class="panel-note">本报告包含巡检上下文，可结合巡检发现确认问题影响范围。</div>
          {{else}}
          <div class="panel-note">当前报告主要依据任务执行证据与已收集产物生成。</div>
          {{end}}
        </article>

        <article class="focus-panel">
          <div class="focus-label">影响范围 / Impact Scope</div>
          <div class="stat-grid">
            <div class="stat-card">
              <div class="label">Cluster Nodes</div>
              <div class="value">{{if .Cluster}}{{.Cluster.NodeCount}}{{else}}-{{end}}</div>
            </div>
            <div class="stat-card">
              <div class="label">Error Occurrences</div>
              <div class="value">{{if .ErrorContext}}{{.ErrorContext.OccurrenceCount}}{{else}}-{{end}}</div>
            </div>
            <div class="stat-card">
              <div class="label">Critical Findings</div>
              <div class="value">{{if .Inspection}}{{.Inspection.CriticalCount}}{{else}}-{{end}}</div>
            </div>
            <div class="stat-card">
              <div class="label">Firing Alerts</div>
              <div class="value">{{if .AlertSnapshot}}{{.AlertSnapshot.Firing}}{{else}}-{{end}}</div>
            </div>
          </div>
          <div class="panel-note">
            {{if .Inspection}}
            巡检窗口：最近 {{.Inspection.LookbackMinutes}} 分钟；共发现 {{.Inspection.CriticalCount}} 严重 / {{.Inspection.WarningCount}} 告警 / {{.Inspection.InfoCount}} 信息。
            {{else}}
            当前影响范围以错误事件、告警快照和任务采集结果为准。
            {{end}}
          </div>
        </article>

        <article class="focus-panel">
          <div class="focus-label">优先动作 / Recommended Next Step</div>
          {{if .Recommendations}}
          <ul class="list-clean">
            {{range .Recommendations}}
            <li>
              <strong>{{.Title}}</strong>
              <div class="muted small">{{.Details}}</div>
            </li>
            {{end}}
          </ul>
          {{else}}
          <div class="empty">当前报告没有生成额外建议，请直接查看错误上下文与任务执行过程。 / No extra recommendations were generated for this report.</div>
          {{end}}
        </article>
      </div>
    </section>

    <section class="section" id="evidence">
      <div class="section-heading">
        <div>
          <h2>关键证据 / Key Evidence</h2>
          <p class="section-lead">这一部分用于回答“问题是什么、现在是否仍在发生、影响到了哪里”。</p>
        </div>
      </div>

      <div class="grid-2">
        <div class="detail-panel">
          <div class="panel-label">错误上下文 / Error Context</div>
          {{if .ErrorContext}}
          <div class="dl">
            <div class="dl-row"><div class="dl-term">错误组</div><div class="dl-value">{{.ErrorContext.GroupTitle}}</div></div>
            <div class="dl-row"><div class="dl-term">异常类型</div><div class="dl-value">{{.ErrorContext.ExceptionClass}}</div></div>
            <div class="dl-row"><div class="dl-term">累计次数</div><div class="dl-value">{{.ErrorContext.OccurrenceCount}}</div></div>
            <div class="dl-row"><div class="dl-term">最近事件数</div><div class="dl-value">{{.ErrorContext.RecentEventCount}}</div></div>
            <div class="dl-row"><div class="dl-term">首次出现</div><div class="dl-value">{{formatTime .ErrorContext.FirstSeenAt}}</div></div>
            <div class="dl-row"><div class="dl-term">最近出现</div><div class="dl-value">{{formatTime .ErrorContext.LastSeenAt}}</div></div>
          </div>
          <div class="callout critical">{{.ErrorContext.SampleMessage}}</div>
          {{if .ErrorContext.Events}}
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Occurred At</th>
                  <th>Host</th>
                  <th>Role</th>
                  <th>Job ID</th>
                  <th>Source File</th>
                </tr>
              </thead>
              <tbody>
                {{range .ErrorContext.Events}}
                <tr>
                  <td>{{.OccurredAt}}</td>
                  <td>{{.HostLabel}}</td>
                  <td>{{.Role}}</td>
                  <td>{{.JobID}}</td>
                  <td><code class="inline">{{.SourceFile}}</code></td>
                </tr>
                <tr>
                  <td colspan="5">
                    <div>{{.Message}}</div>
                    {{if .Evidence}}<div class="muted small" style="margin-top: 6px;">{{.Evidence}}</div>{{end}}
                  </td>
                </tr>
                {{end}}
              </tbody>
            </table>
          </div>
          {{end}}
          {{else}}
          <div class="empty">当前诊断报告未附带错误组上下文。 / No error-group context is attached to this report.</div>
          {{end}}
        </div>

        <div class="detail-panel">
          <div class="panel-label">巡检结果 / Inspection Summary</div>
          {{if .Inspection}}
          <div class="dl">
            <div class="dl-row"><div class="dl-term">Summary</div><div class="dl-value">{{.Inspection.Summary}}</div></div>
            <div class="dl-row"><div class="dl-term">Status</div><div class="dl-value"><span class="badge {{statusClass .Inspection.Status}}">{{.Inspection.Status}}</span></div></div>
            <div class="dl-row"><div class="dl-term">Requested By</div><div class="dl-value">{{.Inspection.RequestedBy}}</div></div>
            <div class="dl-row"><div class="dl-term">Lookback Window</div><div class="dl-value">{{.Inspection.LookbackMinutes}} min</div></div>
            <div class="dl-row"><div class="dl-term">Started At</div><div class="dl-value">{{formatTime .Inspection.StartedAt}}</div></div>
            <div class="dl-row"><div class="dl-term">Finished At</div><div class="dl-value">{{formatTime .Inspection.FinishedAt}}</div></div>
          </div>
          <div class="subsection">
            <div class="subsection-label">结构化发现 / Findings</div>
            {{if .Inspection.Findings}}
            <div class="list">
              {{range .Inspection.Findings}}
              <div class="entry">
                <div class="entry-header">
                  <div>
                    <div class="entry-title">{{.CheckName}}</div>
                    <div class="muted small">{{.CheckCode}}</div>
                  </div>
                  <span class="badge {{statusClass .Severity}}">{{.Severity}}</span>
                </div>
                <div>{{.Summary}}</div>
                {{if .Evidence}}<div class="muted" style="margin-top: 8px;">{{.Evidence}}</div>{{end}}
                {{if .Recommendation}}<div class="muted" style="margin-top: 8px;">{{.Recommendation}}</div>{{end}}
              </div>
              {{end}}
            </div>
            {{else}}
            <div class="empty">No inspection findings were recorded.</div>
            {{end}}
          </div>
          {{else}}
          <div class="empty">当前诊断报告没有巡检详情上下文。 / No inspection context is attached to this report.</div>
          {{end}}
        </div>
      </div>

      <div class="grid-2" style="margin-top: 18px;">
        <div class="detail-panel">
          <div class="panel-label">告警快照 / Alert Snapshot</div>
          {{if .AlertSnapshot}}
          <div class="stat-grid">
            <div class="stat-card"><div class="label">Total Alerts</div><div class="value">{{.AlertSnapshot.Total}}</div></div>
            <div class="stat-card"><div class="label">Critical</div><div class="value">{{.AlertSnapshot.Critical}}</div></div>
            <div class="stat-card"><div class="label">Warning</div><div class="value">{{.AlertSnapshot.Warning}}</div></div>
            <div class="stat-card"><div class="label">Firing</div><div class="value">{{.AlertSnapshot.Firing}}</div></div>
          </div>
          <div class="subsection">
            <div class="subsection-label">告警明细 / Alert Details</div>
            <div class="list">
              {{range .AlertSnapshot.Alerts}}
              <div class="entry">
                <div class="entry-header">
                  <div class="entry-title">{{.Name}}</div>
                  <div style="display:flex; gap:8px; flex-wrap:wrap;">
                    <span class="badge {{statusClass .Severity}}">{{.Severity}}</span>
                    <span class="badge {{statusClass .Status}}">{{.Status}}</span>
                  </div>
                </div>
                {{if .Summary}}<div>{{.Summary}}</div>{{end}}
                {{if .Description}}<div class="muted" style="margin-top: 8px;">{{.Description}}</div>{{end}}
              </div>
              {{end}}
            </div>
          </div>
          {{else}}
          <div class="empty">未采集到活动告警。 / No alert snapshot was collected.</div>
          {{end}}
        </div>

        <div class="detail-panel">
          <div class="panel-label">进程信号 / Process Signals</div>
          {{if .ProcessEvents}}
          <div class="stat-grid">
            <div class="stat-card"><div class="label">Total Events</div><div class="value">{{.ProcessEvents.Total}}</div></div>
            {{range .ProcessEvents.ByType}}
            <div class="stat-card"><div class="label">{{.Label}}</div><div class="value">{{.Value}}</div></div>
            {{end}}
          </div>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Created At</th>
                  <th>Event Type</th>
                  <th>Process</th>
                  <th>Node</th>
                  <th>Details</th>
                </tr>
              </thead>
              <tbody>
                {{range .ProcessEvents.Events}}
                <tr>
                  <td>{{.CreatedAt}}</td>
                  <td>{{.EventType}}</td>
                  <td>{{.ProcessName}}</td>
                  <td>{{.NodeLabel}}</td>
                  <td>{{.Details}}</td>
                </tr>
                {{end}}
              </tbody>
            </table>
          </div>
          {{else}}
          <div class="empty">未采集到近期进程事件。 / No process events were collected.</div>
          {{end}}
        </div>
      </div>
    </section>

    <section class="section" id="overview">
      <div class="section-heading">
        <div>
          <h2>任务概览 / Task Overview</h2>
          <p class="section-lead">回答“这份报告从哪里来、由谁触发、针对哪些节点、采集了哪些能力”。</p>
        </div>
      </div>

      <div class="detail-columns">
        <div class="detail-panel">
          <div class="panel-label">任务信息 / Task Metadata</div>
          <div class="dl">
            <div class="dl-row"><div class="dl-term">任务摘要 / Summary</div><div class="dl-value">{{.Task.Summary}}</div></div>
            <div class="dl-row"><div class="dl-term">创建人 / Created By</div><div class="dl-value">{{.Task.CreatedBy}}</div></div>
            <div class="dl-row"><div class="dl-term">开始时间 / Started At</div><div class="dl-value">{{formatTime .Task.StartedAt}}</div></div>
            <div class="dl-row"><div class="dl-term">完成时间 / Completed At</div><div class="dl-value">{{formatTime .Task.CompletedAt}}</div></div>
            <div class="dl-row"><div class="dl-term">诊断包目录 / Bundle Dir</div><div class="dl-value"><code class="inline">{{.Task.BundleDir}}</code></div></div>
            <div class="dl-row"><div class="dl-term">Manifest</div><div class="dl-value"><code class="inline">{{.Task.ManifestPath}}</code></div></div>
            <div class="dl-row"><div class="dl-term">Report Index</div><div class="dl-value"><code class="inline">{{.Task.IndexPath}}</code></div></div>
          </div>
        </div>

        <div class="detail-panel">
          <div class="panel-label">来源与选项 / Source & Options</div>
          <div class="dl">
            {{range .SourceTraceability}}
            <div class="dl-row"><div class="dl-term">{{.Label}}</div><div class="dl-value">{{.Value}}</div></div>
            {{end}}
            <div class="dl-row"><div class="dl-term">Thread Dump</div><div class="dl-value">{{if .Task.Options.IncludeThreadDump}}Enabled{{else}}Disabled{{end}}</div></div>
            <div class="dl-row"><div class="dl-term">JVM Dump</div><div class="dl-value">{{if .Task.Options.IncludeJVMDump}}Enabled{{else}}Disabled{{end}}</div></div>
            <div class="dl-row"><div class="dl-term">Log Sample Lines</div><div class="dl-value">{{.Task.Options.LogSampleLines}}</div></div>
            <div class="dl-row"><div class="dl-term">Min Free Space for JVM Dump</div><div class="dl-value">{{.Task.Options.JVMDumpMinFreeMB}} MB</div></div>
          </div>
        </div>
      </div>

      <div class="detail-columns" style="margin-top: 18px;">
        <div class="detail-panel">
          <div class="panel-label">目标节点 / Selected Nodes</div>
          {{if .Task.SelectedNodes}}
          <div class="table-wrap" style="margin-top: 0;">
            <table>
              <thead>
                <tr>
                  <th>Host</th>
                  <th>Role</th>
                  <th>Cluster Node</th>
                  <th>Install Dir</th>
                </tr>
              </thead>
              <tbody>
                {{range .Task.SelectedNodes}}
                <tr>
                  <td>{{.HostLabel}}</td>
                  <td>{{.Role}}</td>
                  <td>{{.ClusterNode}}</td>
                  <td><code class="inline">{{.InstallDir}}</code></td>
                </tr>
                {{end}}
              </tbody>
            </table>
          </div>
          {{else}}
          <div class="empty">No selected nodes recorded.</div>
          {{end}}
        </div>

        <div class="detail-panel">
          <div class="panel-label">已确认正常 / Confirmed Normal</div>
          {{if .PassedChecks}}
          <div class="list">
            {{range .PassedChecks}}
            <div class="entry">
              <div class="entry-title">{{.Title}}</div>
              <div class="muted" style="margin-top: 6px;">{{.Details}}</div>
            </div>
            {{end}}
          </div>
          {{else}}
          <div class="empty">当前没有可展示的已通过项。 / No passed checks are available for this report.</div>
          {{end}}
        </div>
      </div>
    </section>

    <section class="section" id="execution">
      <div class="section-heading">
        <div>
          <h2>执行过程 / Task Execution</h2>
          <p class="section-lead">用于确认采集步骤是否完整执行，哪些步骤或节点失败，失败点在哪。</p>
        </div>
      </div>
      <div class="grid-2">
        <div class="detail-panel">
          <div class="panel-label">步骤状态 / Steps</div>
          {{if .TaskExecution.Steps}}
          <div class="table-wrap" style="margin-top: 0;">
            <table>
              <thead>
                <tr>
                  <th>#</th>
                  <th>Step</th>
                  <th>Status</th>
                  <th>Message</th>
                  <th>Time</th>
                </tr>
              </thead>
              <tbody>
                {{range .TaskExecution.Steps}}
                <tr>
                  <td>{{.Sequence}}</td>
                  <td><strong>{{.Title}}</strong><div class="muted small">{{.Code}}</div></td>
                  <td><span class="badge {{statusClass .Status}}">{{.Status}}</span></td>
                  <td>{{if ne .Error "-"}}{{.Error}}{{else}}{{.Message}}{{end}}</td>
                  <td>{{.StartedAt}} → {{.CompletedAt}}</td>
                </tr>
                {{end}}
              </tbody>
            </table>
          </div>
          {{else}}
          <div class="empty">No task steps recorded.</div>
          {{end}}
        </div>

        <div class="detail-panel">
          <div class="panel-label">节点执行 / Node Execution</div>
          {{if .TaskExecution.Nodes}}
          <div class="table-wrap" style="margin-top: 0;">
            <table>
              <thead>
                <tr>
                  <th>Host</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>Current Step</th>
                  <th>Message</th>
                </tr>
              </thead>
              <tbody>
                {{range .TaskExecution.Nodes}}
                <tr>
                  <td>{{.HostLabel}}</td>
                  <td>{{.Role}}</td>
                  <td><span class="badge {{statusClass .Status}}">{{.Status}}</span></td>
                  <td>{{.CurrentStep}}</td>
                  <td>{{if ne .Error "-"}}{{.Error}}{{else}}{{.Message}}{{end}}</td>
                </tr>
                {{end}}
              </tbody>
            </table>
          </div>
          {{else}}
          <div class="empty">No node executions recorded.</div>
          {{end}}
        </div>
      </div>
    </section>

    <section class="section" id="artifacts">
      <div class="section-heading">
        <div>
          <h2>诊断产物 / Diagnostic Artifacts</h2>
          <p class="section-lead">按类别展示已收集证据；需要深挖时可直接打开相对路径或查看预览。</p>
        </div>
      </div>
      {{if .ArtifactGroups}}
      {{range .ArtifactGroups}}
      <div class="artifact-group">
        <div class="section-heading" style="margin-bottom: 12px;">
          <div>
            <h3>{{.Label}}</h3>
            <p class="section-lead">{{len .Items}} artifact(s)</p>
          </div>
        </div>
        <div class="artifact-grid">
          {{range .Items}}
          <div class="artifact-card">
            <div class="entry-header" style="margin-bottom: 0;">
              <div>
                <div class="entry-title">{{.CategoryLabel}}</div>
                <div class="muted small">{{.StepCode}}</div>
              </div>
              <div style="display:flex; gap:8px; flex-wrap:wrap;">
                <span class="badge {{statusClass .Status}}">{{.Status}}</span>
                <span class="badge">{{.Format}}</span>
              </div>
            </div>
            <div class="artifact-meta">
              <div class="meta-item"><div class="label">Host</div><div class="value">{{.HostLabel}}</div></div>
              <div class="meta-item"><div class="label">Size</div><div class="value">{{.SizeLabel}}</div></div>
              <div class="meta-item"><div class="label">Relative Path</div><div class="value">{{if ne .RelativePath "-"}}<a href="{{.RelativePath}}"><code class="inline">{{.RelativePath}}</code></a>{{else}}-{{end}}</div></div>
              <div class="meta-item"><div class="label">Remote Path</div><div class="value">{{if ne .RemotePath "-"}}<code class="inline">{{.RemotePath}}</code>{{else}}-{{end}}</div></div>
            </div>
            <div>
              <div class="muted small">Message</div>
              <div>{{.Message}}</div>
            </div>
            {{if ne .LocalPath "-"}}
            <div>
              <div class="muted small">Local Path</div>
              <code class="inline">{{.LocalPath}}</code>
            </div>
            {{end}}
            <details>
              <summary>产物预览 / Artifact Preview</summary>
              {{if ne .Preview "-"}}<pre>{{.Preview}}</pre>{{else}}<div class="empty" style="margin-top: 12px;">暂无可展示预览。 / No preview available.</div>{{end}}
              {{if ne .PreviewNote "-"}}<div class="muted small" style="margin-top: 10px;">{{.PreviewNote}}</div>{{end}}
            </details>
          </div>
          {{end}}
        </div>
      </div>
      {{end}}
      {{else}}
      <div class="empty">No artifacts were registered in this bundle.</div>
      {{end}}
    </section>

    <section class="section" id="appendix">
      <div class="section-heading">
        <div>
          <h2>集群附录 / Cluster Appendix</h2>
          <p class="section-lead">用于补充部署元信息；通常在确认问题后再查阅即可。</p>
        </div>
      </div>
      {{if .Cluster}}
      <div class="detail-columns">
        <div class="detail-panel">
          <div class="panel-label">集群快照 / Cluster Snapshot</div>
          <div class="dl">
            <div class="dl-row"><div class="dl-term">Name</div><div class="dl-value">{{.Cluster.Name}}</div></div>
            <div class="dl-row"><div class="dl-term">Version</div><div class="dl-value">{{.Cluster.Version}}</div></div>
            <div class="dl-row"><div class="dl-term">Status</div><div class="dl-value"><span class="badge {{statusClass .Cluster.Status}}">{{.Cluster.Status}}</span></div></div>
            <div class="dl-row"><div class="dl-term">Deployment</div><div class="dl-value">{{.Cluster.DeploymentMode}}</div></div>
            <div class="dl-row"><div class="dl-term">Install Dir</div><div class="dl-value"><code class="inline">{{.Cluster.InstallDir}}</code></div></div>
            <div class="dl-row"><div class="dl-term">Node Count</div><div class="dl-value">{{.Cluster.NodeCount}}</div></div>
          </div>
        </div>
        <div class="detail-panel">
          <div class="panel-label">节点快照 / Nodes</div>
          <div class="table-wrap" style="margin-top: 0;">
            <table>
              <thead>
                <tr>
                  <th>Host ID</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>PID</th>
                  <th>Install Dir</th>
                </tr>
              </thead>
              <tbody>
                {{range .Cluster.Nodes}}
                <tr>
                  <td>#{{.HostID}}</td>
                  <td>{{.Role}}</td>
                  <td><span class="badge {{statusClass .Status}}">{{.Status}}</span></td>
                  <td>{{.ProcessPID}}</td>
                  <td><code class="inline">{{.InstallDir}}</code></td>
                </tr>
                {{end}}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      {{else}}
      <div class="empty">未采集到集群快照。 / Cluster snapshot was not collected.</div>
      {{end}}
    </section>
  </div>
</body>
</html>`
