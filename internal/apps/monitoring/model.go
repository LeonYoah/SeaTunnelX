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

// Package monitoring provides cluster monitoring overview APIs for SeaTunnelX.
// monitoring 包为 SeaTunnelX 提供集群监控总览 API。
package monitoring

import (
	"fmt"
	"time"

	"github.com/seatunnel/seatunnelX/internal/apps/monitor"
)

// Response is the standard API response for monitoring endpoints.
// Response 是监控接口的标准响应结构。
type Response struct {
	ErrorMsg string      `json:"error_msg"`
	Data     interface{} `json:"data"`
}

// EventStats summarizes process events.
// EventStats 汇总进程事件统计。
type EventStats struct {
	Started             int64 `json:"started"`
	Stopped             int64 `json:"stopped"`
	Crashed             int64 `json:"crashed"`
	Restarted           int64 `json:"restarted"`
	RestartFailed       int64 `json:"restart_failed"`
	RestartLimitReached int64 `json:"restart_limit_reached"`
}

// CriticalCount returns the number of critical events.
// CriticalCount 返回关键告警事件数量。
func (e *EventStats) CriticalCount() int64 {
	if e == nil {
		return 0
	}
	return e.Crashed + e.RestartFailed + e.RestartLimitReached
}

// OverviewStats represents global monitoring statistics.
// OverviewStats 表示全局监控统计。
type OverviewStats struct {
	TotalClusters     int   `json:"total_clusters"`
	HealthyClusters   int   `json:"healthy_clusters"`
	UnhealthyClusters int   `json:"unhealthy_clusters"`
	UnknownClusters   int   `json:"unknown_clusters"`
	TotalNodes        int   `json:"total_nodes"`
	OnlineNodes       int   `json:"online_nodes"`
	OfflineNodes      int   `json:"offline_nodes"`
	CrashedEvents24h  int64 `json:"crashed_events_24h"`
	RestartFailed24h  int64 `json:"restart_failed_events_24h"`
	ActiveAlerts1h    int64 `json:"active_alerts_1h"`
}

// ClusterMonitoringSummary represents one cluster in monitoring overview.
// ClusterMonitoringSummary 表示监控总览中的单个集群摘要。
type ClusterMonitoringSummary struct {
	ClusterID              uint       `json:"cluster_id"`
	ClusterName            string     `json:"cluster_name"`
	Status                 string     `json:"status"`
	HealthStatus           string     `json:"health_status"`
	TotalNodes             int        `json:"total_nodes"`
	OnlineNodes            int        `json:"online_nodes"`
	OfflineNodes           int        `json:"offline_nodes"`
	CrashedEvents24h       int64      `json:"crashed_events_24h"`
	RestartFailedEvents24h int64      `json:"restart_failed_events_24h"`
	ActiveAlerts1h         int64      `json:"active_alerts_1h"`
	LastEventAt            *time.Time `json:"last_event_at"`
}

// OverviewData represents the monitoring overview payload.
// OverviewData 表示监控总览数据。
type OverviewData struct {
	GeneratedAt   time.Time                   `json:"generated_at"`
	Stats         *OverviewStats              `json:"stats"`
	EventStats24h *EventStats                 `json:"event_stats_24h"`
	Clusters      []*ClusterMonitoringSummary `json:"clusters"`
}

// ClusterBaseInfo represents basic cluster status in detail API.
// ClusterBaseInfo 表示集群详情中的基础状态。
type ClusterBaseInfo struct {
	ClusterID    uint   `json:"cluster_id"`
	ClusterName  string `json:"cluster_name"`
	Status       string `json:"status"`
	HealthStatus string `json:"health_status"`
}

// ClusterDetailStats represents monitoring stats for one cluster.
// ClusterDetailStats 表示单集群监控统计。
type ClusterDetailStats struct {
	TotalNodes             int   `json:"total_nodes"`
	OnlineNodes            int   `json:"online_nodes"`
	OfflineNodes           int   `json:"offline_nodes"`
	CrashedEvents24h       int64 `json:"crashed_events_24h"`
	RestartFailedEvents24h int64 `json:"restart_failed_events_24h"`
	ActiveAlerts1h         int64 `json:"active_alerts_1h"`
}

