# Design: Java Proxy 多集群部署模型

## Instance Boundary

Java Proxy 实例边界定义为：

```text
host_id / agent_id + install_dir
```

其中：

- `host_id / agent_id` 表示部署节点。
- `install_dir` 表示该节点上的 SeaTunnel runtime/classpath。
- `cluster_id` 是 Control Plane 选择和展示维度，但 Agent 侧实际进程隔离应以 `install_dir` 为准。

因此：

- 不同 host：天然隔离。
- 同 host + 不同 `install_dir`：不同 Java Proxy 实例，端口自动避让。
- 同 host + 同 `install_dir`：产品层不允许多个集群共享；卸载集群时无需跨集群引用计数，可强制清理该实例。

## Data Flow

```text
Control Plane config.yaml java_proxy.default_port
  -> backend cluster.Service attaches java_proxy_default_port
  -> Agent context default port
  -> managed java-proxy port search: default, default+1, default+2...

FE ClusterDetail
  -> cluster service API with node_id
  -> backend cluster.Service picks node + host agent
  -> Agent PRECHECK / service command with install_dir/version
  -> Agent managed java-proxy status/start/stop
  -> backend rewrites managed endpoint to node_host_ip:actual_port
  -> FE shows user-facing cluster proxy state and hides endpoints in diagnostics
```

## Default Port Contract

`java_proxy.default_port` in Control Plane `config.yaml` is the single source for the default starting port.

Rules:

- Control Plane reads `java_proxy.default_port` and sends it to Agent as `java_proxy_default_port`.
- Agent treats this value as the first port to try.
- If the port is occupied by another process or another `install_dir`, Agent tries `default_port + 1`, then keeps incrementing until it finds a free port.
- `service.port` remains the actual runtime port record and is used for status/direct endpoint display.
- Built-in `18080` remains only as a compatibility fallback when old deployments do not provide config.

## Ownership Validation

Agent 不能只以 `/healthz` 是否返回 OK 判断某端口是否属于当前集群，因为同机其他 `install_dir` 的 Java Proxy 也可能健康。

Agent 需要通过监听端口 PID 的命令行校验进程是否属于当前 `install_dir`：

```text
-Dseatunnelx.java.proxy.seatunnel.home=<install_dir>
```

规则：

- 端口健康且 PID 属于当前 `install_dir`：可复用。
- 端口健康但 PID 属于其他 `install_dir`：跳过该端口，启动时从 `default_port + 1` 开始递增分配新端口。
- 无法解析 PID：为兼容旧环境，不直接误杀；状态读取可保持保守，启动优先选择空闲端口。

## Process Safety

- `GetManagedSeatunnelXJavaProxyServiceStatus`：只把当前 `install_dir` 的 PID/端口标记为 Running/Healthy。
- `StopManagedSeatunnelXJavaProxyService`：只停止当前 `install_dir` 的 PID。
- `forceKillSeatunnelXJavaProxyProcesses`：端口发现到的 PID 必须再次校验 `install_dir`。
- `scripts/seatunnelx-java-proxy.sh`：同端口旧进程只有属于当前 `SEATUNNEL_HOME` 时才允许 kill，避免启动 B 集群时杀掉 A 集群。
- 集群删除/卸载：Control Plane 在停止 SeaTunnel 进程和删除安装目录前，先向对应节点下发 `service=seatunnelx_java_proxy, force=true, graceful=false` 的停止命令；Agent 走直接 `SIGKILL` 强制清理，并在删除安装目录前再次防御性强制停止一次。

## Endpoint Contract

后端 `SeatunnelXJavaProxyStatus` 保留：

- `local_endpoint`：Agent 本机看到的 endpoint，通常 `http://127.0.0.1:<port>`。
- `direct_endpoint`：Control Plane 直连 endpoint，`http://<node_host_ip>:<actual_port>`。
- `endpoint`：兼容字段，对 managed 实例等同于 `direct_endpoint`。

FE 文案必须强调“实际端口以状态返回为准”，不能固定写 18080。

## UI Contract

- 集群详情新增独立 Tab：中文名“集群代理”，英文名 “Cluster Proxy”。
- “运行时存储”页不再展示 Java Proxy 管理卡，只在 Java Proxy 未就绪时展示轻量跳转提示。
- 未安装状态只展示“安装 Java Proxy”主动作；点击后弹窗选择一个在线 Master / MasterWorker 节点。
- 已安装状态不展示部署节点下拉；仅展示部署节点、服务状态、端口、版本等用户能理解的摘要。
- endpoint、PID、日志路径等技术字段默认收进“诊断信息”折叠区域，不在主界面解释 Control Plane 直连实现。
- 不再将后端原始 message（如 `support assets installed or repaired`）直接展示给用户；操作反馈使用前端 i18n 文案。

## Compatibility

- 不改变已有一键安装资产目录：Java Proxy jar/script 仍安装到 Agent 支持目录，支持多个版本 jar 共存。
- 不改变已有 cluster/node API 主路径，只在 Java Proxy 子接口中传递可选 `node_id`。
- 老状态目录中的 `service.port` 如果指向其他 `install_dir` 的实例，会被状态读取识别为非本实例并在启动时避让。
