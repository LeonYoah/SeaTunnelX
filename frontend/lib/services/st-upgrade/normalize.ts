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

import type {
  ConfigMergeFile,
  ConfigMergePlan,
  PrecheckResult,
  TaskListData,
  TaskLogsData,
  TaskStepsData,
  UpgradePlanRecord,
  UpgradeStepLog,
  UpgradeTask,
  UpgradeTaskStep,
  UpgradeTaskSummary,
  UpgradeNodeExecution,
} from './types';
import {getCurrentLocale, type Locale} from '@/lib/i18n/config';
import {localizeBackendText} from '@/lib/i18n/localize-text';

export function localizeUpgradeText(
  value?: string | null,
  locale: Locale = getCurrentLocale(),
): string {
  return localizeBackendText(value, locale);
}

function sanitizeConfigMergeFile(file: ConfigMergeFile): ConfigMergeFile {
  const conflicts = Array.isArray(file.conflicts) ? file.conflicts : [];

  return {
    ...file,
    conflicts,
  };
}

export function sanitizeConfigMergePlan(plan?: ConfigMergePlan | null): ConfigMergePlan | null {
  if (!plan) {
    return null;
  }

  const files = Array.isArray(plan.files) ? plan.files.map(sanitizeConfigMergeFile) : [];

  return {
    ...plan,
    files,
  };
}

export function sanitizePrecheckResult(precheck?: PrecheckResult | null): PrecheckResult | null {
  if (!precheck) {
    return null;
  }

  return {
    ...precheck,
    issues: Array.isArray(precheck.issues)
      ? precheck.issues.map((issue) => ({
          ...issue,
          message: localizeUpgradeText(issue.message),
        }))
      : [],
    node_targets: Array.isArray(precheck.node_targets) ? precheck.node_targets : [],
    config_merge_plan: sanitizeConfigMergePlan(precheck.config_merge_plan) || precheck.config_merge_plan,
  };
}

export function sanitizeUpgradePlanRecord(plan?: UpgradePlanRecord | null): UpgradePlanRecord | null {
  if (!plan) {
    return null;
  }

  return {
    ...plan,
    snapshot: {
      ...plan.snapshot,
      node_targets: Array.isArray(plan.snapshot.node_targets) ? plan.snapshot.node_targets : [],
      steps: Array.isArray(plan.snapshot.steps) ? plan.snapshot.steps : [],
      config_merge_plan: sanitizeConfigMergePlan(plan.snapshot.config_merge_plan) || plan.snapshot.config_merge_plan,
    },
  };
}

function sanitizeTaskStep(step: UpgradeTaskStep): UpgradeTaskStep {
  return {
    ...step,
    message: localizeUpgradeText(step.message),
    error: localizeUpgradeText(step.error),
  };
}

function sanitizeNodeExecution(node: UpgradeNodeExecution): UpgradeNodeExecution {
  return {
    ...node,
    message: localizeUpgradeText(node.message),
    error: localizeUpgradeText(node.error),
  };
}

export function sanitizeUpgradeTask(task?: UpgradeTask | null): UpgradeTask | null {
  if (!task) {
    return null;
  }

  return {
    ...task,
    failure_reason: localizeUpgradeText(task.failure_reason),
    rollback_reason: localizeUpgradeText(task.rollback_reason),
    plan: sanitizeUpgradePlanRecord(task.plan) || task.plan,
    steps: Array.isArray(task.steps) ? task.steps.map(sanitizeTaskStep) : [],
    node_executions: Array.isArray(task.node_executions)
      ? task.node_executions.map(sanitizeNodeExecution)
      : [],
  };
}

function sanitizeUpgradeTaskSummary(
  task: UpgradeTaskSummary,
): UpgradeTaskSummary {
  return {
    ...task,
    failure_reason: localizeUpgradeText(task.failure_reason),
    rollback_reason: localizeUpgradeText(task.rollback_reason),
  };
}

export function sanitizeTaskListData(data?: TaskListData | null): TaskListData | null {
  if (!data) {
    return null;
  }

  return {
    ...data,
    items: Array.isArray(data.items)
      ? data.items.map(sanitizeUpgradeTaskSummary)
      : [],
  };
}

export function sanitizeTaskStepsData(data?: TaskStepsData | null): TaskStepsData | null {
  if (!data) {
    return null;
  }

  return {
    ...data,
    steps: Array.isArray(data.steps) ? data.steps.map(sanitizeTaskStep) : [],
    node_executions: Array.isArray(data.node_executions)
      ? data.node_executions.map(sanitizeNodeExecution)
      : [],
  };
}

function sanitizeUpgradeStepLog(log: UpgradeStepLog): UpgradeStepLog {
  return {
    ...log,
    message: localizeUpgradeText(log.message),
  };
}

export function sanitizeTaskLogsData(data?: TaskLogsData | null): TaskLogsData | null {
  if (!data) {
    return null;
  }

  return {
    ...data,
    items: Array.isArray(data.items)
      ? data.items.map(sanitizeUpgradeStepLog)
      : [],
  };
}