// NodeSnapshot represents node runtime snapshot in monitoring detail.
// NodeSnapshot 表示监控详情中的节点运行快照。
type NodeSnapshot struct {
	NodeID     uint   `json:"node_id"`
	HostID     uint   `json:"host_id"`
	HostName   string `json:"host_name"`
	HostIP     string `json:"host_ip"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	IsOnline   bool   `json:"is_online"`
	ProcessPID int    `json:"process_pid"`
}

// ClusterOverviewData represents one cluster monitoring detail payload.
// ClusterOverviewData 表示单集群监控详情数据。
type ClusterOverviewData struct {
	GeneratedAt   time.Time                       `json:"generated_at"`
	Cluster       *ClusterBaseInfo                `json:"cluster"`
	Stats         *ClusterDetailStats             `json:"stats"`
	EventStats24h *EventStats                     `json:"event_stats_24h"`
	EventStats1h  *EventStats                     `json:"event_stats_1h"`
	MonitorConfig *monitor.MonitorConfig          `json:"monitor_config"`
	Nodes         []*NodeSnapshot                 `json:"nodes"`
	RecentEvents  []*monitor.ProcessEventWithHost `json:"recent_events"`
}

// AlertSeverity represents alert severity level.
// AlertSeverity 表示告警严重级别。
type AlertSeverity string

const (
	// AlertSeverityWarning indicates warning-level alerts.
	// AlertSeverityWarning 表示告警级别为 warning。
	AlertSeverityWarning AlertSeverity = "warning"
	// AlertSeverityCritical indicates critical-level alerts.
	// AlertSeverityCritical 表示告警级别为 critical。
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents alert lifecycle state.
// AlertStatus 表示告警生命周期状态。
type AlertStatus string

const (
	// AlertStatusFiring indicates the alert is currently active.
	// AlertStatusFiring 表示告警正在触发。
	AlertStatusFiring AlertStatus = "firing"
	// AlertStatusAcknowledged indicates the alert has been acknowledged.
	// AlertStatusAcknowledged 表示告警已确认。
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	// AlertStatusSilenced indicates the alert is temporarily silenced.
	// AlertStatusSilenced 表示告警处于静默状态。
	AlertStatusSilenced AlertStatus = "silenced"
)

const (
	// AlertRuleKeyProcessCrashed is the rule key for process crashed events.
	// AlertRuleKeyProcessCrashed 是进程崩溃规则键。
	AlertRuleKeyProcessCrashed = "process_crashed"
	// AlertRuleKeyProcessRestartFailed is the rule key for process restart failed events.
	// AlertRuleKeyProcessRestartFailed 是进程重启失败规则键。
	AlertRuleKeyProcessRestartFailed = "process_restart_failed"
	// AlertRuleKeyProcessRestartLimitReached is the rule key for process restart limit reached events.
	// AlertRuleKeyProcessRestartLimitReached 是达到重启上限规则键。
	AlertRuleKeyProcessRestartLimitReached = "process_restart_limit_reached"
)

// AlertFilter represents query filters for alert list.
// AlertFilter 表示告警列表查询过滤条件。
type AlertFilter struct {
	ClusterID uint        `json:"cluster_id"`
	Status    AlertStatus `json:"status"`
	StartTime *time.Time  `json:"start_time"`
	EndTime   *time.Time  `json:"end_time"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
}

// AlertStats summarizes alert status counts in current query result.
// AlertStats 汇总当前查询结果的告警状态数量。
type AlertStats struct {
	Firing       int64 `json:"firing"`
	Acknowledged int64 `json:"acknowledged"`
	Silenced     int64 `json:"silenced"`
}

// AlertEvent represents one alert item in alert center.
// AlertEvent 表示告警中心中的单条告警记录。
type AlertEvent struct {
	AlertID          string                   `json:"alert_id"`
	EventID          uint                     `json:"event_id"`
	ClusterID        uint                     `json:"cluster_id"`
	ClusterName      string                   `json:"cluster_name"`
	NodeID           uint                     `json:"node_id"`
	HostID           uint                     `json:"host_id"`
	Hostname         string                   `json:"hostname"`
	IP               string                   `json:"ip"`
	EventType        monitor.ProcessEventType `json:"event_type"`
	Severity         AlertSeverity            `json:"severity"`
	Status           AlertStatus              `json:"status"`
	RuleKey          string                   `json:"rule_key"`
	RuleName         string                   `json:"rule_name"`
	ProcessName      string                   `json:"process_name"`
	PID              int                      `json:"pid"`
	Role             string                   `json:"role"`
	Details          string                   `json:"details"`
	CreatedAt        time.Time                `json:"created_at"`
	AcknowledgedBy   string                   `json:"acknowledged_by,omitempty"`
	AcknowledgedAt   *time.Time               `json:"acknowledged_at,omitempty"`
	SilencedBy       string                   `json:"silenced_by,omitempty"`
	SilencedUntil    *time.Time               `json:"silenced_until,omitempty"`
	LatestActionNote string                   `json:"latest_action_note,omitempty"`
}

// AlertListData represents alert center list payload.
// AlertListData 表示告警中心列表响应数据。
type AlertListData struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Page        int           `json:"page"`
	PageSize    int           `json:"page_size"`
	Total       int64         `json:"total"`
	Stats       *AlertStats   `json:"stats"`
	Alerts      []*AlertEvent `json:"alerts"`
}

