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
 * Monitoring Center Service
 * 监控中心服务
 */

import apiClient from '../core/api-client';
import {BaseService} from '../core/base.service';
import type {
  AcknowledgeAlertRequest,
  AlertActionResult,
  AlertFilterParams,
  AlertInstanceActionResult,
  AlertInstanceFilterParams,
  AlertInstanceListData,
  AlertListData,
  AlertRule,
  AlertRuleListData,
  ClusterHealthData,
  ClusterMonitoringOverviewData,
  IntegrationStatusData,
  MonitoringOverviewData,
  NotificationChannel,
  NotificationChannelListData,
  NotificationDeliveryFilterParams,
  NotificationDeliveryListData,
  NotificationChannelTestResult,
  NotificationRoute,
  NotificationRouteListData,
  PlatformHealthData,
  RemoteAlertFilterParams,
  RemoteAlertListData,
  SilenceAlertRequest,
  UpsertNotificationChannelRequest,
  UpsertNotificationRouteRequest,
  UpdateAlertRuleRequest,
} from './types';

export class MonitoringService extends BaseService {
  protected static readonly basePath = '/monitoring';

  /**
   * Get global monitoring overview.
   * 获取全局监控总览。
   */
  static async getOverview(): Promise<MonitoringOverviewData> {
    return this.get<MonitoringOverviewData>('/overview');
  }

  /**
   * Get monitoring overview for one cluster.
   * 获取单集群监控总览。
   */
  static async getClusterOverview(
    clusterId: number,
  ): Promise<ClusterMonitoringOverviewData> {
    return this.get<ClusterMonitoringOverviewData>(
      `/clusters/${clusterId}/overview`,
    );
  }

  /**
   * List alerts for monitoring center.
   * 获取监控中心告警列表。
   */
  static async getAlerts(params?: AlertFilterParams): Promise<AlertListData> {
    return this.get<AlertListData>(
      '/alerts',
      params as Record<string, unknown> | undefined,
    );
  }

  /**
   * List unified alert instances for monitoring center.
   * 获取统一告警实例列表。
   */
  static async getAlertInstances(
    params?: AlertInstanceFilterParams,
  ): Promise<AlertInstanceListData> {
    return this.get<AlertInstanceListData>(
      '/alert-instances',
      params as Record<string, unknown> | undefined,
    );
  }

  /**
   * Acknowledge one alert event.
   * 确认一条告警事件。
   */
  static async acknowledgeAlert(
    eventId: number,
    payload?: AcknowledgeAlertRequest,
  ): Promise<AlertActionResult> {
    return this.post<AlertActionResult>(
      `/alerts/${eventId}/ack`,
      payload || {},
    );
  }

  /**
   * Acknowledge one unified alert instance.
   * 确认一条统一告警实例。
   */
  static async acknowledgeAlertInstance(
    alertId: string,
    payload?: AcknowledgeAlertRequest,
  ): Promise<AlertInstanceActionResult> {
    return this.post<AlertInstanceActionResult>(
      `/alert-instances/${encodeURIComponent(alertId)}/ack`,
      payload || {},
    );
  }

  /**
   * Silence one alert event.
   * 静默一条告警事件。
   */
  static async silenceAlert(
    eventId: number,
    payload: SilenceAlertRequest,
  ): Promise<AlertActionResult> {
    return this.post<AlertActionResult>(`/alerts/${eventId}/silence`, payload);
  }

  /**
   * Silence one unified alert instance.
   * 静默一条统一告警实例。
   */
  static async silenceAlertInstance(
    alertId: string,
    payload: SilenceAlertRequest,
  ): Promise<AlertInstanceActionResult> {
    return this.post<AlertInstanceActionResult>(
      `/alert-instances/${encodeURIComponent(alertId)}/silence`,
      payload,
    );
  }

  /**
   * List alert rules for one cluster.
   * 获取集群告警规则列表。
   */
  static async getClusterRules(clusterId: number): Promise<AlertRuleListData> {
    return this.get<AlertRuleListData>(`/clusters/${clusterId}/rules`);
  }

  /**
   * Update one alert rule.
   * 更新一条告警规则。
   */
  static async updateClusterRule(
    clusterId: number,
    ruleId: number,
    payload: UpdateAlertRuleRequest,
  ): Promise<AlertRule> {
    return this.put<AlertRule>(
      `/clusters/${clusterId}/rules/${ruleId}`,
      payload,
    );
  }

  /**
   * Get observability integration status.
   * 获取可观测组件联动状态。
   */
  static async getIntegrationStatus(): Promise<IntegrationStatusData> {
    return this.get<IntegrationStatusData>('/integration/status');
  }

  /**
   * List remote alerts ingested from Alertmanager webhook.
   * 查询 Alertmanager webhook 入库后的远程告警。
   */
  static async getRemoteAlerts(
    params?: RemoteAlertFilterParams,
  ): Promise<RemoteAlertListData> {
    return this.get<RemoteAlertListData>(
      '/remote-alerts',
      params as Record<string, unknown> | undefined,
    );
  }

