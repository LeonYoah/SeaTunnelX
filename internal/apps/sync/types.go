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

// CreateTaskRequest represents the payload for creating a sync workspace node.
type CreateTaskRequest struct {
	ParentID      *uint   `json:"parent_id"`
	NodeType      string  `json:"node_type"`
	Name          string  `json:"name" binding:"required"`
	Description   string  `json:"description"`
	ClusterID     uint    `json:"cluster_id"`
	EngineVersion string  `json:"engine_version"`
	Mode          string  `json:"mode"`
	ContentFormat string  `json:"content_format"`
	Content       string  `json:"content"`
	JobName       string  `json:"job_name"`
	SortOrder     int     `json:"sort_order"`
	Definition    JSONMap `json:"definition"`
}

// UpdateTaskRequest represents the payload for updating a sync workspace node.
type UpdateTaskRequest struct {
	ParentID      *uint   `json:"parent_id"`
	NodeType      string  `json:"node_type"`
	Name          string  `json:"name" binding:"required"`
	Description   string  `json:"description"`
	ClusterID     uint    `json:"cluster_id"`
	EngineVersion string  `json:"engine_version"`
	Mode          string  `json:"mode"`
	ContentFormat string  `json:"content_format"`
	Content       string  `json:"content"`
	JobName       string  `json:"job_name"`
	SortOrder     int     `json:"sort_order"`
	Definition    JSONMap `json:"definition"`
}

// PublishTaskRequest represents the payload for publishing a sync task.
type PublishTaskRequest struct {
	Comment string `json:"comment"`
}

// CreateGlobalVariableRequest represents one global variable payload.
type CreateGlobalVariableRequest struct {
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// UpdateGlobalVariableRequest represents one global variable update payload.
type UpdateGlobalVariableRequest struct {
	Key         string `json:"key" binding:"required"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// RecoverTaskRequest represents the payload for recovering from one previous job.
type RecoverTaskRequest struct {
	Comment string `json:"comment,omitempty"`
}

// CancelJobRequest represents stop-job behavior.
type CancelJobRequest struct {
	StopWithSavepoint bool `json:"stop_with_savepoint"`
}

// TaskFilter represents task list query filters.
type TaskFilter struct {
	Name   string
	Status TaskStatus
	Page   int
	Size   int
}

// JobFilter represents job instance list query filters.
type JobFilter struct {
	TaskID  uint
	RunType RunType
	Page    int
	Size    int
}

// ValidateResult represents validation result payload.
type ValidateResult struct {
	Valid         bool              `json:"valid"`
	Errors        []string          `json:"errors"`
	Warnings      []string          `json:"warnings"`
	Summary       string            `json:"summary"`
	Resolved      map[string]string `json:"resolved,omitempty"`
	DetectedVars  []string          `json:"detected_vars,omitempty"`
	DetectedFiles []string          `json:"detected_files,omitempty"`
	Checks        []ValidateCheck   `json:"checks,omitempty"`
}

// ValidateCheck represents one connector config/connection validation entry.
type ValidateCheck struct {
	NodeID        string `json:"node_id"`
	Kind          string `json:"kind"`
	ConnectorType string `json:"connector_type"`
	Target        string `json:"target,omitempty"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

// DAGResult represents DAG response payload.
type DAGResult struct {
	Nodes       []JSONMap `json:"nodes"`
	Edges       []JSONMap `json:"edges"`
	WebUIJob    JSONMap   `json:"webui_job,omitempty"`
	SimpleGraph bool      `json:"simple_graph,omitempty"`
	Warnings    []string  `json:"warnings,omitempty"`
}

// TaskListData represents task list response data.
type TaskListData struct {
	Total int64   `json:"total"`
	Items []*Task `json:"items"`
}

// TaskVersionListData represents paginated task versions.
type TaskVersionListData struct {
	Total int64          `json:"total"`
	Items []*TaskVersion `json:"items"`
}

// JobListData represents job list response data.
type JobListData struct {
	Total int64          `json:"total"`
	Items []*JobInstance `json:"items"`
}

// GlobalVariableListData represents paginated global variables.
type GlobalVariableListData struct {
	Total int64             `json:"total"`
	Items []*GlobalVariable `json:"items"`
}

// TaskTreeNode represents one node in the sync workspace tree.
type TaskTreeNode struct {
	ID             uint            `json:"id"`
	ParentID       *uint           `json:"parent_id,omitempty"`
	NodeType       TaskNodeType    `json:"node_type"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	ClusterID      uint            `json:"cluster_id"`
	EngineVersion  string          `json:"engine_version"`
	Mode           TaskMode        `json:"mode"`
	Status         TaskStatus      `json:"status"`
	ContentFormat  ContentFormat   `json:"content_format"`
	Content        string          `json:"content"`
	JobName        string          `json:"job_name"`
	Definition     JSONMap         `json:"definition"`
	SortOrder      int             `json:"sort_order"`
	CurrentVersion int             `json:"current_version"`
	Children       []*TaskTreeNode `json:"children,omitempty"`
}

// TaskTreeData represents tree response payload.
type TaskTreeData struct {
	Items []*TaskTreeNode `json:"items"`
}
