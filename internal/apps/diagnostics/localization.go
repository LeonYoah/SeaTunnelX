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
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DiagnosticLanguage string

const (
	DiagnosticLanguageZH DiagnosticLanguage = "zh"
	DiagnosticLanguageEN DiagnosticLanguage = "en"
)

type diagnosticLocalizedText struct {
	ZH string
	EN string
}

type diagnosticLocalizedPattern struct {
	pattern *regexp.Regexp
	build   func(parts []string) diagnosticLocalizedText
}

var diagnosticExactTextMap = map[string]diagnosticLocalizedText{
	"Error Center": {
		ZH: "错误中心",
		EN: "Error Center",
	},
	"Track structured Seatunnel ERROR groups and related context.": {
		ZH: "追踪结构化 Seatunnel ERROR 分组及其关联上下文。",
		EN: "Track structured Seatunnel ERROR groups and related context.",
	},
	"Inspections": {
		ZH: "巡检中心",
		EN: "Inspections",
	},
	"Run and review cluster inspections based on managed runtime signals.": {
		ZH: "基于受管运行时信号发起并查看集群巡检。",
		EN: "Run and review cluster inspections based on managed runtime signals.",
	},
	"Error Evidence": {
		ZH: "错误证据",
		EN: "Error Evidence",
	},
	"Diagnostics owns Seatunnel ERROR evidence and links to cluster / alert context.": {
		ZH: "诊断中心负责维护 Seatunnel ERROR 证据，并关联集群 / 告警上下文。",
		EN: "Diagnostics owns Seatunnel ERROR evidence and links to cluster / alert context.",
	},
	"Inspection Signals": {
		ZH: "巡检信号",
		EN: "Inspection Signals",
	},
	"Diagnostics consumes monitoring, process events, and alert signals for inspections.": {
		ZH: "诊断中心消费监控、进程事件与告警信号用于巡检。",
		EN: "Diagnostics consumes monitoring, process events, and alert signals for inspections.",
	},
	"汇总错误上下文": {
		ZH: "汇总错误上下文",
		EN: "Collect Error Context",
	},
	"加载错误组、巡检结果和来源上下文。": {
		ZH: "加载错误组、巡检结果和来源上下文。",
		EN: "Load error groups, inspection results, and source context.",
	},
	"收集进程事件": {
		ZH: "收集进程事件",
		EN: "Collect Process Events",
	},
	"采集近期进程事件和自动拉起记录。": {
		ZH: "采集近期进程事件和自动拉起记录。",
		EN: "Collect recent process events and auto-restart history.",
	},
	"收集告警快照": {
		ZH: "收集告警快照",
		EN: "Collect Alert Snapshot",
	},
	"采集相关告警状态与通知上下文。": {
		ZH: "采集相关告警状态与通知上下文。",
		EN: "Collect related alert states and notification context.",
	},
	"收集配置快照": {
		ZH: "收集配置快照",
		EN: "Collect Config Snapshot",
	},
	"导出 Seatunnel 与相关运行配置快照。": {
		ZH: "导出 Seatunnel 与相关运行配置快照。",
		EN: "Export Seatunnel runtime config snapshots and related settings.",
	},
	"收集日志样本": {
		ZH: "收集日志样本",
		EN: "Collect Log Sample",
	},
	"采集错误附近日志样本和近期运行日志片段。": {
		ZH: "采集错误附近日志样本和近期运行日志片段。",
		EN: "Collect nearby log samples and recent runtime log fragments.",
	},
	"收集线程栈": {
		ZH: "收集线程栈",
		EN: "Collect Thread Dump",
	},
	"对选中节点执行线程栈采集。": {
		ZH: "对选中节点执行线程栈采集。",
		EN: "Collect thread dumps on selected nodes.",
	},
	"收集 JVM Dump": {
		ZH: "收集 JVM Dump",
		EN: "Collect JVM Dump",
	},
	"对选中节点执行 JVM Dump 采集。": {
		ZH: "对选中节点执行 JVM Dump 采集。",
		EN: "Collect JVM dumps on selected nodes.",
	},
	"生成 Manifest": {
		ZH: "生成 Manifest",
		EN: "Assemble Manifest",
	},
	"生成机器可读的诊断证据清单。": {
		ZH: "生成机器可读的诊断证据清单。",
		EN: "Generate a machine-readable diagnostic evidence manifest.",
	},
	"生成诊断报告": {
		ZH: "生成诊断报告",
		EN: "Render Diagnostic Report",
	},
	"渲染 index.html 诊断报告，便于离线查看与分享。": {
		ZH: "渲染 index.html 诊断报告，便于离线查看与分享。",
		EN: "Render the offline diagnostic report for viewing and sharing.",
	},
	"完成": {
		ZH: "完成",
		EN: "Complete",
	},
	"标记诊断任务完成并输出入口索引。": {
		ZH: "标记诊断任务完成并输出入口索引。",
		EN: "Mark the diagnostic task as completed and publish the entry index.",
	},
	"Offline Node": {
		ZH: "离线节点",
		EN: "Offline Node",
	},
	"Restart Failure": {
		ZH: "重启失败",
		EN: "Restart Failure",
	},
	"Recent Error Burst": {
		ZH: "近期错误突增",
		EN: "Recent Error Burst",
	},
	"Active Alert": {
		ZH: "活动告警",
		EN: "Active Alert",
	},
	"Check host heartbeat, agent status, and SeaTunnel process state before retrying operations.": {
		ZH: "在重试操作前，先检查主机心跳、Agent 状态与 SeaTunnel 进程状态。",
		EN: "Check host heartbeat, agent status, and SeaTunnel process state before retrying operations.",
	},
	"Review node event history and runtime logs, then verify startup command and dependencies.": {
		ZH: "查看节点事件历史与运行日志，并核对启动命令和依赖是否正常。",
		EN: "Review node event history and runtime logs, then verify startup command and dependencies.",
	},
	"Open the error center to inspect grouped evidence and identify the underlying dependency or runtime issue.": {
		ZH: "打开错误中心查看分组证据，定位底层依赖或运行时问题。",
		EN: "Open the error center to inspect grouped evidence and identify the underlying dependency or runtime issue.",
	},
	"Review the alert detail and linked runtime evidence before performing restart or scale actions.": {
		ZH: "在执行重启或扩缩容前，先查看告警详情与关联运行时证据。",
		EN: "Review the alert detail and linked runtime evidence before performing restart or scale actions.",
	},
	"COLLECT_ERROR_CONTEXT": {
		ZH: "汇总错误上下文",
		EN: "Collect Error Context",
	},
	"COLLECT_PROCESS_EVENTS": {
		ZH: "收集进程事件",
		EN: "Collect Process Events",
	},
	"COLLECT_ALERT_SNAPSHOT": {
		ZH: "收集告警快照",
		EN: "Collect Alert Snapshot",
	},
	"COLLECT_CONFIG_SNAPSHOT": {
		ZH: "收集配置快照",
		EN: "Collect Config Snapshot",
	},
	"COLLECT_LOG_SAMPLE": {
		ZH: "收集日志样本",
		EN: "Collect Log Sample",
	},
	"COLLECT_THREAD_DUMP": {
		ZH: "收集线程栈",
		EN: "Collect Thread Dump",
	},
	"COLLECT_JVM_DUMP": {
		ZH: "收集 JVM Dump",
		EN: "Collect JVM Dump",
	},
	"ASSEMBLE_MANIFEST": {
		ZH: "生成 Manifest",
		EN: "Assemble Manifest",
	},
	"RENDER_HTML_SUMMARY": {
		ZH: "生成诊断报告",
		EN: "Render Diagnostic Report",
	},
	"COMPLETE": {
		ZH: "完成",
		EN: "Complete",
	},
}

