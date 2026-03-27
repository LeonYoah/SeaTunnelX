package sync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	clusterapp "github.com/seatunnel/seatunnelX/internal/apps/cluster"
	hostapp "github.com/seatunnel/seatunnelX/internal/apps/host"
)

type AgentCommandSender interface {
	SendCommand(ctx context.Context, agentID string, commandType string, params map[string]string) (success bool, output string, err error)
}

type ClusterLogProvider interface {
	GetNodeLogs(ctx context.Context, clusterID uint, nodeID uint, req *clusterapp.GetNodeLogsRequest) (string, error)
}

type ExecutionTargetResolver interface {
	ResolveExecutionTarget(ctx context.Context, clusterID uint, definition JSONMap) (*ExecutionTarget, error)
	ResolveExecutionTargets(ctx context.Context, clusterID uint, definition JSONMap) ([]*ExecutionTarget, error)
}

type ExecutionTarget struct {
	ClusterID     uint
	NodeID        uint
	HostID        uint
	AgentID       string
	InstallDir    string
	Role          string
	HostIP        string
	APIPort       int
	HazelcastPort int
}

type DefaultExecutionTargetResolver struct {
	clusterRepo *clusterapp.Repository
	hostRepo    *hostapp.Repository
}

func NewDefaultExecutionTargetResolver(clusterRepo *clusterapp.Repository, hostRepo *hostapp.Repository) *DefaultExecutionTargetResolver {
	return &DefaultExecutionTargetResolver{clusterRepo: clusterRepo, hostRepo: hostRepo}
}

func (r *DefaultExecutionTargetResolver) ResolveExecutionTarget(ctx context.Context, clusterID uint, definition JSONMap) (*ExecutionTarget, error) {
	targets, err := r.ResolveExecutionTargets(ctx, clusterID, definition)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, ErrExecutionTargetUnavailable
	}
	return targets[0], nil
}

func (r *DefaultExecutionTargetResolver) ResolveExecutionTargets(ctx context.Context, clusterID uint, definition JSONMap) ([]*ExecutionTarget, error) {
	if r == nil || r.clusterRepo == nil || r.hostRepo == nil {
		return nil, ErrExecutionTargetUnavailable
	}
	if clusterID == 0 {
		return nil, ErrLocalClusterRequired
	}
	if nodeID, ok := intValue(definition, "local_node_id", "node_id"); ok && nodeID > 0 {
		node, err := r.clusterRepo.GetNodeByID(ctx, uint(nodeID))
		if err == nil {
			target, buildErr := r.buildTarget(ctx, node)
			if buildErr != nil {
				return nil, buildErr
			}
			return []*ExecutionTarget{target}, nil
		}
	}
	nodes, err := r.clusterRepo.GetNodesByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrExecutionTargetUnavailable
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return targetNodePriority(nodes[i]) < targetNodePriority(nodes[j])
	})
	targets := make([]*ExecutionTarget, 0, len(nodes))
	for _, node := range nodes {
		target, buildErr := r.buildTarget(ctx, node)
		if buildErr == nil {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return nil, ErrExecutionTargetUnavailable
	}
	return targets, nil
}

func targetNodePriority(node *clusterapp.ClusterNode) int {
	if node == nil {
		return 99
	}
	switch node.Role {
	case clusterapp.NodeRoleMasterWorker:
		return 0
	case clusterapp.NodeRoleMaster:
		return 1
	case clusterapp.NodeRoleWorker:
		return 2
	default:
		return 3
	}
}

