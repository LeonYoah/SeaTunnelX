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
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	monitoringapp "github.com/seatunnel/seatunnelX/internal/apps/monitoring"
	"github.com/seatunnel/seatunnelX/internal/pkg/schedulex"
)

const autoPolicyRequestedByPrefix = "auto-policy:"

// AutoPolicyChecker evaluates auto-inspection policies against incoming signals.
// AutoPolicyChecker 根据传入信号评估自动巡检策略。
type AutoPolicyChecker struct {
	repo    *Repository
	service *Service
}

// NewAutoPolicyChecker creates an auto-policy checker.
// NewAutoPolicyChecker 创建自动策略检查器。
func NewAutoPolicyChecker(repo *Repository, service *Service) *AutoPolicyChecker {
	return &AutoPolicyChecker{
		repo:    repo,
		service: service,
	}
}

// CheckJavaErrorTrigger checks whether a Java error event should trigger an auto-inspection.
// CheckJavaErrorTrigger 检查一条 Java 错误事件是否应触发自动巡检。
func (c *AutoPolicyChecker) CheckJavaErrorTrigger(ctx context.Context, clusterID uint, exceptionClass string, messageSnippet string) error {
	if c == nil || c.repo == nil || c.service == nil || clusterID == 0 {
		return nil
	}

	policies, err := c.repo.ListEnabledPoliciesForCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("auto-policy: failed to list policies for cluster %d: %w", clusterID, err)
	}
	if len(policies) == 0 {
		return nil
	}

	exceptionClassLower := strings.ToLower(strings.TrimSpace(exceptionClass))
	messageSnippetLower := strings.ToLower(strings.TrimSpace(messageSnippet))

	for _, policy := range policies {
		if policy == nil || !policy.Enabled {
			continue
		}
		for _, condition := range policy.Conditions {
			if !condition.Enabled {
				continue
			}
			matched, templateCode := c.matchJavaErrorCondition(condition, exceptionClassLower, messageSnippetLower)
			if !matched {
				continue
			}

			triggerReason := fmt.Sprintf("%s|%s: %s", c.policyReasonPrefix(policy.ID), templateCode, strings.TrimSpace(exceptionClass))
			triggered, err := c.triggerAutoPolicyInspection(ctx, clusterID, policy, triggerReason)
			if err != nil {
				return err
			}
			if triggered {
				return nil
			}
		}
	}
	return nil
}

