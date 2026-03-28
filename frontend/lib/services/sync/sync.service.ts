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

import {BaseService} from '../core/base.service';
import apiClient from '../core/api-client';
import type {ApiResponse} from '../core/types';
import type {
  CreateSyncGlobalVariableRequest,
  CreateSyncTaskRequest,
  PreviewSyncTaskRequest,
  PublishSyncTaskRequest,
  SyncDagResult,
  SyncGlobalVariable,
  SyncGlobalVariableListData,
  SyncJobInstance,
  SyncJobListData,
  SyncJobLogsResult,
  SyncCheckpointSnapshot,
  SyncPreviewSnapshot,
  SyncTask,
  SyncTaskListData,
  SyncTaskTreeData,
  SyncTaskVersion,
  SyncTaskVersionListData,
  SyncValidateResult,
  UpdateSyncGlobalVariableRequest,
  UpdateSyncTaskRequest,
} from './types';

export class SyncService extends BaseService {
  protected static readonly basePath = '/sync';

  static async getTree(): Promise<SyncTaskTreeData> {
    return this.get<SyncTaskTreeData>('/tree');
  }

  static async listTasks(params?: {
    current?: number;
    size?: number;
    status?: string;
    name?: string;
  }): Promise<SyncTaskListData> {
    return this.get<SyncTaskListData>('/tasks', params);
  }

  static async getTask(taskId: number): Promise<SyncTask> {
    return this.get<SyncTask>(`/tasks/${taskId}`);
  }

  static async createTask(request: CreateSyncTaskRequest): Promise<SyncTask> {
    return this.post<SyncTask>('/tasks', request);
  }

  static async updateTask(
    taskId: number,
    request: UpdateSyncTaskRequest,
  ): Promise<SyncTask> {
    return this.put<SyncTask>(`/tasks/${taskId}`, request);
  }

  static async deleteTask(taskId: number): Promise<{deleted: boolean}> {
    return this.delete<{deleted: boolean}>(`/tasks/${taskId}`);
  }

  static async listGlobalVariables(params?: {
    current?: number;
    size?: number;
  }): Promise<SyncGlobalVariableListData> {
    return this.get<SyncGlobalVariableListData>('/global-variables', params);
  }

  static async createGlobalVariable(
    request: CreateSyncGlobalVariableRequest,
  ): Promise<SyncGlobalVariable> {
    return this.post<SyncGlobalVariable>('/global-variables', request);
  }

  static async updateGlobalVariable(
    id: number,
    request: UpdateSyncGlobalVariableRequest,
  ): Promise<SyncGlobalVariable> {
    return this.put<SyncGlobalVariable>(`/global-variables/${id}`, request);
  }

  static async deleteGlobalVariable(id: number): Promise<{deleted: boolean}> {
    return this.delete<{deleted: boolean}>(`/global-variables/${id}`);
  }

  static async publishTask(
    taskId: number,
    request?: PublishSyncTaskRequest,
  ): Promise<SyncTaskVersion> {
    return this.post<SyncTaskVersion>(
      `/tasks/${taskId}/publish`,
      request || {},
    );
  }

  static async listVersions(
    taskId: number,
    params?: {
      current?: number;
      size?: number;
    },
  ): Promise<SyncTaskVersionListData> {
    return this.get<SyncTaskVersionListData>(
      `/tasks/${taskId}/versions`,
      params,
    );
  }

  static async rollbackVersion(
    taskId: number,
    versionId: number,
  ): Promise<SyncTask> {
    return this.post<SyncTask>(
      `/tasks/${taskId}/versions/${versionId}/rollback`,
      {},
    );
  }

  static async deleteVersion(
    taskId: number,
    versionId: number,
  ): Promise<{deleted: boolean}> {
    return this.delete<{deleted: boolean}>(
      `/tasks/${taskId}/versions/${versionId}`,
    );
  }

  static async validateTask(taskId: number): Promise<SyncValidateResult> {
    return this.post<SyncValidateResult>(`/tasks/${taskId}/validate`, {});
  }

  static async testConnections(taskId: number): Promise<SyncValidateResult> {
    return this.post<SyncValidateResult>(
      `/tasks/${taskId}/test-connections`,
      {},
    );
  }

  static async buildDag(taskId: number): Promise<SyncDagResult> {
    return this.post<SyncDagResult>(`/tasks/${taskId}/dag`, {});
  }

  static async previewTask(
    taskId: number,
    request?: PreviewSyncTaskRequest,
  ): Promise<SyncJobInstance> {
    return this.post<SyncJobInstance>(
      `/tasks/${taskId}/preview`,
      request || {},
    );
  }

  static async submitTask(taskId: number): Promise<SyncJobInstance> {
    return this.post<SyncJobInstance>(`/tasks/${taskId}/submit`, {});
  }

  static async listJobs(params?: {
    current?: number;
    size?: number;
    task_id?: number;
    run_type?: string;
  }): Promise<SyncJobListData> {
    return this.get<SyncJobListData>('/jobs', params);
  }

  static async getJob(jobId: number): Promise<SyncJobInstance> {
    return this.get<SyncJobInstance>(`/jobs/${jobId}`);
  }

  static async getPreviewSnapshot(
    jobId: number,
    params?: {table_path?: string},
  ): Promise<SyncPreviewSnapshot> {
    return this.get<SyncPreviewSnapshot>(`/jobs/${jobId}/preview`, params);
  }

  static async getJobCheckpoint(
    jobId: number,
    params?: {pipeline_id?: number; limit?: number; status?: string},
  ): Promise<SyncCheckpointSnapshot> {
    return this.get<SyncCheckpointSnapshot>(
      `/jobs/${jobId}/checkpoint`,
      params,
    );
  }

  static async getJobLogs(
    jobId: number,
    params?: {
      offset?: string;
      limit_bytes?: number;
      keyword?: string;
      level?: 'warn' | 'error' | 'all';
      signal?: AbortSignal;
    },
  ): Promise<SyncJobLogsResult> {
    const {signal, ...query} = params || {};
    const response = await apiClient.get<ApiResponse<SyncJobLogsResult>>(
      `${this.basePath}/jobs/${jobId}/logs`,
      {params: query, signal},
    );
    return response.data.data;
  }

  static async recoverJob(jobId: number): Promise<SyncJobInstance> {
    return this.post<SyncJobInstance>(`/jobs/${jobId}/recover`, {});
  }

  static async cancelJob(
    jobId: number,
    request?: {stop_with_savepoint?: boolean},
  ): Promise<SyncJobInstance> {
    return this.post<SyncJobInstance>(`/jobs/${jobId}/cancel`, request || {});
  }
}
