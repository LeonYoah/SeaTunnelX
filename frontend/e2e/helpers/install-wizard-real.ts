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
import {expect, type Page} from '@playwright/test';

const backendBaseURL =
  process.env.E2E_BACKEND_BASE_URL ?? 'http://127.0.0.1:18000';

export interface OnlineHostFixture {
  id: number;
  name: string;
  ipAddress: string;
}

interface HostListResponse {
  data?: {
    hosts?: Array<{
      id?: number | string;
      name?: string;
      ip_address?: string;
      is_online?: boolean;
      agent_status?: string;
    }>;
  };
}

export async function waitForOnlineHost(
  page: Page,
  timeoutMs: number = 120000,
): Promise<OnlineHostFixture> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    const response = await page.context().request.get(
      `${backendBaseURL}/api/v1/hosts`,
    );
    if (response.ok()) {
      const payload = (await response.json()) as HostListResponse;
      const hosts = payload?.data?.hosts ?? [];
      const onlineHost = hosts.find(
        (host) => host?.is_online && host?.agent_status === 'installed',
      );
      if (onlineHost?.id) {
        return {
          id: Number(onlineHost.id),
          name: String(onlineHost.name || `Host-${onlineHost.id}`),
          ipAddress: String(onlineHost.ip_address || '127.0.0.1'),
        };
      }
    }
    await page.waitForTimeout(2000);
  }

  throw new Error('Timed out waiting for an online agent-backed host');
}

export function buildInstallWizardLabURL(options: {
  hostId: number;
  hostName: string;
  version: string;
  installDir: string;
  clusterPort: number;
  httpPort: number;
}): string {
  const params = new URLSearchParams({
    hostId: String(options.hostId),
    hostName: options.hostName,
    initialVersion: options.version,
    initialInstallDir: options.installDir,
    initialClusterPort: String(options.clusterPort),
    initialHttpPort: String(options.httpPort),
  });
  return `/e2e-lab/install-wizard?${params.toString()}`;
}

export async function chooseSelectOption(
  page: Page,
  testId: string,
  optionName: RegExp | string,
): Promise<void> {
  await page.getByTestId(testId).click();
  await page.getByRole('option', {name: optionName}).click();
}

export async function waitForFileContent(
  filePath: string,
  timeoutMs: number = 120000,
): Promise<string> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    try {
      return await fs.readFile(filePath, 'utf8');
    } catch {
      // Ignore until timeout.
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }

  throw new Error(`Timed out waiting for file: ${filePath}`);
}

export async function expectInstallationSuccess(page: Page): Promise<void> {
  await expect(
    page.getByTestId('install-wizard-step-complete'),
  ).toBeVisible({timeout: 900000});
  await expect(page.getByTestId('install-complete-result')).toContainText(
    /安装成功|Installation Success/i,
    {timeout: 900000},
  );
}

export async function assertFileContains(
  filePath: string,
  snippets: string[],
): Promise<void> {
  const content = await waitForFileContent(filePath);
  for (const snippet of snippets) {
    expect(content).toContain(snippet);
  }
}

export function resolveInstalledConfigPaths(installDir: string) {
  const configDir = path.join(installDir, 'config');
  return {
    seatunnel: path.join(configDir, 'seatunnel.yaml'),
    hazelcast: path.join(configDir, 'hazelcast.yaml'),
    hazelcastClient: path.join(configDir, 'hazelcast-client.yaml'),
    log4j2: path.join(configDir, 'log4j2.properties'),
  };
}
