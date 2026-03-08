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

/**
 * SeaTunnel Upgrade Service
 * SeaTunnel 升级服务
 */

import {BaseService} from '../core/base.service';
import apiClient from '../core/api-client';
import type {ApiResponse} from '../core/types';
import type {
  CreatePlanRequest,
  CreatePlanResult,
  ExecutePlanRequest,
  ListUpgradeTasksQuery,
  PrecheckRequest,
  PrecheckResult,
  ServiceResult,
  TaskListData,
  TaskLogsData,
  TaskLogsQuery,
  TaskStepsData,
  UpgradePlanRecord,
  UpgradeTask,
} from './types';

export class StUpgradeService extends BaseService {
  protected static readonly basePath = '/st-upgrade';

  private static async postWithAcceptedStatuses<T>(
    path: string,
    payload: unknown,
    acceptedStatuses: number[],
  ): Promise<T> {
    const response = await apiClient.post<ApiResponse<T>>(
      this.getFullPath(path),
      payload,
      {
        validateStatus: (status) => acceptedStatuses.includes(status),
      },
    );

    if (response.data.error_msg) {
      throw new Error(response.data.error_msg);
    }
    return response.data.data;
  }

  static async runPrecheck(payload: PrecheckRequest): Promise<PrecheckResult> {
    return this.postWithAcceptedStatuses<PrecheckResult>(
      '/precheck',
      payload,
      [200, 409],
    );
  }

  static async createPlan(
    payload: CreatePlanRequest,
  ): Promise<CreatePlanResult> {
    return this.postWithAcceptedStatuses<CreatePlanResult>(
      '/plan',
      payload,
      [201, 409],
    );
  }

  static async getPlan(planId: number): Promise<UpgradePlanRecord> {
    return this.get<UpgradePlanRecord>(`/plans/${planId}`);
  }

  static async executePlan(payload: ExecutePlanRequest): Promise<UpgradeTask> {
    return this.post<UpgradeTask>('/execute', payload);
  }

  static async listTasks(query?: ListUpgradeTasksQuery): Promise<TaskListData> {
    const result = await this.get<TaskListData>(
      '/tasks',
      query as Record<string, unknown> | undefined,
    );
    return {
      ...result,
      items: Array.isArray(result.items) ? result.items : [],
    };
  }

  static async getTask(taskId: number): Promise<UpgradeTask> {
    return this.get<UpgradeTask>(`/tasks/${taskId}`);
  }

  static async getTaskSteps(taskId: number): Promise<TaskStepsData> {
    return this.get<TaskStepsData>(`/tasks/${taskId}/steps`);
  }

  static async getTaskLogs(
    taskId: number,
    query?: TaskLogsQuery,
  ): Promise<TaskLogsData> {
    return this.get<TaskLogsData>(
      `/tasks/${taskId}/logs`,
      query as Record<string, unknown> | undefined,
    );
  }

  static getTaskEventsUrl(taskId: number): string {
    return `/api/v1${this.basePath}/tasks/${taskId}/events/stream`;
  }

  static async runPrecheckSafe(
    payload: PrecheckRequest,
  ): Promise<ServiceResult<PrecheckResult>> {
    try {
      return {success: true, data: await this.runPrecheck(payload)};
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : '升级预检查失败',
      };
    }
  }

  static async createPlanSafe(
    payload: CreatePlanRequest,
  ): Promise<ServiceResult<CreatePlanResult>> {
    try {
      return {success: true, data: await this.createPlan(payload)};
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : '创建升级计划失败',
      };
    }
  }

  static async executePlanSafe(
    payload: ExecutePlanRequest,
  ): Promise<ServiceResult<UpgradeTask>> {
    try {
      return {success: true, data: await this.executePlan(payload)};
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : '启动升级执行失败',
      };
    }
  }

  static async listTasksSafe(
    query?: ListUpgradeTasksQuery,
  ): Promise<ServiceResult<TaskListData>> {
    try {
      return {success: true, data: await this.listTasks(query)};
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : '获取升级任务列表失败',
      };
    }
  }

  static async getTaskSafe(
    taskId: number,
  ): Promise<ServiceResult<UpgradeTask>> {
    try {
      return {success: true, data: await this.getTask(taskId)};
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : '获取升级任务失败',
      };
    }
  }
}
