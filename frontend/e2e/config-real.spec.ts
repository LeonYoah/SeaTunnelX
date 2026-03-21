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
import {expect, test} from '@playwright/test';
import {
  resolveInstalledConfigPaths,
  waitForFileContent,
} from './helpers/install-wizard-real';
import {
  initClusterConfigsFromNode,
  openClusterConfigsTab,
  openTemplateConfigEditor,
  selectClusterConfigType,
  syncTemplateConfigToAllNodes,
  waitForFileToContain,
} from './helpers/config-real';
import {installSourceCluster} from './helpers/upgrade-real';

const seatunnelVersion = process.env.E2E_INSTALLER_REAL_VERSION ?? '2.3.13';
const tmpDirRoot =
  process.env.E2E_INSTALLER_REAL_TMP_DIR ??
  path.resolve(process.cwd(), '../tmp/e2e/installer-real');
const installDir = path.join(
  tmpDirRoot,
  'install',
  `config-real-seatunnel-${seatunnelVersion}`,
);
const clusterPort = Number(
  process.env.E2E_INSTALLER_REAL_CLUSTER_PORT_PRIMARY || '38181',
);
const httpPort = Number(
  process.env.E2E_INSTALLER_REAL_HTTP_PORT_PRIMARY || '38080',
);

test.describe.serial('config real e2e', () => {
  test('manages global cluster configs with smart repair, sync, history and live import', async ({
    page,
  }) => {
    test.slow();
    console.log(
      `[config-real] installing real source cluster ${seatunnelVersion} at ${installDir}`,
    );

    const cluster = await installSourceCluster(page, {
      sourceVersion: seatunnelVersion,
      installDir,
      clusterPort,
      httpPort,
    });
    const files = resolveInstalledConfigPaths(installDir);
    const repairedNamespace = '/tmp/seatunnel/checkpoint-config-real/';
    const importedNamespace = '/tmp/seatunnel/checkpoint-imported-from-node/';

    await openClusterConfigsTab(page, cluster.clusterId);
    await selectClusterConfigType(page, /SeaTunnel/i);
    await initClusterConfigsFromNode(page);
    console.log('[config-real] cluster configs initialized from node');

    await openTemplateConfigEditor(page);
    const brokenSeatunnelConfig = `seatunnel:
    engine:
        http:
            enable-http: true
            port: ${httpPort}
        checkpoint:
            interval: 10000
      storage:
        type: hdfs
        plugin-config:
          namespace: ${repairedNamespace}
          storage.type: hdfs
          fs.defaultFS: file:///
`;

    const editContent = page.getByTestId('cluster-configs-edit-content');
    await editContent.fill(brokenSeatunnelConfig);
    await page.getByTestId('cluster-configs-smart-repair').click();
    await expect(editContent).toHaveValue(new RegExp(repairedNamespace));
    await expect(editContent).toHaveValue(/checkpoint:\n\s+interval: 10000\n\s+storage:/);
    console.log('[config-real] smart repair normalized seatunnel.yaml');

    await page.getByTestId('cluster-configs-save-only').click();
    await expect(page.getByTestId('cluster-configs-edit-dialog')).toHaveCount(0);
    await expect(page.getByTestId('cluster-configs-pending-sync')).toContainText(
      /1/,
    );
    console.log('[config-real] template saved without node sync');

    await syncTemplateConfigToAllNodes(page);
    await waitForFileToContain(files.seatunnel, [
      `namespace: ${repairedNamespace}`,
      `port: ${httpPort}`,
    ], 60000);
    await openClusterConfigsTab(page, cluster.clusterId);
    await selectClusterConfigType(page, /SeaTunnel/i);
    await expect(page.getByTestId('cluster-configs-pending-sync')).toHaveCount(0, {
      timeout: 30000,
    });
    console.log('[config-real] template synced to node and file updated');

    await page.getByTestId('cluster-configs-template-versions').click();
    await expect(page.getByTestId('cluster-configs-versions-dialog')).toBeVisible(
      {timeout: 30000},
    );
    await expect(
      page.getByTestId('cluster-configs-version-preview-content'),
    ).toContainText(repairedNamespace);
    console.log('[config-real] version preview verified');

    const compareButton = page
      .locator('[data-testid^="cluster-configs-version-compare-"]:not([disabled])')
      .first();
    await expect(compareButton).toBeVisible({timeout: 30000});
    await compareButton.click();
    await expect(
      page.getByTestId('cluster-configs-version-compare-content'),
    ).toContainText('/tmp/seatunnel/checkpoint/');
    await expect(
      page.getByTestId('cluster-configs-version-compare-content'),
    ).toContainText(repairedNamespace);
    console.log('[config-real] version compare verified');

    const rollbackButton = page
      .locator('[data-testid^="cluster-configs-version-rollback-"]')
      .first();
    await expect(rollbackButton).toBeVisible({timeout: 30000});
    await rollbackButton.click();
    await expect(page.getByTestId('cluster-configs-versions-dialog')).toHaveCount(
      0,
      {timeout: 30000},
    );
    await expect(page.getByTestId('cluster-configs-pending-sync')).toBeVisible({
      timeout: 30000,
    });
    await syncTemplateConfigToAllNodes(page);
    await waitForFileToContain(files.seatunnel, [
      'namespace: /tmp/seatunnel/checkpoint/',
    ], 60000);
    await openClusterConfigsTab(page, cluster.clusterId);
    await selectClusterConfigType(page, /SeaTunnel/i);
    await expect(page.getByTestId('cluster-configs-pending-sync')).toHaveCount(0, {
      timeout: 30000,
    });
    console.log('[config-real] rollback synced back to node');

    const liveSeatunnelConfig = await waitForFileContent(files.seatunnel);
    const importedSeatunnelConfig = liveSeatunnelConfig.replace(
      '/tmp/seatunnel/checkpoint/',
      importedNamespace,
    );
    expect(importedSeatunnelConfig).not.toBe(liveSeatunnelConfig);
    await fs.writeFile(files.seatunnel, importedSeatunnelConfig, 'utf8');
    console.log('[config-real] live seatunnel.yaml modified out-of-band');

    await initClusterConfigsFromNode(page);
    await expect(page.getByTestId('cluster-configs-template-content')).toContainText(
      importedNamespace,
      {timeout: 60000},
    );
    console.log('[config-real] init from node refreshed template from live file');
  });
});