  /**
   * Get platform-level health summary.
   * 获取平台级健康摘要。
   */
  static async getPlatformHealth(): Promise<PlatformHealthData> {
    return this.get<PlatformHealthData>('/platform-health');
  }

  /**
   * Get cluster-level health summary.
   * 获取集群级健康摘要。
   */
  static async getClustersHealth(): Promise<ClusterHealthData> {
    const response = await apiClient.get<{
      error_msg: string;
      data: ClusterHealthData;
    }>('/clusters/health');
    if (response.data.error_msg) {
      throw new Error(response.data.error_msg);
    }
    return response.data.data;
  }

  /**
   * List notification channels.
   * 获取通知渠道列表。
   */
  static async listNotificationChannels(): Promise<NotificationChannelListData> {
    return this.get<NotificationChannelListData>('/notification-channels');
  }

  /**
   * Create notification channel.
   * 创建通知渠道。
   */
  static async createNotificationChannel(
    payload: UpsertNotificationChannelRequest,
  ): Promise<NotificationChannel> {
    return this.post<NotificationChannel>('/notification-channels', payload);
  }

  /**
   * Update notification channel.
   * 更新通知渠道。
   */
  static async updateNotificationChannel(
    id: number,
    payload: UpsertNotificationChannelRequest,
  ): Promise<NotificationChannel> {
    return this.put<NotificationChannel>(
      `/notification-channels/${id}`,
      payload,
    );
  }

  /**
   * Delete notification channel.
   * 删除通知渠道。
   */
  static async deleteNotificationChannel(id: number): Promise<{id: number}> {
    return this.delete<{id: number}>(`/notification-channels/${id}`);
  }

  /**
   * Test one notification channel.
   * 测试一个通知渠道。
   */
  static async testNotificationChannel(
    id: number,
  ): Promise<NotificationChannelTestResult> {
    return this.post<NotificationChannelTestResult>(
      `/notification-channels/${id}/test`,
      {},
    );
  }

  /**
   * List notification delivery history.
   * 获取通知投递历史。
   */
  static async listNotificationDeliveries(
    params?: NotificationDeliveryFilterParams,
  ): Promise<NotificationDeliveryListData> {
    return this.get<NotificationDeliveryListData>(
      '/notification-deliveries',
      params as Record<string, unknown> | undefined,
    );
  }

  /**
   * List notification routes.
   * 获取通知路由列表。
   */
  static async listNotificationRoutes(): Promise<NotificationRouteListData> {
    return this.get<NotificationRouteListData>('/notification-routes');
  }

  /**
   * Create notification route.
   * 创建通知路由。
   */
  static async createNotificationRoute(
    payload: UpsertNotificationRouteRequest,
  ): Promise<NotificationRoute> {
    return this.post<NotificationRoute>('/notification-routes', payload);
  }

  /**
   * Update notification route.
   * 更新通知路由。
   */
  static async updateNotificationRoute(
    id: number,
    payload: UpsertNotificationRouteRequest,
  ): Promise<NotificationRoute> {
    return this.put<NotificationRoute>(`/notification-routes/${id}`, payload);
  }

  /**
   * Delete notification route.
   * 删除通知路由。
   */
  static async deleteNotificationRoute(id: number): Promise<{id: number}> {
    return this.delete<{id: number}>(`/notification-routes/${id}`);
  }

  // ==================== Safe Methods 安全方法 ====================