// AlertRuleDTO represents a monitoring alert rule for frontend.
// AlertRuleDTO 表示前端可消费的告警规则。
type AlertRuleDTO struct {
	ID            uint          `json:"id"`
	ClusterID     uint          `json:"cluster_id"`
	RuleKey       string        `json:"rule_key"`
	RuleName      string        `json:"rule_name"`
	Description   string        `json:"description"`
	Severity      AlertSeverity `json:"severity"`
	Enabled       bool          `json:"enabled"`
	Threshold     int           `json:"threshold"`
	WindowSeconds int           `json:"window_seconds"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// AlertRuleListData represents rule list payload.
// AlertRuleListData 表示规则列表响应数据。
type AlertRuleListData struct {
	GeneratedAt time.Time       `json:"generated_at"`
	ClusterID   uint            `json:"cluster_id"`
	Rules       []*AlertRuleDTO `json:"rules"`
}

// UpdateAlertRuleRequest represents patch fields for one alert rule.
// UpdateAlertRuleRequest 表示告警规则更新请求。
type UpdateAlertRuleRequest struct {
	RuleName      *string        `json:"rule_name"`
	Description   *string        `json:"description"`
	Severity      *AlertSeverity `json:"severity"`
	Enabled       *bool          `json:"enabled"`
	Threshold     *int           `json:"threshold"`
	WindowSeconds *int           `json:"window_seconds"`
}

// Validate validates update request values.
// Validate 验证规则更新请求参数。
func (r *UpdateAlertRuleRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("empty request")
	}
	if r.Severity != nil {
		if *r.Severity != AlertSeverityWarning && *r.Severity != AlertSeverityCritical {
			return fmt.Errorf("invalid severity")
		}
	}
	if r.Threshold != nil && (*r.Threshold < 1 || *r.Threshold > 9999) {
		return fmt.Errorf("threshold must be between 1 and 9999")
	}
	if r.WindowSeconds != nil && (*r.WindowSeconds < 10 || *r.WindowSeconds > 86400) {
		return fmt.Errorf("window_seconds must be between 10 and 86400")
	}
	return nil
}

// AcknowledgeAlertRequest represents acknowledge action payload.
// AcknowledgeAlertRequest 表示确认告警请求。
type AcknowledgeAlertRequest struct {
	Note string `json:"note"`
}

// SilenceAlertRequest represents silence action payload.
// SilenceAlertRequest 表示静默告警请求。
type SilenceAlertRequest struct {
	DurationMinutes int    `json:"duration_minutes"`
	Note            string `json:"note"`
}

// Validate validates silence request.
// Validate 验证静默请求参数。
func (r *SilenceAlertRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("empty request")
	}
	if r.DurationMinutes < 1 || r.DurationMinutes > 7*24*60 {
		return fmt.Errorf("duration_minutes must be between 1 and 10080")
	}
	return nil
}

// AlertActionResult represents action result on one alert.
// AlertActionResult 表示单条告警动作执行结果。
type AlertActionResult struct {
	EventID          uint        `json:"event_id"`
	Status           AlertStatus `json:"status"`
	AcknowledgedBy   string      `json:"acknowledged_by,omitempty"`
	AcknowledgedAt   *time.Time  `json:"acknowledged_at,omitempty"`
	SilencedBy       string      `json:"silenced_by,omitempty"`
	SilencedUntil    *time.Time  `json:"silenced_until,omitempty"`
	LatestActionNote string      `json:"latest_action_note,omitempty"`
}

// IntegrationComponentStatus represents one monitoring stack component status.
// IntegrationComponentStatus 表示监控栈组件状态。
type IntegrationComponentStatus struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	Healthy    bool   `json:"healthy"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
}

// IntegrationStatusData represents full integration status payload.
// IntegrationStatusData 表示集成状态响应数据。
type IntegrationStatusData struct {
	GeneratedAt time.Time                     `json:"generated_at"`
	Components  []*IntegrationComponentStatus `json:"components"`
}

