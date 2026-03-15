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
)

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
// It loads enabled policies for the cluster, matches java_error conditions, checks cooldown,
// and creates an inspection report if conditions are met.
// CheckJavaErrorTrigger 检查一条 Java 错误事件是否应触发自动巡检。
// 加载集群的已启用策略，匹配 java_error 条件，检查冷却时间，满足时创建巡检报告。
func (c *AutoPolicyChecker) CheckJavaErrorTrigger(ctx context.Context, clusterID uint, exceptionClass string, messageSnippet string) error {
	if c == nil || c.repo == nil || c.service == nil {
		return nil
	}
	if clusterID == 0 {
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

			// 检查冷却时间 Check cooldown period
			if c.isInCooldown(ctx, clusterID, policy.CooldownMinutes) {
				continue
			}

			// 触发巡检 Trigger inspection
			triggerReason := fmt.Sprintf("%s: %s", templateCode, strings.TrimSpace(exceptionClass))
			report, err := c.service.StartInspection(ctx, &StartClusterInspectionRequest{
				ClusterID:     clusterID,
				TriggerSource: InspectionTriggerSourceAuto,
			}, 0, fmt.Sprintf("auto-policy:%d", policy.ID))
			if err != nil {
				return fmt.Errorf("auto-policy: failed to start inspection for cluster %d (reason: %s): %w", clusterID, triggerReason, err)
			}

			// 更新最新报告的 AutoTriggerReason
			// Update the latest report's AutoTriggerReason
			c.setAutoTriggerReason(ctx, clusterID, triggerReason)

			// 如果策略要求自动生成诊断包，则为本次巡检自动创建诊断任务。
			// If the policy is configured to auto-create a diagnostics bundle, create a diagnostic task for this inspection.
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

			// 一个策略匹配到一个条件就触发，不重复触发
			// One match per policy is enough, avoid duplicate triggers
			return nil
		}
	}
	return nil
}

// matchJavaErrorCondition checks if a java_error condition matches.
// matchJavaErrorCondition 检查 java_error 条件是否匹配。
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
		// 非 java_error 类型条件在此不做匹配
		// Non java_error conditions are not matched here
	}

	// 检查额外关键字 Check extra keywords
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

// isInCooldown checks whether a recent inspection exists within the cooldown window.
// isInCooldown 检查冷却时间窗口内是否已有巡检。
func (c *AutoPolicyChecker) isInCooldown(ctx context.Context, clusterID uint, cooldownMinutes int) bool {
	if cooldownMinutes <= 0 {
		cooldownMinutes = 30
	}
	lastReport, err := c.repo.GetLastInspectionReportForCluster(ctx, clusterID)
	if err != nil {
		if errors.Is(err, ErrInspectionReportNotFound) {
			return false
		}
		// 查询失败时保守处理，不触发 If query fails, be conservative and don't trigger
		return true
	}
	cooldownDeadline := time.Now().UTC().Add(-time.Duration(cooldownMinutes) * time.Minute)
	return lastReport.CreatedAt.After(cooldownDeadline)
}

// setAutoTriggerReason updates the most recent inspection report with the trigger reason.
// setAutoTriggerReason 更新最近的巡检报告的自动触发原因。
func (c *AutoPolicyChecker) setAutoTriggerReason(ctx context.Context, clusterID uint, reason string) {
	lastReport, err := c.repo.GetLastInspectionReportForCluster(ctx, clusterID)
	if err != nil {
		return
	}
	lastReport.AutoTriggerReason = truncateString(reason, 200)
	_ = c.repo.UpdateInspectionReport(ctx, lastReport)
}

// CheckPrometheusTrigger is a stub for Prometheus metric condition checks.
// Will be implemented in a future iteration.
// CheckPrometheusTrigger 是 Prometheus 指标条件检查的桩函数。
// 将在后续迭代中实现。
func (c *AutoPolicyChecker) CheckPrometheusTrigger(ctx context.Context, clusterID uint) error {
	log.Printf("[DiagnosticsAutoPolicy] prometheus checker not yet implemented: cluster_id=%d", clusterID)
	return nil
}
