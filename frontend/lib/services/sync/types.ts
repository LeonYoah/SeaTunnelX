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

export type SyncJSON = Record<string, unknown>;

export type SyncTaskStatus = 'draft' | 'published' | 'archived';
export type SyncTaskMode = 'streaming' | 'batch';
export type SyncNodeType = 'folder' | 'file';
export type SyncRunType = 'preview' | 'run' | 'recover';
export type SyncJobStatus =
  | 'pending'
  | 'running'
  | 'success'
  | 'failed'
  | 'canceled';
export type SyncFormat = 'hocon' | 'json';

export interface SyncTask {
  id: number;
  parent_id?: number;
  node_type: SyncNodeType;
  name: string;
  description: string;
  cluster_id: number;
  engine_version: string;
  mode: SyncTaskMode;
  status: SyncTaskStatus;
  content_format: SyncFormat;
  content: string;
  job_name: string;
  definition: SyncJSON;
  sort_order: number;
  current_version: number;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface SyncTaskTreeNode {
  id: number;
  parent_id?: number;
  node_type: SyncNodeType;
  name: string;
  description: string;
  cluster_id: number;
  engine_version: string;
  mode: SyncTaskMode;
  status: SyncTaskStatus;
  content_format: SyncFormat;
  content: string;
  job_name: string;
  definition: SyncJSON;
  sort_order: number;
  current_version: number;
  children?: SyncTaskTreeNode[];
}

export interface SyncTaskVersion {
  id: number;
  task_id: number;
  version: number;
  name_snapshot: string;
  description_snapshot: string;
  cluster_id_snapshot: number;
  engine_version_snapshot: string;
  mode_snapshot: SyncTaskMode;
  content_format_snapshot: SyncFormat;
  content_snapshot: string;
  job_name_snapshot: string;
  definition_snapshot: SyncJSON;
  comment: string;
  created_by: number;
  created_at: string;
}

export interface SyncGlobalVariable {
  id: number;
  key: string;
  value: string;
  description: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface SyncJobLogsResult {
  mode: string;
  source: string;
  logs: string;
  next_offset?: string;
  file_size?: number;
  updated_at: string;
}

export interface SyncPreviewDataset {
  name: string;
  catalog?: SyncJSON;
  columns?: string[];
  rows?: SyncJSON[];
  page?: number;
  page_size?: number;
  total?: number;
  updated_at?: string;
}

export interface SyncJobInstance {
  id: number;
  task_id: number;
  task_version: number;
  run_type: SyncRunType;
  platform_job_id: string;
  engine_job_id: string;
  recovered_from_instance_id?: number;
  status: SyncJobStatus;
  submit_spec: SyncJSON;
  result_preview: SyncJSON;
  error_message: string;
  started_at?: string;
  finished_at?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface SyncTaskListData {
  total: number;
  items: SyncTask[];
}

export interface SyncTaskTreeData {
  items: SyncTaskTreeNode[];
}

export interface SyncTaskVersionListData {
  total: number;
  items: SyncTaskVersion[];
}

export interface SyncJobListData {
  total: number;
  items: SyncJobInstance[];
}

export interface SyncGlobalVariableListData {
  total: number;
  items: SyncGlobalVariable[];
}

export interface SyncValidateResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
  summary: string;
  resolved?: Record<string, string>;
  detected_vars?: string[];
  detected_files?: string[];
  checks?: SyncValidationCheck[];
}

export interface SyncValidationCheck {
  node_id: string;
  kind: string;
  connector_type: string;
  target?: string;
  status: string;
  message: string;
}

export interface SyncDagResult {
  nodes: SyncJSON[];
  edges: SyncJSON[];
  warnings?: string[];
  simple_graph?: boolean;
  webui_job?: SyncWebUIDagPreviewJob;
}

export interface SyncWebUIDagEdge {
  inputVertexId: number;
  targetVertexId: number;
}

export interface SyncWebUIDagVertexInfo {
  vertexId: number;
  type: string;
  connectorType: string;
  tablePaths?: string[];
}

export interface SyncWebUIJobDag {
  jobId: string;
  pipelineEdges: Record<string, SyncWebUIDagEdge[]>;
  vertexInfoMap: Record<string, SyncWebUIDagVertexInfo>;
  envOptions?: SyncJSON;
}

export interface SyncWebUIDagPreviewJob {
  jobId: string;
  jobName: string;
  jobStatus: string;
  errorMsg?: string;
  createTime?: string;
  finishTime?: string;
  jobDag: SyncWebUIJobDag;
  metrics?: SyncJSON;
  pluginJarsUrls?: string[];
  simpleGraph?: boolean;
  warnings?: string[];
}

export interface CreateSyncTaskRequest {
  parent_id?: number;
  node_type?: SyncNodeType;
  name: string;
  description?: string;
  cluster_id?: number;
  engine_version?: string;
  mode?: SyncTaskMode;
  content_format?: SyncFormat;
  content?: string;
  job_name?: string;
  sort_order?: number;
  definition?: SyncJSON;
}

export interface UpdateSyncTaskRequest extends CreateSyncTaskRequest {}

export interface PublishSyncTaskRequest {
  comment?: string;
}

export interface CreateSyncGlobalVariableRequest {
  key: string;
  value?: string;
  description?: string;
}

export interface UpdateSyncGlobalVariableRequest extends CreateSyncGlobalVariableRequest {}
