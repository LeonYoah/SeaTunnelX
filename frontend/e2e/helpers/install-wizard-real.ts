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
import os from 'node:os';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import {expect, type Page} from '@playwright/test';

const backendBaseURL =
  process.env.E2E_BACKEND_BASE_URL ?? 'http://127.0.0.1:18000';
const repoRoot = path.resolve(process.cwd(), '..');
const execFileAsync = promisify(execFile);

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

interface ClusterListResponse {
  data?: {
    clusters?: Array<{
      id?: number | string;
      name?: string;
      status?: string;
      install_dir?: string;
    }>;
  };
}

interface RuntimeStorageResponse {
  data?: {
    checkpoint?: {
      enabled?: boolean;
      endpoint?: string;
      bucket?: string;
      namespace?: string;
    };
    imap?: {
      enabled?: boolean;
      endpoint?: string;
      bucket?: string;
      namespace?: string;
    };
  };
}

interface SeatunnelXJavaProxyStatusResponse {
  data?: {
    managed?: boolean;
    running?: boolean;
    healthy?: boolean;
    pid?: number;
    endpoint?: string;
  };
}

interface RuntimeStorageValidationResponse {
  data?: {
    success?: boolean;
    warning?: string;
    hosts?: Array<{
      success?: boolean;
      message?: string;
    }>;
  };
}

interface RuntimeStorageListResponse {
  data?: {
    path?: string;
    items?: Array<{
      path?: string;
      name?: string;
      directory?: boolean;
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

export async function waitForClusterByInstallDir(
  page: Page,
  installDir: string,
  timeoutMs: number = 180000,
): Promise<{id: number; status: string; name: string}> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    const response = await page.context().request.get(`${backendBaseURL}/api/v1/clusters`, {
      params: {
        current: '1',
        size: '100',
      },
    });
    if (response.ok()) {
      const payload = (await response.json()) as ClusterListResponse;
      const cluster = (payload?.data?.clusters ?? []).find(
        (item) => item?.install_dir === installDir,
      );
      if (cluster?.id) {
        return {
          id: Number(cluster.id),
          status: String(cluster.status || 'unknown'),
          name: String(cluster.name || `Cluster-${cluster.id}`),
        };
      }
    }
    await page.waitForTimeout(2000);
  }

  throw new Error(`Timed out waiting for cluster with install_dir=${installDir}`);
}

export async function ensureClusterRunning(
  page: Page,
  clusterId: number,
  timeoutMs: number = 180000,
): Promise<void> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    const response = await page.context().request.get(
      `${backendBaseURL}/api/v1/clusters/${clusterId}`,
    );
    if (response.ok()) {
      const payload = (await response.json()) as {
        data?: {status?: string};
      };
      const status = payload?.data?.status;
      if (status === 'running') {
        return;
      }
      if (status === 'installed' || status === 'stopped' || status === 'unknown') {
        await page.context().request.post(
          `${backendBaseURL}/api/v1/clusters/${clusterId}/start`,
        );
      }
    }
    await page.waitForTimeout(3000);
  }

  throw new Error(`Timed out waiting for cluster ${clusterId} to become running`);
}

export async function waitForRuntimeStorageReady(
  page: Page,
  clusterId: number,
  timeoutMs: number = 180000,
): Promise<NonNullable<RuntimeStorageResponse['data']>> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    const response = await page.context().request.get(
      `${backendBaseURL}/api/v1/clusters/${clusterId}/runtime-storage`,
    );
    if (response.ok()) {
      const payload = (await response.json()) as RuntimeStorageResponse;
      if (payload?.data?.checkpoint?.namespace || payload?.data?.imap?.namespace) {
        return payload.data!;
      }
    }
    await page.waitForTimeout(3000);
  }

  throw new Error(`Timed out waiting for cluster ${clusterId} runtime storage`);
}