// CheckScheduledPolicies checks all enabled schedule-based policies for one cluster.
// CheckScheduledPolicies 检查单个集群的所有定时巡检策略。
func (c *AutoPolicyChecker) CheckScheduledPolicies(ctx context.Context, clusterID uint, now time.Time) error {
	if c == nil || c.repo == nil || c.service == nil || clusterID == 0 {
		return nil
	}

	policies, err := c.repo.ListEnabledPoliciesForCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("auto-policy: failed to list scheduled policies for cluster %d: %w", clusterID, err)
	}

	now = now.UTC().Truncate(time.Minute)
	for _, policy := range policies {
		if policy == nil || !policy.Enabled {
			continue
		}
		for _, condition := range policy.Conditions {
			if !condition.Enabled || condition.TemplateCode != ConditionCodeScheduled {
				continue
			}
			matched, reason, err := c.matchScheduledCondition(policy, condition, now)
			if err != nil {
				log.Printf("[DiagnosticsAutoPolicy] schedule condition invalid: cluster_id=%d policy_id=%d err=%v", clusterID, policy.ID, err)
				continue
			}
			if !matched {
				continue
			}
			if _, err := c.triggerAutoPolicyInspection(ctx, clusterID, policy, reason); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// CheckPrometheusPolicies evaluates metric-signal policies through unified alert instances.
// CheckPrometheusPolicies 通过统一告警实例评估指标信号类策略。
func (c *AutoPolicyChecker) CheckPrometheusPolicies(ctx context.Context, clusterID uint, now time.Time) error {
	if c == nil || c.repo == nil || c.service == nil || clusterID == 0 {
		return nil
	}
	if c.service.monitoringService == nil {
		return nil
	}

	policies, err := c.repo.ListEnabledPoliciesForCluster(ctx, clusterID)
	if err != nil {
		return fmt.Errorf("auto-policy: failed to list metric-signal policies for cluster %d: %w", clusterID, err)
	}
	alerts, err := c.service.monitoringService.ListAlertInstances(ctx, &monitoringapp.AlertInstanceFilter{
		ClusterID: fmt.Sprintf("%d", clusterID),
		Page:      1,
		PageSize:  200,
		Status:    monitoringapp.AlertDisplayStatusFiring,
	})
	if err != nil {
		return fmt.Errorf("auto-policy: failed to list alert instances for cluster %d: %w", clusterID, err)
	}
	if alerts == nil || len(alerts.Alerts) == 0 {
		return nil
	}

	now = now.UTC()
	for _, policy := range policies {
		if policy == nil || !policy.Enabled {
			continue
		}
		for _, condition := range policy.Conditions {
			if !condition.Enabled {
				continue
			}
			matchedAlert, ok := matchMetricAlertCondition(condition, alerts.Alerts)
			if !ok {
				continue
			}
			reason := fmt.Sprintf("%s|%s alert=%s", c.policyReasonPrefix(policy.ID), condition.TemplateCode, normalizeMetricAlertReasonKey(matchedAlert))
			if _, err := c.triggerAutoPolicyInspection(ctx, clusterID, policy, reason); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (c *AutoPolicyChecker) matchJavaErrorCondition(condition InspectionConditionItem, exceptionClassLower, messageSnippetLower string) (bool, InspectionConditionTemplateCode) {
	code := condition.TemplateCode

	switch code {
	case ConditionCodeJavaOOM:
		if strings.Contains(exceptionClassLower, "outofmemoryerror") {
			return true, code
		}
	case ConditionCodeJavaStackOverflow:
		if strings.Contains(exceptionClassLower, "stackoverflowerror") {
			return true, code
		}
	case ConditionCodeJavaMetaspace:
		if strings.Contains(exceptionClassLower, "outofmemoryerror") && strings.Contains(messageSnippetLower, "metaspace") {
			return true, code
		}
	default:
	}

	if len(condition.ExtraKeywords) > 0 {
		for _, keyword := range condition.ExtraKeywords {
			kw := strings.ToLower(strings.TrimSpace(keyword))
			if kw == "" {
				continue
			}
			if strings.Contains(messageSnippetLower, kw) || strings.Contains(exceptionClassLower, kw) {
				return true, code
			}
		}
	}

	return false, ""
}

func (c *AutoPolicyChecker) matchScheduledCondition(policy *InspectionAutoPolicy, condition InspectionConditionItem, now time.Time) (bool, string, error) {
	expr, err := resolveScheduledCronExpression(condition)
	if err != nil {
		return false, "", err
	}
	matched, _, _, err := schedulex.MatchMinuteWindow(expr, now.UTC(), schedulex.DefaultTimezone)
	if err != nil {
		return false, "", err
	}
	if !matched {
		return false, "", nil
	}

	return true, fmt.Sprintf("%s|%s cron=%s", c.policyReasonPrefix(policy.ID), condition.TemplateCode, expr), nil
}

func (c *AutoPolicyChecker) triggerAutoPolicyInspection(ctx context.Context, clusterID uint, policy *InspectionAutoPolicy, triggerReason string) (bool, error) {
	if policy == nil {
		return false, nil
	}

	if c.isInCooldown(ctx, clusterID, policy.ID, policy.CooldownMinutes) {
		return false, nil
	}

	report, err := c.service.StartInspection(ctx, &StartClusterInspectionRequest{
		ClusterID:     clusterID,
		TriggerSource: InspectionTriggerSourceAuto,
	}, 0, c.policyRequestedBy(policy.ID))
	if err != nil {
		return false, fmt.Errorf("auto-policy: failed to start inspection for cluster %d (reason: %s): %w", clusterID, triggerReason, err)
	}

	if report != nil && report.Report != nil && report.Report.ID > 0 {
		trimmedReason := truncateString(strings.TrimSpace(triggerReason), 200)
		if err := c.repo.UpdateInspectionReportAutoTriggerReason(ctx, report.Report.ID, trimmedReason); err != nil {
			log.Printf("[DiagnosticsAutoPolicy] update auto trigger reason failed: cluster_id=%d policy_id=%d report_id=%d err=%v", clusterID, policy.ID, report.Report.ID, err)
		}
	}

	if policy.AutoCreateTask && report != nil && report.Report != nil {
		options := policy.TaskOptions.Normalize()
		_, taskErr := c.service.CreateDiagnosticTask(ctx, &CreateDiagnosticTaskRequest{
			ClusterID:     report.Report.ClusterID,
			TriggerSource: DiagnosticTaskSourceInspectionFinding,
			SourceRef: DiagnosticTaskSourceRef{
				InspectionReportID: report.Report.ID,
			},
			Options:   options,
			AutoStart: policy.AutoStartTask,
		}, 0, "auto-policy")
		if taskErr != nil {
			log.Printf("[DiagnosticsAutoPolicy] auto create diagnostic task failed: cluster_id=%d policy_id=%d report_id=%d err=%v", clusterID, policy.ID, report.Report.ID, taskErr)
		}
	}

	return true, nil
}

func (c *AutoPolicyChecker) isInCooldown(ctx context.Context, clusterID uint, policyID uint, cooldownMinutes int) bool {
	if cooldownMinutes <= 0 {
		cooldownMinutes = 30
	}
	lastReport, err := c.repo.GetLastInspectionReportForRequester(ctx, clusterID, c.policyRequestedBy(policyID))
	if err != nil {
		if errors.Is(err, ErrInspectionReportNotFound) {
			return false
		}
		return true
	}
	cooldownDeadline := time.Now().UTC().Add(-time.Duration(cooldownMinutes) * time.Minute)
	return lastReport.CreatedAt.After(cooldownDeadline)
}

func (c *AutoPolicyChecker) policyRequestedBy(policyID uint) string {
	return fmt.Sprintf("%s%d", autoPolicyRequestedByPrefix, policyID)
}

func (c *AutoPolicyChecker) policyReasonPrefix(policyID uint) string {
	return fmt.Sprintf("policy:%d", policyID)
}

func resolveScheduledCronExpression(condition InspectionConditionItem) (string, error) {
	expr := strings.TrimSpace(condition.CronExprOverride)
	if expr != "" {
		return expr, nil
	}
	template, ok := findBuiltinConditionTemplate(condition.TemplateCode)
	if !ok {
		return "", fmt.Errorf("unknown schedule template %s", condition.TemplateCode)
	}
	expr = strings.TrimSpace(template.DefaultCronExpr)
	if expr == "" {
		return "", fmt.Errorf("default cron expression is empty for %s", condition.TemplateCode)
	}
	return expr, nil
}

func matchMetricAlertCondition(condition InspectionConditionItem, alerts []*monitoringapp.AlertInstance) (*monitoringapp.AlertInstance, bool) {
	for _, alert := range alerts {
		if alert == nil || alert.Status != monitoringapp.AlertDisplayStatusFiring {
			continue
		}
		if metricAlertMatchesCondition(condition, alert) {
			return alert, true
		}
	}
	return nil, false
}

func metricAlertMatchesCondition(condition InspectionConditionItem, alert *monitoringapp.AlertInstance) bool {
	if alert == nil {
		return false
	}
	haystacks := []string{
		strings.ToLower(strings.TrimSpace(alert.RuleKey)),
		strings.ToLower(strings.TrimSpace(alert.AlertName)),
		strings.ToLower(strings.TrimSpace(alert.Summary)),
		strings.ToLower(strings.TrimSpace(alert.Description)),
	}

	switch condition.TemplateCode {
	case ConditionCodePromCPUHigh:
		return containsAnyMetricAlertAlias(haystacks, "cpu_usage_high", "cpu load high", "cpu high")
	case ConditionCodePromHeapHigh:
		return containsAnyMetricAlertAlias(haystacks, "memory_usage_high", "heap memory usage high", "heap usage high", "memory high")
	case ConditionCodePromGCFrequent:
		return containsAnyMetricAlertAlias(haystacks, "gc_frequent", "gc frequent", "gc time ratio high", "gc high")
	case ConditionCodePromHeapRising:
		return containsAnyMetricAlertAlias(haystacks, "heap_rising", "heap rising", "heap keeps rising", "memory rising")
	default:
		return false
	}
}

func containsAnyMetricAlertAlias(haystacks []string, aliases ...string) bool {
	for _, haystack := range haystacks {
		if haystack == "" {
			continue
		}
		for _, alias := range aliases {
			if alias != "" && strings.Contains(haystack, alias) {
				return true
			}
		}
	}
	return false
}

func normalizeMetricAlertReasonKey(alert *monitoringapp.AlertInstance) string {
	if alert == nil {
		return ""
	}
	if value := strings.TrimSpace(alert.RuleKey); value != "" {
		return value
	}
	if value := strings.TrimSpace(alert.AlertName); value != "" {
		return value
	}
	return strings.TrimSpace(alert.Summary)
}
