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
