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
 * SeaTunnel Upgrade Session Helpers
 * SeaTunnel 升级会话辅助方法
 */

import type {
  CreatePlanRequest,
  PrecheckResult,
  UpgradePlanRecord,
  UpgradeTask,
} from '@/lib/services/st-upgrade';
import {sanitizePrecheckResult, sanitizeUpgradePlanRecord} from '@/lib/services/st-upgrade';

export interface StUpgradeSessionState {
  clusterId: number;
  request?: CreatePlanRequest;
  precheck?: PrecheckResult;
  plan?: UpgradePlanRecord;
  task?: UpgradeTask;
}

function sanitizeSessionState(state: StUpgradeSessionState): StUpgradeSessionState {
  return {
    ...state,
    precheck: sanitizePrecheckResult(state.precheck) || undefined,
    plan: sanitizeUpgradePlanRecord(state.plan) || undefined,
  };
}

function getStorageKey(clusterId: number): string {
  return `st-upgrade-session:${clusterId}`;
}

export function loadStUpgradeSession(clusterId: number): StUpgradeSessionState | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const raw = window.sessionStorage.getItem(getStorageKey(clusterId));
  if (!raw) {
    return null;
  }
  try {
    return sanitizeSessionState(JSON.parse(raw) as StUpgradeSessionState);
  } catch {
    window.sessionStorage.removeItem(getStorageKey(clusterId));
    return null;
  }
}

export function saveStUpgradeSession(clusterId: number, state: StUpgradeSessionState): void {
  if (typeof window === 'undefined') {
    return;
  }
  window.sessionStorage.setItem(getStorageKey(clusterId), JSON.stringify(sanitizeSessionState(state)));
}

export function patchStUpgradeSession(clusterId: number, patch: Partial<StUpgradeSessionState>): StUpgradeSessionState {
  const current = loadStUpgradeSession(clusterId) ?? {clusterId};
  const next = sanitizeSessionState({
    ...current,
    ...patch,
  });
  saveStUpgradeSession(clusterId, next);
  return next;
}

export function clearStUpgradeSession(clusterId: number): void {
  if (typeof window === 'undefined') {
    return;
  }
  window.sessionStorage.removeItem(getStorageKey(clusterId));
}