var diagnosticLocalizedPatterns = []diagnosticLocalizedPattern{
	{
		pattern: regexp.MustCompile(`^Diagnostic bundle created from error group #(\d+)$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "错误组 #" + parts[0] + " 触发的诊断包",
				EN: "Diagnostic bundle created from error group #" + parts[0],
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Diagnostic bundle created from inspection finding #(\d+)$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "巡检发现 #" + parts[0] + " 触发的诊断包",
				EN: "Diagnostic bundle created from inspection finding #" + parts[0],
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Diagnostic bundle created from alert (.+)$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "告警 " + parts[0] + " 触发的诊断包",
				EN: "Diagnostic bundle created from alert " + parts[0],
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Node (.+) \((.+)\) is offline$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "节点 " + parts[0] + "（" + parts[1] + "）离线",
				EN: "Node " + parts[0] + " (" + parts[1] + ") is offline",
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Recent (.+) detected for process (.+)$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "近期检测到进程 " + parts[1] + " 出现 " + parts[0],
				EN: "Recent " + parts[0] + " detected for process " + parts[1],
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Recent error group "(.+)" occurred (\d+) times in the last (\d+) minutes$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "近期错误组“" + parts[0] + "”在最近 " + parts[2] + " 分钟内出现 " + parts[1] + " 次",
				EN: `Recent error group "` + parts[0] + `" occurred ` + parts[1] + ` times in the last ` + parts[2] + ` minutes`,
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Active (.+) alert: (.+)$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "活动 " + parts[0] + " 告警：" + parts[1],
				EN: "Active " + parts[0] + " alert: " + parts[1],
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^Cluster (\d+) inspection failed$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: "集群 " + parts[0] + " 巡检失败",
				EN: "Cluster " + parts[0] + " inspection failed",
			}
		},
	},
	{
		pattern: regexp.MustCompile(`^(.+) inspection for the last (\d+) minutes generated (\d+) findings \((\d+) critical / (\d+) warning / (\d+) info\)$`),
		build: func(parts []string) diagnosticLocalizedText {
			return diagnosticLocalizedText{
				ZH: parts[0] + " 在最近 " + parts[1] + " 分钟巡检中生成 " + parts[2] + " 条发现（严重 " + parts[3] + " / 告警 " + parts[4] + " / 信息 " + parts[5] + "）",
				EN: parts[0] + " inspection for the last " + parts[1] + " minutes generated " + parts[2] + " findings (" + parts[3] + " critical / " + parts[4] + " warning / " + parts[5] + " info)",
			}
		},
	},
}

func normalizeDiagnosticLanguage(value string) DiagnosticLanguage {
	if strings.EqualFold(strings.TrimSpace(value), string(DiagnosticLanguageEN)) {
		return DiagnosticLanguageEN
	}
	return DiagnosticLanguageZH
}

func chooseDiagnosticLocalizedText(text diagnosticLocalizedText, lang DiagnosticLanguage) string {
	if lang == DiagnosticLanguageEN {
		return firstNonEmptyString(strings.TrimSpace(text.EN), strings.TrimSpace(text.ZH))
	}
	return firstNonEmptyString(strings.TrimSpace(text.ZH), strings.TrimSpace(text.EN))
}

func localizeDiagnosticText(value string, lang DiagnosticLanguage) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	if zh, en, ok := splitDiagnosticLocalizedText(trimmed); ok {
		return chooseDiagnosticLocalizedText(diagnosticLocalizedText{ZH: zh, EN: en}, lang)
	}
	if text, ok := diagnosticExactTextMap[trimmed]; ok {
		return chooseDiagnosticLocalizedText(text, lang)
	}
	for _, item := range diagnosticLocalizedPatterns {
		parts := item.pattern.FindStringSubmatch(trimmed)
		if len(parts) == 0 {
			continue
		}
		return chooseDiagnosticLocalizedText(item.build(parts[1:]), lang)
	}
	return trimmed
}

func localizeWorkspaceBootstrapData(data *WorkspaceBootstrapData, lang DiagnosticLanguage) *WorkspaceBootstrapData {
	if data == nil {
		return nil
	}
	copyData := *data
	copyData.Tabs = make([]*WorkspaceTab, 0, len(data.Tabs))
	for _, item := range data.Tabs {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Label = localizeDiagnosticText(item.Label, lang)
		copyItem.Description = localizeDiagnosticText(item.Description, lang)
		copyData.Tabs = append(copyData.Tabs, &copyItem)
	}
	copyData.Boundaries = make([]*WorkspaceBoundary, 0, len(data.Boundaries))
	for _, item := range data.Boundaries {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Title = localizeDiagnosticText(item.Title, lang)
		copyItem.Description = localizeDiagnosticText(item.Description, lang)
		copyData.Boundaries = append(copyData.Boundaries, &copyItem)
	}
	if data.EntryContext != nil {
		contextCopy := *data.EntryContext
		contextCopy.Source = localizeDiagnosticText(data.EntryContext.Source, lang)
		copyData.EntryContext = &contextCopy
	}
	if data.ClusterOptions != nil {
		copyData.ClusterOptions = append([]*ClusterOption(nil), data.ClusterOptions...)
	}
	return &copyData
}

func localizeInspectionReportsData(data *ClusterInspectionReportsData, lang DiagnosticLanguage) *ClusterInspectionReportsData {
	if data == nil {
		return nil
	}
	copyData := *data
	copyData.Items = make([]*ClusterInspectionReportInfo, 0, len(data.Items))
	for _, item := range data.Items {
		copyData.Items = append(copyData.Items, localizeInspectionReportInfo(item, lang))
	}
	return &copyData
}

func localizeInspectionReportDetailData(data *ClusterInspectionReportDetailData, lang DiagnosticLanguage) *ClusterInspectionReportDetailData {
	if data == nil {
		return nil
	}
	copyData := *data
	copyData.Report = localizeInspectionReportInfo(data.Report, lang)
	copyData.Findings = make([]*ClusterInspectionFindingInfo, 0, len(data.Findings))
	for _, item := range data.Findings {
		copyData.Findings = append(copyData.Findings, localizeInspectionFindingInfo(item, lang))
	}
	copyData.RelatedDiagnosticTask = localizeDiagnosticTask(data.RelatedDiagnosticTask, lang)
	return &copyData
}

func localizeInspectionReportInfo(info *ClusterInspectionReportInfo, lang DiagnosticLanguage) *ClusterInspectionReportInfo {
	if info == nil {
		return nil
	}
	copyInfo := *info
	copyInfo.Summary = localizeDiagnosticText(info.Summary, lang)
	copyInfo.ErrorMessage = localizeDiagnosticText(info.ErrorMessage, lang)
	copyInfo.AutoTriggerReason = localizeDiagnosticText(info.AutoTriggerReason, lang)
	return &copyInfo
}

func localizeInspectionFindingInfo(info *ClusterInspectionFindingInfo, lang DiagnosticLanguage) *ClusterInspectionFindingInfo {
	if info == nil {
		return nil
	}
	copyInfo := *info
	copyInfo.CheckName = localizeDiagnosticText(info.CheckName, lang)
	copyInfo.Summary = localizeDiagnosticText(info.Summary, lang)
	copyInfo.Recommendation = localizeDiagnosticText(info.Recommendation, lang)
	copyInfo.EvidenceSummary = localizeDiagnosticText(info.EvidenceSummary, lang)
	return &copyInfo
}

func localizeDiagnosticTaskListData(data *DiagnosticTaskListData, lang DiagnosticLanguage) *DiagnosticTaskListData {
	if data == nil {
		return nil
	}
	copyData := *data
	copyData.Items = make([]*DiagnosticTaskSummary, 0, len(data.Items))
	for _, item := range data.Items {
		copyData.Items = append(copyData.Items, localizeDiagnosticTaskSummary(item, lang))
	}
	return &copyData
}

func localizeDiagnosticTaskSteps(items []*DiagnosticTaskStep, lang DiagnosticLanguage) []*DiagnosticTaskStep {
	if len(items) == 0 {
		return []*DiagnosticTaskStep{}
	}
	result := make([]*DiagnosticTaskStep, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Title = localizeDiagnosticText(item.Title, lang)
		copyItem.Description = localizeDiagnosticText(item.Description, lang)
		copyItem.Message = localizeDiagnosticText(item.Message, lang)
		copyItem.Error = localizeDiagnosticText(item.Error, lang)
		result = append(result, &copyItem)
	}
	return result
}

func localizeDiagnosticTaskSummary(item *DiagnosticTaskSummary, lang DiagnosticLanguage) *DiagnosticTaskSummary {
	if item == nil {
		return nil
	}
	copyItem := *item
	copyItem.Summary = localizeDiagnosticText(item.Summary, lang)
	copyItem.FailureReason = localizeDiagnosticText(item.FailureReason, lang)
	return &copyItem
}

func localizeDiagnosticTask(task *DiagnosticTask, lang DiagnosticLanguage) *DiagnosticTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	copyTask.Summary = localizeDiagnosticText(task.Summary, lang)
	copyTask.FailureReason = localizeDiagnosticText(task.FailureReason, lang)
	copyTask.IndexPath = resolveDiagnosticLocalizedHTMLPath(task.IndexPath, lang)
	copyTask.SelectedNodes = append(DiagnosticTaskNodeTargets{}, task.SelectedNodes...)
	copyTask.Steps = make([]DiagnosticTaskStep, 0, len(task.Steps))
	for _, item := range task.Steps {
		copyItem := item
		copyItem.Title = localizeDiagnosticText(item.Title, lang)
		copyItem.Description = localizeDiagnosticText(item.Description, lang)
		copyItem.Message = localizeDiagnosticText(item.Message, lang)
		copyItem.Error = localizeDiagnosticText(item.Error, lang)
		copyTask.Steps = append(copyTask.Steps, copyItem)
	}
	copyTask.NodeExecutions = make([]DiagnosticNodeExecution, 0, len(task.NodeExecutions))
	for _, item := range task.NodeExecutions {
		copyItem := item
		copyItem.Message = localizeDiagnosticText(item.Message, lang)
		copyItem.Error = localizeDiagnosticText(item.Error, lang)
		copyTask.NodeExecutions = append(copyTask.NodeExecutions, copyItem)
	}
	return &copyTask
}

func localizeBuiltinConditionTemplates(templates []*InspectionConditionTemplate, lang DiagnosticLanguage) []*InspectionConditionTemplate {
	if len(templates) == 0 {
		return []*InspectionConditionTemplate{}
	}
	result := make([]*InspectionConditionTemplate, 0, len(templates))
	for _, item := range templates {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Name = localizeDiagnosticText(item.Name, lang)
		copyItem.Description = localizeDiagnosticText(item.Description, lang)
		result = append(result, &copyItem)
	}
	return result
}

func localizeDiagnosticTaskLogListData(data *DiagnosticTaskLogListData, lang DiagnosticLanguage) *DiagnosticTaskLogListData {
	if data == nil {
		return nil
	}
	copyData := *data
	copyData.Items = make([]*DiagnosticStepLog, 0, len(data.Items))
	for _, item := range data.Items {
		if item == nil {
			continue
		}
		copyItem := *item
		copyItem.Message = localizeDiagnosticText(item.Message, lang)
		copyData.Items = append(copyData.Items, &copyItem)
	}
	return &copyData
}

func localizeDiagnosticTaskEvent(event DiagnosticTaskEvent, lang DiagnosticLanguage) DiagnosticTaskEvent {
	event.Message = localizeDiagnosticText(event.Message, lang)
	event.Error = localizeDiagnosticText(event.Error, lang)
	event.FailureReason = localizeDiagnosticText(event.FailureReason, lang)
	return event
}

func resolveDiagnosticLocalizedHTMLPath(path string, lang DiagnosticLanguage) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return trimmed
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	if ext != ".html" {
		return trimmed
	}
	base := strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
	base = strings.TrimSuffix(base, ".zh")
	base = strings.TrimSuffix(base, ".en")
	switch lang {
	case DiagnosticLanguageEN:
		candidate := base + ".en" + filepath.Ext(trimmed)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	case DiagnosticLanguageZH:
		candidate := base + ".zh" + filepath.Ext(trimmed)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return trimmed
}
