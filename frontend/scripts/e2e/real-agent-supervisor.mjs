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

import http from 'node:http';
import process from 'node:process';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawn} from 'node:child_process';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '../../..');
const backendBaseURL =
  process.env.E2E_BACKEND_BASE_URL ?? 'http://127.0.0.1:18000';
const supervisorPort = Number(process.env.E2E_AGENT_SUPERVISOR_PORT || 18181);
const goBin = process.env.GO_BIN || 'go';
const agentConfigPath =
  process.env.E2E_AGENT_REAL_CONFIG_PATH ??
  path.resolve(repoRoot, 'config.e2e.agent-real.yaml');

let shuttingDown = false;
let backendReady = false;
let agentStarted = false;
let agentExited = false;
let lastError = '';

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitForBackendHealth(timeoutMs = 120000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch(`${backendBaseURL}/api/v1/health`);
      if (response.ok) {
        backendReady = true;
        return;
      }
    } catch {
      // Ignore until timeout.
    }
    await sleep(1000);
  }
  throw new Error(`backend did not become healthy within ${timeoutMs}ms`);
}

function startHealthServer() {
  const server = http.createServer((_req, res) => {
    if (backendReady && agentStarted && !agentExited) {
      res.writeHead(200, {'Content-Type': 'application/json'});
      res.end(JSON.stringify({status: 'ok'}));
      return;
    }
    res.writeHead(503, {'Content-Type': 'application/json'});
    res.end(
      JSON.stringify({
        status: 'starting',
        backendReady,
        agentStarted,
        agentExited,
        lastError,
      }),
    );
  });

  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(supervisorPort, '127.0.0.1', () => resolve(server));
  });
}

async function main() {
  const server = await startHealthServer();

  try {
    await waitForBackendHealth();

    const agentChild = spawn(
      goBin,
      ['run', './cmd', '--config', agentConfigPath],
      {
        cwd: path.join(repoRoot, 'agent'),
        env: {
          ...process.env,
          AGENT_LOG_FILE: path.join(
            repoRoot,
            'tmp/e2e/installer-real/logs/seatunnelx-agent.log',
          ),
        },
        stdio: 'inherit',
      },
    );

    agentStarted = true;

    agentChild.once('exit', (code, signal) => {
      agentExited = true;
      if (!shuttingDown) {
        lastError = `agent exited unexpectedly (code=${code ?? 'null'}, signal=${signal ?? 'null'})`;
      }
    });

    agentChild.once('error', (error) => {
      agentExited = true;
      lastError = error.message;
    });

    const shutdown = async () => {
      if (shuttingDown) {
        return;
      }
      shuttingDown = true;
      if (!agentExited) {
        agentChild.kill('SIGTERM');
        await sleep(1500);
        if (!agentExited) {
          agentChild.kill('SIGKILL');
        }
      }
      await new Promise((resolve) => server.close(resolve));
      process.exit(0);
    };

    process.on('SIGINT', shutdown);
    process.on('SIGTERM', shutdown);

    // Give the agent a short window to register before Playwright starts.
    await sleep(4000);
  } catch (error) {
    lastError = error instanceof Error ? error.message : String(error);
    process.stderr.write(`[installer-real-e2e] ${lastError}\n`);
    process.exitCode = 1;
  }
}

await main();