export async function waitForSeatunnelXJavaProxyHealthy(
  page: Page,
  clusterId: number,
  timeoutMs: number = 180000,
): Promise<NonNullable<SeatunnelXJavaProxyStatusResponse['data']>> {
  const startedAt = Date.now();

  while (Date.now() - startedAt < timeoutMs) {
    const response = await page.context().request.get(
      `${backendBaseURL}/api/v1/clusters/${clusterId}/seatunnelx-java-proxy/status`,
    );
    if (response.ok()) {
      const payload = (await response.json()) as SeatunnelXJavaProxyStatusResponse;
      if (payload?.data?.managed && payload?.data?.running && payload?.data?.healthy) {
        return payload.data!;
      }
    }
    await page.context().request.post(
      `${backendBaseURL}/api/v1/clusters/${clusterId}/seatunnelx-java-proxy/start`,
    );
    await page.waitForTimeout(3000);
  }

  throw new Error(`Timed out waiting for cluster ${clusterId} seatunnelx-java-proxy`);
}

export async function validateClusterRuntimeStorage(
  page: Page,
  clusterId: number,
  kind: 'checkpoint' | 'imap',
): Promise<NonNullable<RuntimeStorageValidationResponse['data']>> {
  const response = await page.context().request.post(
    `${backendBaseURL}/api/v1/clusters/${clusterId}/runtime-storage/${kind}/validate`,
  );
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as RuntimeStorageValidationResponse;
  expect(payload?.data?.success).toBeTruthy();
  expect(
    (payload?.data?.hosts ?? []).every((host) => host.success),
    JSON.stringify(payload?.data),
  ).toBeTruthy();
  return payload.data!;
}

export async function listClusterRuntimeStorage(
  page: Page,
  clusterId: number,
  kind: 'checkpoint' | 'imap',
  path: string,
): Promise<NonNullable<RuntimeStorageListResponse['data']>> {
  const response = await page.context().request.post(
    `${backendBaseURL}/api/v1/clusters/${clusterId}/runtime-storage/${kind}/list`,
    {
      data: {
        path,
      },
    },
  );
  expect(response.ok()).toBeTruthy();
  const payload = (await response.json()) as RuntimeStorageListResponse;
  expect(payload?.data?.path).toBeTruthy();
  expect(Array.isArray(payload?.data?.items ?? [])).toBeTruthy();
  return payload.data!;
}

function resolveSeatunnelXJavaProxyAssets(version: string) {
  return {
    script: path.join(repoRoot, 'scripts', 'seatunnelx-java-proxy.sh'),
    jar: path.join(repoRoot, 'lib', `seatunnelx-java-proxy-${version}.jar`),
    home: repoRoot,
  };
}

export async function expectSeatunnelXJavaProxyProbeSuccess(options: {
  installDir: string;
  version: string;
  kind: 'checkpoint' | 'imap';
  request: Record<string, unknown>;
}) {
  const {script, jar, home} = resolveSeatunnelXJavaProxyAssets(options.version);
  await fs.access(script);
  await fs.access(jar);

  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'stx-java-proxy-e2e-'));
  const requestFile = path.join(tempDir, `${options.kind}.request.json`);
  const responseFile = path.join(tempDir, `${options.kind}.response.json`);
  await fs.writeFile(requestFile, JSON.stringify(options.request, null, 2), 'utf8');

  try {
    await execFileAsync(
      'bash',
      [
        script,
        'probe-once',
        options.kind,
        '--request-file',
        requestFile,
        '--response-file',
        responseFile,
      ],
      {
        env: {
          ...process.env,
          SEATUNNEL_HOME: options.installDir,
          SEATUNNELX_JAVA_PROXY_HOME: home,
          SEATUNNEL_PROXY_JAR: jar,
          SEATUNNELX_JAVA_PROXY_VERSION: options.version,
        },
      },
    );
    const payload = JSON.parse(await fs.readFile(responseFile, 'utf8')) as {
      ok?: boolean;
      writable?: boolean;
      readable?: boolean;
      message?: string;
    };
    expect(payload?.ok, JSON.stringify(payload)).toBeTruthy();
    expect(payload?.writable, JSON.stringify(payload)).toBeTruthy();
    expect(payload?.readable, JSON.stringify(payload)).toBeTruthy();
    return payload;
  } finally {
    await fs.rm(tempDir, {recursive: true, force: true});
  }
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
