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

import fs from 'node:fs/promises';
import path from 'node:path';
import {expect, type APIRequestContext} from '@playwright/test';
import type {InstalledPlugin, LocalPlugin, PluginDependency} from '@/lib/services/plugin';

const backendBaseURL =
  process.env.E2E_BACKEND_BASE_URL ?? 'http://127.0.0.1:18000';
const tmpDirRoot =
  process.env.E2E_INSTALLER_REAL_TMP_DIR ??
  path.resolve(process.cwd(), '../tmp/e2e/installer-real');

interface ErrorResponse {
  error_msg?: string;
}

interface LocalPluginsResponse extends ErrorResponse {
  data?: LocalPlugin[];
}

interface InstalledPluginsResponse extends ErrorResponse {
  data?: InstalledPlugin[];
}

function normalizeTargetDir(targetDir: string): string {
  return targetDir.replace(/^[/\\]+/, '');
}

function getDependencyFileName(dep: PluginDependency): string {
  const artifact = dep.artifact_id?.trim() || 'dependency';
  const version = dep.version?.trim();
  return version ? `${artifact}-${version}.jar` : `${artifact}.jar`;
}

export function resolveLocalPluginDependencyPath(
  version: string,
  dependency: PluginDependency,
): string {
  return path.join(
    tmpDirRoot,
    'storage',
    'plugins',
    version,
    normalizeTargetDir(dependency.target_dir),
    getDependencyFileName(dependency),
  );
}

export async function waitForLocalPlugin(
  request: APIRequestContext,
  pluginName: string,
  version: string,
  predicate: (plugin: LocalPlugin) => boolean = () => true,
  timeoutMs: number = 300000,
): Promise<LocalPlugin> {
  await expect
    .poll(
      async () => {
        const response = await request.get(`${backendBaseURL}/api/v1/plugins/local`);
        expect(response.ok()).toBeTruthy();
        const payload = (await response.json()) as LocalPluginsResponse;
        const localPlugin = (payload.data || []).find(
          (plugin) =>
            plugin.name === pluginName &&
            plugin.version === version &&
            predicate(plugin),
        );
        return localPlugin ?? null;
      },
      {timeout: timeoutMs},
    )
    .not.toBeNull();

  const response = await request.get(`${backendBaseURL}/api/v1/plugins/local`);
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as LocalPluginsResponse;
  return (payload.data || []).find(
    (plugin) =>
      plugin.name === pluginName &&
      plugin.version === version &&
      predicate(plugin),
  ) as LocalPlugin;
}

async function waitForFile(pathname: string, timeoutMs: number = 30000): Promise<void> {
  await expect
    .poll(
      async () => {
        try {
          await fs.access(pathname);
          return true;
        } catch {
          return false;
        }
      },
      {timeout: timeoutMs},
    )
    .toBeTruthy();
}

export async function assertLocalPluginAssets(plugin: LocalPlugin): Promise<void> {
  await waitForFile(plugin.connector_path);
  for (const dependency of plugin.dependencies || []) {
    const dependencyPath = resolveLocalPluginDependencyPath(plugin.version, dependency);
    await waitForFile(dependencyPath);
  }
}

export async function installPluginToClusterApi(
  request: APIRequestContext,
  clusterId: number,
  pluginName: string,
  version: string,
  profileKeys?: string[],
): Promise<void> {
  const response = await request.post(
    `${backendBaseURL}/api/v1/clusters/${clusterId}/plugins`,
    {
      data: {
        plugin_name: pluginName,
        version,
        mirror: 'apache',
        profile_keys: profileKeys,
      },
    },
  );
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as ErrorResponse;
  expect(payload.error_msg ?? '').toBe('');
}

export async function waitForInstalledPlugin(
  request: APIRequestContext,
  clusterId: number,
  pluginName: string,
  version: string,
  predicate: (plugin: InstalledPlugin) => boolean = () => true,
  timeoutMs: number = 180000,
): Promise<InstalledPlugin> {
  await expect
    .poll(
      async () => {
        const response = await request.get(
          `${backendBaseURL}/api/v1/clusters/${clusterId}/plugins`,
        );
        expect(response.ok()).toBeTruthy();
        const payload = (await response.json()) as InstalledPluginsResponse;
        const installedPlugin = (payload.data || []).find(
          (plugin) =>
            plugin.plugin_name === pluginName &&
            plugin.version === version &&
            predicate(plugin),
        );
        return installedPlugin ?? null;
      },
      {timeout: timeoutMs},
    )
    .not.toBeNull();

  const response = await request.get(
    `${backendBaseURL}/api/v1/clusters/${clusterId}/plugins`,
  );
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as InstalledPluginsResponse;
  return (payload.data || []).find(
    (plugin) =>
      plugin.plugin_name === pluginName &&
      plugin.version === version &&
      predicate(plugin),
  ) as InstalledPlugin;
}
