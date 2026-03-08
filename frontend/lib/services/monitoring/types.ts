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
 * Monitoring Center Types
 * 监控中心类型定义
 */

import type {MonitorConfig, ProcessEvent} from '@/lib/services/monitor';

export interface EventStats {
  started: number;
  stopped: number;
  crashed: number;
  restarted: number;
  restart_failed: number;
  restart_limit_reached: number;
}

export interface MonitoringOverviewStats {
  total_clusters: number;
  healthy_clusters: number;
  unhealthy_clusters: number;
  unknown_clusters: number;
  total_nodes: number;
  online_nodes: number;
  offline_nodes: number;
  crashed_events_24h: number;
  restart_failed_events_24h: number;
  active_alerts_1h: number;
}

export interface ClusterMonitoringSummary {
  cluster_id: number;
  cluster_name: string;
  status: string;
  health_status: string;
  total_nodes: number;
  online_nodes: number;
  offline_nodes: number;
  crashed_events_24h: number;
  restart_failed_events_24h: number;
  active_alerts_1h: number;
  last_event_at?: string | null;
}

export interface MonitoringOverviewData {
  generated_at: string;
  stats: MonitoringOverviewStats;
  event_stats_24h: EventStats;
  clusters: ClusterMonitoringSummary[];
}

export interface ClusterBaseInfo {
  cluster_id: number;
  cluster_name: string;
  status: string;
  health_status: string;
}

export interface ClusterMonitoringDetailStats {
  total_nodes: number;
  online_nodes: number;
  offline_nodes: number;
  crashed_events_24h: number;
  restart_failed_events_24h: number;
  active_alerts_1h: number;
}

export interface NodeSnapshot {
  node_id: number;
  host_id: number;
  host_name: string;
  host_ip: string;
  role: string;
  status: string;
  is_online: boolean;
  process_pid: number;
}

export interface ClusterMonitoringOverviewData {
  generated_at: string;
  cluster: ClusterBaseInfo;
  stats: ClusterMonitoringDetailStats;
  event_stats_24h: EventStats;
  event_stats_1h: EventStats;
  monitor_config: MonitorConfig | null;
  nodes: NodeSnapshot[];
  recent_events: ProcessEvent[];
}

export type AlertSeverity = 'warning' | 'critical';
export type AlertStatus = 'firing' | 'acknowledged' | 'silenced';

export interface AlertStats {
  firing: number;
  acknowledged: number;
  silenced: number;
}

export interface AlertEvent {
  alert_id: string;
  event_id: number;
  cluster_id: number;
  cluster_name: string;
  node_id: number;
  host_id: number;
  hostname: string;
  ip: string;
  event_type: string;
  severity: AlertSeverity;
  status: AlertStatus;
  rule_key: string;
  rule_name: string;
  process_name: string;
  pid: number;
  role: string;
  details: string;
  created_at: string;
  acknowledged_by?: string;
  acknowledged_at?: string | null;
  silenced_by?: string;
  silenced_until?: string | null;
  latest_action_note?: string;
}

export interface AlertListData {
  generated_at: string;
  page: number;
  page_size: number;
  total: number;
  stats: AlertStats;
  alerts: AlertEvent[];
}

export type AlertSourceType = 'local_process_event' | 'remote_alertmanager';
export type AlertLifecycleStatus = 'firing' | 'resolved';
export type AlertHandlingStatus = 'pending' | 'acknowledged' | 'silenced';

export interface AlertInstanceStats {
  firing: number;
  resolved: number;
  pending: number;
  acknowledged: number;
  silenced: number;
}

export interface AlertInstanceSourceRef {
  event_id?: number;
  fingerprint?: string;
  event_type?: string;
  process_name?: string;
  hostname?: string;
  receiver?: string;
  env?: string;
}

export interface AlertInstance {
  alert_id: string;
  source_type: AlertSourceType;
  cluster_id: string;
  cluster_name: string;
  severity: AlertSeverity | string;
  alert_name: string;
  rule_key: string;
  summary: string;
  description: string;
  lifecycle_status: AlertLifecycleStatus;
  handling_status: AlertHandlingStatus;
  created_at: string;
  firing_at: string;
  resolved_at?: string | null;
  last_seen_at: string;
  acknowledged_by?: string;
  acknowledged_at?: string | null;
  silenced_by?: string;
  silenced_until?: string | null;
  latest_note?: string;
  source_ref?: AlertInstanceSourceRef | null;
}

export interface AlertInstanceListData {
  generated_at: string;
  page: number;
  page_size: number;
  total: number;
  stats: AlertInstanceStats;
  alerts: AlertInstance[];
}

export interface AlertInstanceFilterParams {
  source_type?: AlertSourceType;
  cluster_id?: string;
  severity?: AlertSeverity;
  lifecycle_status?: AlertLifecycleStatus;
  handling_status?: AlertHandlingStatus;
  start_time?: string;
  end_time?: string;
  page?: number;
  page_size?: number;
}

export interface AlertInstanceActionResult {
  alert_id: string;
  handling_status: AlertHandlingStatus;
  acknowledged_by?: string;
  acknowledged_at?: string | null;
  silenced_by?: string;
  silenced_until?: string | null;
  latest_note?: string;
}

export interface AlertFilterParams {
  cluster_id?: number;
  status?: AlertStatus;
  start_time?: string;
  end_time?: string;
  page?: number;
  page_size?: number;
}

export interface AcknowledgeAlertRequest {
  note?: string;
}

