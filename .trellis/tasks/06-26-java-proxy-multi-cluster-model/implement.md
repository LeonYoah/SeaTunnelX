# Implementation Plan

## Checklist

1. [x] Agent ownership helpers
   - Add install_dir-aware PID/cmdline matching helpers.
   - Reuse existing `seatunnelxJavaProxyPIDsByPort` and `/proc/<pid>/cmdline` parsing.

2. [x] Control Plane default port config
   - Add `java_proxy.default_port` to Control Plane `config.yaml`.
   - Pass `java_proxy_default_port` through Java Proxy lifecycle commands and runtime storage proxy commands.
   - Use the configured default in one-click Agent install wrappers.

3. [x] Agent managed service status/start/stop
   - Add `InstallDir` to `SeatunnelXJavaProxyServiceStatus` for internal ownership checks.
   - In status lookup, validate `service.port` and candidate ports against install_dir before marking healthy.
   - In start path, skip healthy ports owned by other install_dir and allocate fallback port by incrementing from the configured default.
   - In stop/force-kill path, refuse to kill processes that do not match current install_dir.

4. [x] Launcher script safety
   - Update `scripts/seatunnelx-java-proxy.sh` `kill_existing_proxy_listener` to kill only same `SEATUNNEL_HOME` Java Proxy process.

5. [x] Backend/FE endpoint clarity
   - Ensure direct endpoint uses returned actual port.
   - Update FE hint text away from fixed 18080 wording.

6. [x] Tests
   - Add Agent unit tests for ownership matching.
   - Verify non-reuse of other install_dir port through install_dir-aware start/status code paths.
   - Keep backend cluster tests for selected node and direct endpoint.
   - Run Go tests and TS/lint checks.

7. [x] Cluster uninstall cleanup
   - On cluster deletion, send a best-effort forced `seatunnelx_java_proxy` stop for each node install_dir before stopping SeaTunnel processes.
   - Treat `force=true` / `graceful=false` Java Proxy stop commands as direct `SIGKILL` cleanup on the Agent.
   - Ensure managed install directory removal force-stops Java Proxy after validating the directory is SeaTunnelX-managed.

8. [x] Cluster Proxy tab UX
   - Move Java Proxy management out of Runtime Storage into a standalone “集群代理 / Cluster Proxy” tab.
   - Show the Master node selector only in the install dialog when dependencies are missing.
   - Hide endpoints/PID/log path behind diagnostics and replace raw backend messages with i18n operation feedback.
   - Add `installed` status propagation so the UI can distinguish missing dependencies from stopped service.

## Validation Commands

```bash
go test ./internal/config ./internal/apps/agent ./internal/apps/cluster ./internal/apps/sync ./internal/router
(cd agent && go test ./internal/installer ./internal/executor ./cmd)
(cd frontend && pnpm exec tsc --noEmit)
(cd frontend && pnpm exec eslint components/common/cluster/ClusterDetail.tsx lib/services/cluster/cluster.service.ts lib/services/cluster/types.ts)
bash -n support-files/release/start.sh && bash -n support-files/release/status.sh && bash -n scripts/restart.sh && bash -n scripts/seatunnelx-java-proxy.sh
```

## Validation Results

- `go test ./internal/config ./internal/apps/agent ./internal/apps/cluster ./internal/apps/sync ./internal/router` ✅
- `(cd agent && go test ./internal/installer ./internal/executor ./cmd)` ✅
- `(cd frontend && pnpm exec tsc --noEmit)` ✅
- `(cd frontend && pnpm exec eslint components/common/cluster/ClusterDetail.tsx lib/services/cluster/cluster.service.ts lib/services/cluster/types.ts)` ✅
- `bash -n support-files/release/start.sh && bash -n support-files/release/status.sh && bash -n scripts/restart.sh && bash -n scripts/seatunnelx-java-proxy.sh` ✅
- 本机真实冲突验证：18080 已有 Java Proxy、18081 已有 SeaTunnel 监听时，临时 `install_dir` 从配置默认端口 18080 启动，最终递增选择 18082，测试结束后 18082 释放 ✅
- 删除集群清理验证：`go test ./internal/apps/cluster` 覆盖删除集群时先强制停止 Java Proxy，再停止 SeaTunnel，并在强删安装目录命令中携带 `java_proxy_default_port` ✅
- 集群代理 UI 验证：`cd frontend && pnpm exec eslint components/common/cluster/ClusterDetail.tsx lib/services/cluster/types.ts` ✅
- 集群代理类型验证：`cd frontend && pnpm exec tsc --noEmit` ✅
- 集群代理状态字段验证：`go test ./internal/apps/cluster && (cd agent && go test ./internal/installer ./cmd)` ✅
- `python3 ./.trellis/scripts/task.py validate .trellis/tasks/06-26-java-proxy-multi-cluster-model` ✅
- `git diff --check` ✅

## Risk / Rollback Points

- Process ownership detection depends on `/proc/<pid>/cmdline`; keep behavior conservative and avoid killing when ownership does not match.
- Do not change Java Proxy HTTP API or SeaTunnel runtime classpath assembly in this task.