  static async getOverviewSafe(): Promise<{
    success: boolean;
    data?: MonitoringOverviewData;
    error?: string;
  }> {
    try {
      const data = await this.getOverview();
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get monitoring overview',
      };
    }
  }

  static async getClusterOverviewSafe(clusterId: number): Promise<{
    success: boolean;
    data?: ClusterMonitoringOverviewData;
    error?: string;
  }> {
    try {
      const data = await this.getClusterOverview(clusterId);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get cluster monitoring overview',
      };
    }
  }

  static async getAlertsSafe(params?: AlertFilterParams): Promise<{
    success: boolean;
    data?: AlertListData;
    error?: string;
  }> {
    try {
      const data = await this.getAlerts(params);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error.message : 'Failed to get alerts',
      };
    }
  }

  static async getAlertInstancesSafe(
    params?: AlertInstanceFilterParams,
  ): Promise<{
    success: boolean;
    data?: AlertInstanceListData;
    error?: string;
  }> {
    try {
      const data = await this.getAlertInstances(params);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get alert instances',
      };
    }
  }

  static async acknowledgeAlertSafe(
    eventId: number,
    payload?: AcknowledgeAlertRequest,
  ): Promise<{
    success: boolean;
    data?: AlertActionResult;
    error?: string;
  }> {
    try {
      const data = await this.acknowledgeAlert(eventId, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to acknowledge alert',
      };
    }
  }

  static async acknowledgeAlertInstanceSafe(
    alertId: string,
    payload?: AcknowledgeAlertRequest,
  ): Promise<{
    success: boolean;
    data?: AlertInstanceActionResult;
    error?: string;
  }> {
    try {
      const data = await this.acknowledgeAlertInstance(alertId, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to acknowledge alert instance',
      };
    }
  }

  static async silenceAlertSafe(
    eventId: number,
    payload: SilenceAlertRequest,
  ): Promise<{
    success: boolean;
    data?: AlertActionResult;
    error?: string;
  }> {
    try {
      const data = await this.silenceAlert(eventId, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error ? error.message : 'Failed to silence alert',
      };
    }
  }

  static async silenceAlertInstanceSafe(
    alertId: string,
    payload: SilenceAlertRequest,
  ): Promise<{
    success: boolean;
    data?: AlertInstanceActionResult;
    error?: string;
  }> {
    try {
      const data = await this.silenceAlertInstance(alertId, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to silence alert instance',
      };
    }
  }

  static async getClusterRulesSafe(clusterId: number): Promise<{
    success: boolean;
    data?: AlertRuleListData;
    error?: string;
  }> {
    try {
      const data = await this.getClusterRules(clusterId);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get cluster rules',
      };
    }
  }

  static async updateClusterRuleSafe(
    clusterId: number,
    ruleId: number,
    payload: UpdateAlertRuleRequest,
  ): Promise<{
    success: boolean;
    data?: AlertRule;
    error?: string;
  }> {
    try {
      const data = await this.updateClusterRule(clusterId, ruleId, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to update cluster rule',
      };
    }
  }

  static async getIntegrationStatusSafe(): Promise<{
    success: boolean;
    data?: IntegrationStatusData;
    error?: string;
  }> {
    try {
      const data = await this.getIntegrationStatus();
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get integration status',
      };
    }
  }

  static async getRemoteAlertsSafe(params?: RemoteAlertFilterParams): Promise<{
    success: boolean;
    data?: RemoteAlertListData;
    error?: string;
  }> {
    try {
      const data = await this.getRemoteAlerts(params);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get remote alerts',
      };
    }
  }

  static async getPlatformHealthSafe(): Promise<{
    success: boolean;
    data?: PlatformHealthData;
    error?: string;
  }> {
    try {
      const data = await this.getPlatformHealth();
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get platform health',
      };
    }
  }

  static async getClustersHealthSafe(): Promise<{
    success: boolean;
    data?: ClusterHealthData;
    error?: string;
  }> {
    try {
      const data = await this.getClustersHealth();
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to get clusters health',
      };
    }
  }

  static async listNotificationChannelsSafe(): Promise<{
    success: boolean;
    data?: NotificationChannelListData;
    error?: string;
  }> {
    try {
      const data = await this.listNotificationChannels();
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to list notification channels',
      };
    }
  }

  static async createNotificationChannelSafe(
    payload: UpsertNotificationChannelRequest,
  ): Promise<{
    success: boolean;
    data?: NotificationChannel;
    error?: string;
  }> {
    try {
      const data = await this.createNotificationChannel(payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to create notification channel',
      };
    }
  }

  static async updateNotificationChannelSafe(
    id: number,
    payload: UpsertNotificationChannelRequest,
  ): Promise<{
    success: boolean;
    data?: NotificationChannel;
    error?: string;
  }> {
    try {
      const data = await this.updateNotificationChannel(id, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to update notification channel',
      };
    }
  }

  static async deleteNotificationChannelSafe(id: number): Promise<{
    success: boolean;
    data?: {id: number};
    error?: string;
  }> {
    try {
      const data = await this.deleteNotificationChannel(id);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to delete notification channel',
      };
    }
  }

  static async testNotificationChannelSafe(id: number): Promise<{
    success: boolean;
    data?: NotificationChannelTestResult;
    error?: string;
  }> {
    try {
      const data = await this.testNotificationChannel(id);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to test notification channel',
      };
    }
  }

  static async listNotificationDeliveriesSafe(
    params?: NotificationDeliveryFilterParams,
  ): Promise<{
    success: boolean;
    data?: NotificationDeliveryListData;
    error?: string;
  }> {
    try {
      const data = await this.listNotificationDeliveries(params);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to list notification deliveries',
      };
    }
  }

  static async listNotificationRoutesSafe(): Promise<{
    success: boolean;
    data?: NotificationRouteListData;
    error?: string;
  }> {
    try {
      const data = await this.listNotificationRoutes();
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to list notification routes',
      };
    }
  }

  static async createNotificationRouteSafe(
    payload: UpsertNotificationRouteRequest,
  ): Promise<{
    success: boolean;
    data?: NotificationRoute;
    error?: string;
  }> {
    try {
      const data = await this.createNotificationRoute(payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to create notification route',
      };
    }
  }

  static async updateNotificationRouteSafe(
    id: number,
    payload: UpsertNotificationRouteRequest,
  ): Promise<{
    success: boolean;
    data?: NotificationRoute;
    error?: string;
  }> {
    try {
      const data = await this.updateNotificationRoute(id, payload);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to update notification route',
      };
    }
  }

  static async deleteNotificationRouteSafe(id: number): Promise<{
    success: boolean;
    data?: {id: number};
    error?: string;
  }> {
    try {
      const data = await this.deleteNotificationRoute(id);
      return {success: true, data};
    } catch (error) {
      return {
        success: false,
        error:
          error instanceof Error
            ? error.message
            : 'Failed to delete notification route',
      };
    }
  }
}