export interface SilenceAlertRequest {
  duration_minutes: number;
  note?: string;
}

export interface AlertActionResult {
  event_id: number;
  status: AlertStatus;
  acknowledged_by?: string;
  acknowledged_at?: string | null;
  silenced_by?: string;
  silenced_until?: string | null;
  latest_action_note?: string;
}

export type RemoteAlertStatus = 'firing' | 'resolved' | string;

export interface RemoteAlertEvent {
  id: number;
  fingerprint: string;
  status: RemoteAlertStatus;
  receiver: string;
  alert_name: string;
  severity: string;
  cluster_id: string;
  cluster_name: string;
  env: string;
  summary: string;
  description: string;
  starts_at: number;
  ends_at: number;
  resolved_at?: string | null;
  last_received_at: string;
  created_at: string;
  updated_at: string;
}

export interface RemoteAlertListData {
  generated_at: string;
  page: number;
  page_size: number;
  total: number;
  alerts: RemoteAlertEvent[];
}

export interface RemoteAlertFilterParams {
  cluster_id?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
  page?: number;
  page_size?: number;
}

export interface ClusterHealthItem {
  cluster_id: number;
  cluster_name: string;
  status: string;
  health_status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | string;
  total_nodes: number;
  online_nodes: number;
  offline_nodes: number;
  active_alerts: number;
  critical_alerts: number;
}

export interface ClusterHealthData {
  generated_at: string;
  total: number;
  clusters: ClusterHealthItem[];
}

export interface PlatformHealthData {
  generated_at: string;
  health_status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | string;
  total_clusters: number;
  healthy_clusters: number;
  degraded_clusters: number;
  unhealthy_clusters: number;
  unknown_clusters: number;
  active_alerts: number;
  critical_alerts: number;
}

export interface AlertRule {
  id: number;
  cluster_id: number;
  rule_key: string;
  rule_name: string;
  description: string;
  severity: AlertSeverity;
  enabled: boolean;
  threshold: number;
  window_seconds: number;
  created_at: string;
  updated_at: string;
}

export interface AlertRuleListData {
  generated_at: string;
  cluster_id: number;
  rules: AlertRule[];
}

export interface UpdateAlertRuleRequest {
  rule_name?: string;
  description?: string;
  severity?: AlertSeverity;
  enabled?: boolean;
  threshold?: number;
  window_seconds?: number;
}

export interface IntegrationComponentStatus {
  name: string;
  url: string;
  healthy: boolean;
  status_code: number;
  error?: string;
}

export interface IntegrationStatusData {
  generated_at: string;
  components: IntegrationComponentStatus[];
}

export type NotificationChannelType =
  | 'webhook'
  | 'email'
  | 'wecom'
  | 'dingtalk'
  | 'feishu';

export interface NotificationChannel {
  id: number;
  name: string;
  type: NotificationChannelType;
  enabled: boolean;
  endpoint: string;
  secret: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface NotificationChannelListData {
  generated_at: string;
  total: number;
  channels: NotificationChannel[];
}

export interface UpsertNotificationChannelRequest {
  name: string;
  type: NotificationChannelType;
  enabled?: boolean;
  endpoint: string;
  secret?: string;
  description?: string;
}

export interface NotificationChannelTestResult {
  channel_id: number;
  delivery_id: number;
  status: string;
  sent_at?: string | null;
  last_error?: string;
  status_code?: number;
  response_body?: string;
}

export interface NotificationRoute {
  id: number;
  name: string;
  enabled: boolean;
  source_type?: AlertSourceType | '' | string;
  cluster_id?: string;
  severity?: AlertSeverity | '' | string;
  rule_key?: string;
  channel_id: number;
  send_resolved: boolean;
  mute_if_acknowledged: boolean;
  mute_if_silenced: boolean;
  created_at: string;
  updated_at: string;
}

export interface NotificationRouteListData {
  generated_at: string;
  total: number;
  routes: NotificationRoute[];
}

export type NotificationDeliveryEventType = 'firing' | 'resolved' | 'test';
export type NotificationDeliveryStatus =
  | 'pending'
  | 'sending'
  | 'sent'
  | 'failed'
  | 'retrying'
  | 'canceled';

export interface NotificationDelivery {
  id: number;
  alert_id: string;
  source_type: string;
  source_key: string;
  cluster_id?: string;
  cluster_name?: string;
  alert_name?: string;
  channel_id: number;
  channel_name?: string;
  event_type: NotificationDeliveryEventType | string;
  status: NotificationDeliveryStatus | string;
  attempt_count: number;
  last_error?: string;
  response_status_code?: number;
  sent_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface NotificationDeliveryListData {
  generated_at: string;
  page: number;
  page_size: number;
  total: number;
  deliveries: NotificationDelivery[];
}

export interface NotificationDeliveryFilterParams {
  channel_id?: number;
  status?: NotificationDeliveryStatus;
  event_type?: NotificationDeliveryEventType;
  cluster_id?: string;
  start_time?: string;
  end_time?: string;
  page?: number;
  page_size?: number;
}

export interface UpsertNotificationRouteRequest {
  name: string;
  enabled?: boolean;
  source_type?: AlertSourceType | '';
  cluster_id?: string;
  severity?: AlertSeverity | '';
  rule_key?: string;
  channel_id: number;
  send_resolved?: boolean;
  mute_if_acknowledged?: boolean;
  mute_if_silenced?: boolean;
}
