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
  Plugin,
  OfficialDependenciesResponse,
  PluginDependencyStatus,
} from '@/lib/services/plugin';

type TranslationFn = (key: string, values?: Record<string, string | number>) => string;

export interface DependencyStatusMeta {
  label: string;
  description: string;
  className: string;
}

function resolveStatusValues(
  status: PluginDependencyStatus | undefined,
  dependencyCount: number | undefined,
  baselineVersion: string | undefined,
) {
  return {
    status: status ?? 'unknown',
    count: dependencyCount ?? 0,
    baselineVersion: baselineVersion ?? '',
  };
}

export function getPluginDependencyStatusMeta(
  plugin: Pick<
    Plugin,
    'dependency_status' | 'dependency_count' | 'dependency_baseline_version'
  > & {
    name?: string;
    artifact_id?: string;
  },
  t: TranslationFn,
): DependencyStatusMeta {
  if (plugin.artifact_id === 'connector-jdbc' || plugin.name === 'jdbc') {
    return {
      label: t('plugin.dependencyStatus.profileRequired'),
      description: t('plugin.dependencyStatusDesc.profileRequired'),
      className:
        'border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-900 dark:bg-indigo-950/40 dark:text-indigo-300',
    };
  }

  const {status, count, baselineVersion} = resolveStatusValues(
    plugin.dependency_status,
    plugin.dependency_count,
    plugin.dependency_baseline_version,
  );

  switch (status) {
    case 'ready_exact':
      return {
        label: t('plugin.dependencyStatus.ready'),
        description: t('plugin.dependencyStatusDesc.ready', {count}),
        className: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300',
      };
    case 'ready_fallback':
      return {
        label: t('plugin.dependencyStatus.ready'),
        description: t('plugin.dependencyStatusDesc.readyFallback', {
          count,
          version: baselineVersion || '-',
        }),
        className: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300',
      };
    case 'runtime_analyzed':
      return {
        label: t('plugin.dependencyStatus.ready'),
        description: t('plugin.dependencyStatusDesc.readyRuntime', {count}),
        className: 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900 dark:bg-sky-950/40 dark:text-sky-300',
      };
    case 'not_required':
      return {
        label: t('plugin.dependencyStatus.notRequired'),
        description: t('plugin.dependencyStatusDesc.notRequired'),
        className: 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-300',
      };
    default:
      return {
        label: t('plugin.dependencyStatus.unknown'),
        description: t('plugin.dependencyStatusDesc.unknown'),
        className: 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300',
      };
  }
}

export function getOfficialDependencyStatusMeta(
  data: Pick<
    OfficialDependenciesResponse,
    'dependency_status' | 'dependency_count' | 'baseline_version_used'
  >,
  t: TranslationFn,
): DependencyStatusMeta {
  return getPluginDependencyStatusMeta(
    {
      dependency_status: data.dependency_status,
      dependency_count: data.dependency_count,
      dependency_baseline_version: data.baseline_version_used,
    },
    t,
  );
}