// NotificationChannelDTO is frontend DTO for notification channels.
// NotificationChannelDTO 是前端使用的通知渠道 DTO。
type NotificationChannelDTO struct {
	ID          uint                    `json:"id"`
	Name        string                  `json:"name"`
	Type        NotificationChannelType `json:"type"`
	Enabled     bool                    `json:"enabled"`
	Endpoint    string                  `json:"endpoint"`
	Secret      string                  `json:"secret"`
	Description string                  `json:"description"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

// NotificationChannelListData represents channel list payload.
// NotificationChannelListData 表示通知渠道列表响应。
type NotificationChannelListData struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Total       int                       `json:"total"`
	Channels    []*NotificationChannelDTO `json:"channels"`
}

// UpsertNotificationChannelRequest represents create/update channel payload.
// UpsertNotificationChannelRequest 表示新增/更新通知渠道请求。
type UpsertNotificationChannelRequest struct {
	Name        string                  `json:"name"`
	Type        NotificationChannelType `json:"type"`
	Enabled     *bool                   `json:"enabled"`
	Endpoint    string                  `json:"endpoint"`
	Secret      string                  `json:"secret"`
	Description string                  `json:"description"`
}

// Validate validates request values.
// Validate 校验通知渠道请求参数。
func (r *UpsertNotificationChannelRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("empty request")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch r.Type {
	case NotificationChannelTypeWebhook, NotificationChannelTypeEmail, NotificationChannelTypeWeCom, NotificationChannelTypeDingTalk, NotificationChannelTypeFeishu:
	default:
		return fmt.Errorf("invalid channel type")
	}
	if r.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	return nil
}

// PrometheusSDTargetGroup is one target group in Prometheus HTTP SD response.
// PrometheusSDTargetGroup 是 Prometheus HTTP SD 响应中的目标组。
type PrometheusSDTargetGroup struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// AlertmanagerWebhookPayload represents Alertmanager webhook body.
// AlertmanagerWebhookPayload 表示 Alertmanager webhook 请求体。
type AlertmanagerWebhookPayload struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []*WebhookAlert   `json:"alerts"`
}

// WebhookAlert represents one alert item in webhook payload.
// WebhookAlert 表示 webhook 中的单条告警。
type WebhookAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// AlertmanagerWebhookResult represents processing result for one webhook request.
// AlertmanagerWebhookResult 表示单次 webhook 处理结果。
type AlertmanagerWebhookResult struct {
	Received int      `json:"received"`
	Stored   int      `json:"stored"`
	Errors   []string `json:"errors,omitempty"`
}

// RemoteAlertFilter represents query filters for remote alert records.
// RemoteAlertFilter 表示远程告警记录查询过滤条件。
type RemoteAlertFilter struct {
	ClusterID string     `json:"cluster_id"`
	Status    string     `json:"status"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// RemoteAlertItem represents one remote alert record returned to frontend.
// RemoteAlertItem 表示返回给前端的单条远程告警记录。
type RemoteAlertItem struct {
	ID             uint       `json:"id"`
	Fingerprint    string     `json:"fingerprint"`
	Status         string     `json:"status"`
	Receiver       string     `json:"receiver"`
	AlertName      string     `json:"alert_name"`
	Severity       string     `json:"severity"`
	ClusterID      string     `json:"cluster_id"`
	ClusterName    string     `json:"cluster_name"`
	Env            string     `json:"env"`
	Summary        string     `json:"summary"`
	Description    string     `json:"description"`
	StartsAt       int64      `json:"starts_at"`
	EndsAt         int64      `json:"ends_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	LastReceivedAt time.Time  `json:"last_received_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RemoteAlertListData represents remote alert list response payload.
// RemoteAlertListData 表示远程告警列表响应数据。
type RemoteAlertListData struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Page        int                `json:"page"`
	PageSize    int                `json:"page_size"`
	Total       int64              `json:"total"`
	Alerts      []*RemoteAlertItem `json:"alerts"`
}

// ClusterHealthItem represents one cluster health summary for monitoring center.
// ClusterHealthItem 表示监控中心中的单集群健康摘要。
type ClusterHealthItem struct {
	ClusterID      uint   `json:"cluster_id"`
	ClusterName    string `json:"cluster_name"`
	Status         string `json:"status"`
	HealthStatus   string `json:"health_status"`
	TotalNodes     int    `json:"total_nodes"`
	OnlineNodes    int    `json:"online_nodes"`
	OfflineNodes   int    `json:"offline_nodes"`
	ActiveAlerts   int64  `json:"active_alerts"`
	CriticalAlerts int64  `json:"critical_alerts"`
}

// ClusterHealthData represents all clusters health payload.
// ClusterHealthData 表示全部集群健康响应数据。
type ClusterHealthData struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Total       int                  `json:"total"`
	Clusters    []*ClusterHealthItem `json:"clusters"`
}

// PlatformHealthData represents platform-level health summary.
// PlatformHealthData 表示平台级健康汇总。
type PlatformHealthData struct {
	GeneratedAt       time.Time `json:"generated_at"`
	HealthStatus      string    `json:"health_status"`
	TotalClusters     int       `json:"total_clusters"`
	HealthyClusters   int       `json:"healthy_clusters"`
	DegradedClusters  int       `json:"degraded_clusters"`
	UnhealthyClusters int       `json:"unhealthy_clusters"`
	UnknownClusters   int       `json:"unknown_clusters"`
	ActiveAlerts      int64     `json:"active_alerts"`
	CriticalAlerts    int64     `json:"critical_alerts"`
}