func (r *DefaultExecutionTargetResolver) buildTarget(ctx context.Context, node *clusterapp.ClusterNode) (*ExecutionTarget, error) {
	if node == nil {
		return nil, ErrExecutionTargetUnavailable
	}
	hostObj, err := r.hostRepo.GetByID(ctx, node.HostID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(hostObj.AgentID) == "" {
		return nil, ErrExecutionTargetUnavailable
	}
	installDir := strings.TrimSpace(node.InstallDir)
	if installDir == "" {
		installDir = "/opt/seatunnel"
	}
	return &ExecutionTarget{
		ClusterID:     node.ClusterID,
		NodeID:        node.ID,
		HostID:        node.HostID,
		AgentID:       strings.TrimSpace(hostObj.AgentID),
		InstallDir:    installDir,
		Role:          string(node.Role),
		HostIP:        strings.TrimSpace(hostObj.IPAddress),
		APIPort:       node.APIPort,
		HazelcastPort: node.HazelcastPort,
	}, nil
}

type LocalRunResponse struct {
	PID        int    `json:"pid"`
	ConfigFile string `json:"config_file"`
	LogFile    string `json:"log_file,omitempty"`
	StatusFile string `json:"status_file,omitempty"`
}

type LocalStatusResponse struct {
	Running    bool   `json:"running"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Message    string `json:"message"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type precheckJSONEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type JobLogsResult struct {
	Mode       string `json:"mode"`
	Source     string `json:"source"`
	Logs       string `json:"logs"`
	NextOffset string `json:"next_offset,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

type clusterJobLogPayload struct {
	Logs       string `json:"logs"`
	Path       string `json:"path,omitempty"`
	NextOffset string `json:"next_offset,omitempty"`
	FileSize   int64  `json:"file_size,omitempty"`
}

type PreviewCollectRequest struct {
	PlatformJobID string                   `json:"platform_job_id"`
	EngineJobID   string                   `json:"engine_job_id"`
	Dataset       string                   `json:"dataset"`
	Catalog       map[string]interface{}   `json:"catalog"`
	Columns       []interface{}            `json:"columns"`
	Rows          []map[string]interface{} `json:"rows"`
	Page          int                      `json:"page"`
	PageSize      int                      `json:"page_size"`
	Total         int                      `json:"total"`
	Replace       bool                     `json:"replace"`
}

func (s *Service) SetAgentCommandSender(sender AgentCommandSender) { s.agentSender = sender }
func (s *Service) SetExecutionTargetResolver(resolver ExecutionTargetResolver) {
	s.executionTargetResolver = resolver
}
func (s *Service) SetClusterLogProvider(provider ClusterLogProvider) { s.clusterLogProvider = provider }

func taskExecutionMode(task *Task) string {
	if task == nil {
		return "cluster"
	}
	mode := strings.ToLower(strings.TrimSpace(stringValue(task.Definition, "execution_mode")))
	if mode == "local" {
		return "local"
	}
	return "cluster"
}

func submitSpecExecutionMode(spec JSONMap) string {
	mode := strings.ToLower(strings.TrimSpace(stringValue(spec, "execution_mode")))
	if mode == "local" {
		return "local"
	}
	return "cluster"
}

func (s *Service) submitLocalTaskInstance(ctx context.Context, task *Task, createdBy uint, runType RunType, platformJobID string, body []byte, format, jobName string) (*JobInstance, error) {
	if s.agentSender == nil || s.executionTargetResolver == nil {
		return nil, ErrLocalExecutionUnavailable
	}
	target, err := s.executionTargetResolver.ResolveExecutionTarget(ctx, task.ClusterID, task.Definition)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"sub_command":     "sync_local_run",
		"install_dir":     target.InstallDir,
		"cluster_id":      strconv.FormatUint(uint64(target.ClusterID), 10),
		"node_id":         strconv.FormatUint(uint64(target.NodeID), 10),
		"host_id":         strconv.FormatUint(uint64(target.HostID), 10),
		"platform_job_id": platformJobID,
		"job_name":        jobName,
		"content":         string(body),
		"content_format":  normalizeSubmitFormat(format),
	}
	success, output, err := s.agentSender.SendCommand(ctx, target.AgentID, "sync_local_run", params)
	if err != nil {
		return nil, err
	}
	if !success {
		if isLocalSyncCommandUnsupported(output) {
			return nil, fmt.Errorf("sync: local execution requires an upgraded agent that supports sync_local_run")
		}
		return nil, fmt.Errorf("sync: local run failed: %s", strings.TrimSpace(output))
	}
	decodedOutput := unwrapPrecheckPayload(output)
	var localRun LocalRunResponse
	if err := json.Unmarshal([]byte(decodedOutput), &localRun); err != nil {
		return nil, err
	}
	now := time.Now()
	instance := &JobInstance{
		TaskID:        task.ID,
		TaskVersion:   task.CurrentVersion,
		RunType:       runType,
		PlatformJobID: platformJobID,
		EngineJobID:   platformJobID,
		Status:        JobStatusRunning,
		SubmitSpec: JSONMap{
			"execution_mode":  "local",
			"cluster_id":      task.ClusterID,
			"target_node_id":  target.NodeID,
			"target_host_id":  target.HostID,
			"target_agent_id": target.AgentID,
			"install_dir":     target.InstallDir,
			"format":          normalizeSubmitFormat(format),
			"job_name":        jobName,
			"platform_job_id": platformJobID,
			"config_file":     localRun.ConfigFile,
			"pid":             localRun.PID,
		},
		ResultPreview: JSONMap{
			"job_status":      "RUNNING",
			"submission_mode": "local",
		},
		StartedAt: &now,
		CreatedBy: createdBy,
	}
	if err := s.repo.CreateJobInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

func (s *Service) refreshLocalJob(ctx context.Context, instance *JobInstance) (*JobInstance, error) {
	if instance == nil || s.agentSender == nil {
		return instance, nil
	}
	agentID := strings.TrimSpace(stringValue(instance.SubmitSpec, "target_agent_id"))
	if agentID == "" {
		return instance, nil
	}
	params := map[string]string{
		"sub_command":     "sync_local_status",
		"pid":             strconv.Itoa(intValueOrZero(instance.SubmitSpec, "pid")),
		"platform_job_id": strings.TrimSpace(instance.PlatformJobID),
		"install_dir":     strings.TrimSpace(stringValue(instance.SubmitSpec, "install_dir")),
	}
	success, output, err := s.agentSender.SendCommand(ctx, agentID, "sync_local_status", params)
	if err != nil || !success {
		return instance, nil
	}
	decodedOutput := unwrapPrecheckPayload(output)
	var status LocalStatusResponse
	if err := json.Unmarshal([]byte(decodedOutput), &status); err != nil {
		return instance, nil
	}
	switch strings.ToLower(strings.TrimSpace(status.Status)) {
	case "success":
		instance.Status = JobStatusSuccess
	case "failed":
		instance.Status = JobStatusFailed
	case "canceled", "cancelled":
		instance.Status = JobStatusCanceled
	case "running":
		instance.Status = JobStatusRunning
	default:
		if status.Running {
			instance.Status = JobStatusRunning
		}
	}
	if instance.ResultPreview == nil {
		instance.ResultPreview = JSONMap{}
	}
	instance.ResultPreview["job_status"] = strings.ToUpper(strings.TrimSpace(status.Status))
	instance.ResultPreview["exit_code"] = status.ExitCode
	if strings.TrimSpace(status.Message) != "" {
		instance.ResultPreview["status_message"] = strings.TrimSpace(status.Message)
	}
	if (instance.Status == JobStatusSuccess || instance.Status == JobStatusFailed || instance.Status == JobStatusCanceled) && instance.FinishedAt == nil {
		now := time.Now()
		instance.FinishedAt = &now
		if strings.TrimSpace(status.Message) != "" && instance.Status == JobStatusFailed {
			instance.ErrorMessage = strings.TrimSpace(status.Message)
		}
	}
	if err := s.repo.UpdateJobInstance(ctx, instance); err != nil {
		return nil, err
	}
	return instance, nil
}

func (s *Service) stopLocalJob(ctx context.Context, instance *JobInstance) error {
	if instance == nil || s.agentSender == nil {
		return ErrLocalExecutionUnavailable
	}
	agentID := strings.TrimSpace(stringValue(instance.SubmitSpec, "target_agent_id"))
	if agentID == "" {
		return ErrLocalExecutionUnavailable
	}
	params := map[string]string{
		"sub_command":     "sync_local_stop",
		"pid":             strconv.Itoa(intValueOrZero(instance.SubmitSpec, "pid")),
		"platform_job_id": strings.TrimSpace(instance.PlatformJobID),
		"install_dir":     strings.TrimSpace(stringValue(instance.SubmitSpec, "install_dir")),
	}
	success, output, err := s.agentSender.SendCommand(ctx, agentID, "sync_local_stop", params)
	if err != nil {
		return err
	}
	if !success {
		if isLocalSyncCommandUnsupported(output) {
			return ErrLocalExecutionUnavailable
		}
		return fmt.Errorf("sync: local stop failed: %s", strings.TrimSpace(output))
	}
	return nil
}

func (s *Service) GetJobLogs(ctx context.Context, id uint, offset string, limitBytes int, keyword string, level string) (*JobLogsResult, error) {
	instance, err := s.repo.GetJobInstanceByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if limitBytes < 0 {
		limitBytes = 0
	}
	if submitSpecExecutionMode(instance.SubmitSpec) == "local" {
		return s.getLocalJobLogs(ctx, instance, offset, limitBytes, keyword, level)
	}
	return s.getClusterJobLogs(ctx, instance, offset, limitBytes, keyword, level)
}

func (s *Service) getLocalJobLogs(ctx context.Context, instance *JobInstance, offset string, limitBytes int, keyword string, level string) (*JobLogsResult, error) {
	if s.agentSender == nil {
		return nil, ErrLocalExecutionUnavailable
	}
	agentID := strings.TrimSpace(stringValue(instance.SubmitSpec, "target_agent_id"))
	platformJobID := strings.TrimSpace(instance.PlatformJobID)
	if agentID == "" || platformJobID == "" {
		return nil, ErrJobLogsUnavailable
	}
	success, output, err := s.agentSender.SendCommand(ctx, agentID, "sync_local_logs", map[string]string{
		"sub_command":     "sync_local_logs",
		"platform_job_id": platformJobID,
		"keyword":         strings.TrimSpace(keyword),
		"level":           strings.TrimSpace(level),
		"install_dir":     strings.TrimSpace(stringValue(instance.SubmitSpec, "install_dir")),
		"offset":          strings.TrimSpace(offset),
		"limit_bytes":     strconv.Itoa(limitBytes),
	})
	if err != nil {
		return nil, err
	}
	if !success {
		if isLocalSyncCommandUnsupported(output) {
			return nil, ErrLocalExecutionUnavailable
		}
		return nil, fmt.Errorf("sync: get local logs failed: %s", strings.TrimSpace(output))
	}
	decodedOutput := unwrapPrecheckPayload(output)
	var payload struct {
		Logs       string `json:"logs"`
		NextOffset string `json:"next_offset"`
		FileSize   int64  `json:"file_size"`
	}
	if err := json.Unmarshal([]byte(decodedOutput), &payload); err != nil {
		return nil, err
	}
	return &JobLogsResult{Mode: "local", Source: "agent-file", Logs: payload.Logs, NextOffset: payload.NextOffset, FileSize: payload.FileSize, UpdatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Service) getClusterJobLogs(ctx context.Context, instance *JobInstance, offset string, limitBytes int, keyword string, level string) (*JobLogsResult, error) {
	if s.agentSender == nil || strings.TrimSpace(instance.EngineJobID) == "" {
		return nil, ErrJobLogsUnavailable
	}
	targets := s.resolveClusterLogTargets(ctx, instance)
	if len(targets) == 0 {
		return nil, ErrJobLogsUnavailable
	}
	logs, nextOffset, fileSize, found, err := s.readClusterLogsFromTargets(ctx, instance, targets, offset, limitBytes, keyword, level)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrJobLogsUnavailable
	}
	return &JobLogsResult{Mode: "cluster", Source: "agent-file", Logs: logs, NextOffset: nextOffset, FileSize: fileSize, UpdatedAt: time.Now().Format(time.RFC3339)}, nil
}

func (s *Service) resolveClusterLogTargets(ctx context.Context, instance *JobInstance) []*ExecutionTarget {
	targets := make([]*ExecutionTarget, 0, 4)
	seen := make(map[string]struct{})
	appendTarget := func(target *ExecutionTarget) {
		if target == nil {
			return
		}
		agentID := strings.TrimSpace(target.AgentID)
		installDir := strings.TrimSpace(target.InstallDir)
		if agentID == "" || installDir == "" {
			return
		}
		hostKey := strings.TrimSpace(target.HostIP)
		if target.HostID > 0 {
			hostKey = fmt.Sprintf("host:%d", target.HostID)
		}
		if hostKey == "" {
			hostKey = "agent:" + agentID
		}
		key := hostKey + "|" + installDir
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		targets = append(targets, target)
	}
	appendTarget(&ExecutionTarget{
		AgentID:    strings.TrimSpace(stringValue(instance.SubmitSpec, "target_agent_id")),
		InstallDir: strings.TrimSpace(stringValue(instance.SubmitSpec, "install_dir")),
	})
	if s.executionTargetResolver == nil {
		return targets
	}
	clusterID := uintValue(instance.SubmitSpec, "cluster_id")
	resolved, err := s.executionTargetResolver.ResolveExecutionTargets(ctx, clusterID, nil)
	if err != nil {
		return targets
	}
	for _, target := range resolved {
		appendTarget(target)
	}
	return targets
}

func (s *Service) readClusterLogsFromTargets(ctx context.Context, instance *JobInstance, targets []*ExecutionTarget, offset string, limitBytes int, keyword string, level string) (string, string, int64, bool, error) {
	chunks := make([]string, 0, len(targets))
	found := false
	cursor := decodeClusterLogOffset(offset)
	nextCursor := make(map[string]int64)
	var totalFileSize int64
	for _, target := range targets {
		targetKey := buildClusterLogTargetKey(target)
		success, output, err := s.agentSender.SendCommand(ctx, target.AgentID, "sync_job_logs", map[string]string{
			"sub_command":     "sync_job_logs",
			"platform_job_id": strings.TrimSpace(instance.PlatformJobID),
			"engine_job_id":   strings.TrimSpace(instance.EngineJobID),
			"keyword":         strings.TrimSpace(keyword),
			"level":           strings.TrimSpace(level),
			"install_dir":     target.InstallDir,
			"offset":          strconv.FormatInt(cursor[targetKey], 10),
			"limit_bytes":     strconv.Itoa(limitBytes),
		})
		if err != nil || !success {
			continue
		}
		decodedOutput := unwrapPrecheckPayload(output)
		var payload clusterJobLogPayload
		if jsonErr := json.Unmarshal([]byte(decodedOutput), &payload); jsonErr != nil {
			continue
		}
		found = true
		if payload.NextOffset != "" {
			nextCursor[targetKey] = parseInt64OrZero(payload.NextOffset)
		}
		totalFileSize += payload.FileSize
		chunks = append(chunks, payload.Logs)
	}
	if !found {
		return "", "", 0, false, ErrJobLogsUnavailable
	}
	return mergeLogChunks(chunks), encodeClusterLogOffset(nextCursor), totalFileSize, true, nil
}

func buildClusterLogTargetKey(target *ExecutionTarget) string {
	hostKey := strings.TrimSpace(target.HostIP)
	if target.HostID > 0 {
		hostKey = fmt.Sprintf("host:%d", target.HostID)
	}
	if hostKey == "" {
		hostKey = "agent:" + strings.TrimSpace(target.AgentID)
	}
	return hostKey + "|" + strings.TrimSpace(target.InstallDir)
}

func decodeClusterLogOffset(raw string) map[string]int64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]int64{}
	}
	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return map[string]int64{}
	}
	result := make(map[string]int64)
	_ = json.Unmarshal(decoded, &result)
	return result
}

func encodeClusterLogOffset(cursor map[string]int64) string {
	if len(cursor) == 0 {
		return ""
	}
	body, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(body)
}

func parseInt64OrZero(raw string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return value
}

func mergeLogChunks(chunks []string) string {
	merged := make([]string, 0, 256)
	for _, chunk := range chunks {
		for _, line := range strings.Split(strings.TrimSpace(chunk), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			merged = append(merged, trimmed)
		}
	}
	if len(merged) == 0 {
		return ""
	}
	return strings.Join(merged, "\n")
}

func isLocalSyncCommandUnsupported(output string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(output)), "unknown precheck sub-command")
}

func unwrapPrecheckPayload(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return trimmed
	}
	var envelope precheckJSONEnvelope
	if err := json.Unmarshal([]byte(trimmed), &envelope); err == nil {
		message := strings.TrimSpace(envelope.Message)
		if message != "" {
			return message
		}
	}
	return trimmed
}

func (s *Service) CollectPreview(ctx context.Context, req *PreviewCollectRequest) error {
	if req == nil {
		return ErrPreviewPayloadInvalid
	}
	instance, err := s.repo.GetPreviewJobInstanceByPlatformOrEngineJobID(ctx, strings.TrimSpace(req.PlatformJobID), strings.TrimSpace(req.EngineJobID))
	if err != nil {
		return err
	}
	if instance.ResultPreview == nil {
		instance.ResultPreview = JSONMap{}
	}
	datasetName := strings.TrimSpace(req.Dataset)
	if datasetName == "" {
		datasetName = "preview_dataset"
	}
	datasets := toDatasetSlice(instance.ResultPreview["datasets"])
	dataset := JSONMap{
		"name":       datasetName,
		"catalog":    cloneAnyMap(req.Catalog),
		"columns":    interfaceSliceToStrings(req.Columns),
		"rows":       req.Rows,
		"page":       normalizePositive(req.Page, 1),
		"page_size":  normalizePositive(req.PageSize, len(req.Rows)),
		"total":      normalizePositive(req.Total, len(req.Rows)),
		"updated_at": time.Now().Format(time.RFC3339),
	}
	replaced := false
	for idx, item := range datasets {
		if strings.EqualFold(strings.TrimSpace(stringValue(item, "name")), datasetName) {
			if !req.Replace {
				existingRows := mapRowsValue(item["rows"])
				dataset["rows"] = append(existingRows, req.Rows...)
				dataset["page"] = 1
				dataset["page_size"] = len(mapRowsValue(dataset["rows"]))
				dataset["total"] = len(mapRowsValue(dataset["rows"]))
			}
			datasets[idx] = dataset
			replaced = true
			break
		}
	}
	if !replaced {
		datasets = append(datasets, dataset)
	}
	instance.ResultPreview["datasets"] = datasets
	instance.ResultPreview["columns"] = interfaceSliceToStrings(req.Columns)
	instance.ResultPreview["rows"] = req.Rows
	if err := s.repo.UpdateJobInstance(ctx, instance); err != nil {
		return err
	}
	return nil
}

func toDatasetSlice(value interface{}) []JSONMap {
	raw, ok := value.([]interface{})
	if !ok {
		if typed, ok := value.([]JSONMap); ok {
			return typed
		}
		return []JSONMap{}
	}
	result := make([]JSONMap, 0, len(raw))
	for _, item := range raw {
		if mapped, ok := item.(map[string]interface{}); ok {
			result = append(result, JSONMap(mapped))
		}
	}
	return result
}

func cloneAnyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func interfaceSliceToStrings(items []interface{}) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				result = append(result, strings.TrimSpace(value))
			}
		case map[string]interface{}:
			if name := strings.TrimSpace(stringValue(JSONMap(value), "name", "field", "column")); name != "" {
				result = append(result, name)
			}
		}
	}
	return result
}

func mapRowsValue(value interface{}) []map[string]interface{} {
	raw, ok := value.([]interface{})
	if !ok {
		if typed, ok := value.([]map[string]interface{}); ok {
			return typed
		}
		return []map[string]interface{}{}
	}
	result := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if mapped, ok := item.(map[string]interface{}); ok {
			result = append(result, mapped)
		}
	}
	return result
}

func intValueOrZero(src JSONMap, keys ...string) int {
	value, ok := intValue(src, keys...)
	if !ok {
		return 0
	}
	return value
}

func uintValue(src JSONMap, keys ...string) uint {
	value, ok := intValue(src, keys...)
	if !ok || value <= 0 {
		return 0
	}
	return uint(value)
}

func normalizePositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func mergePreviewDeriveMetadata(existing JSONMap, previewResult *ConfigToolPreviewResponse) JSONMap {
	if existing == nil {
		existing = JSONMap{}
	}
	if previewResult == nil {
		return existing
	}
	if previewResult.ContentFormat != "" {
		existing["content_format"] = previewResult.ContentFormat
	}
	if len(previewResult.Warnings) > 0 {
		existing["warnings"] = previewResult.Warnings
	}
	if len(previewResult.Graph.Nodes) > 0 {
		existing["graph"] = map[string]interface{}{"nodes": previewResult.Graph.Nodes, "edges": previewResult.Graph.Edges}
	}
	return existing
}
