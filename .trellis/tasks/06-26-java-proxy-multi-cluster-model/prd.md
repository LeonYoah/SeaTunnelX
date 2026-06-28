# Java Proxy 多集群部署模型

## Goal

让 SeaTunnelX Java Proxy 在多 SeaTunnel 集群、多节点、同机多安装目录场景下有明确且安全的实例边界：每个集群选择一个部署节点，Java Proxy 实例跟随该集群在该节点上的 SeaTunnel `install_dir`，Control Plane 通过节点 IP 与实际端口直连对应实例。

## Confirmed Facts

- Java Proxy 是依赖 SeaTunnel runtime/classpath 的辅助服务，不应被不同 SeaTunnel 版本或不同安装目录无条件全局复用。
- Agent 当前托管 Java Proxy 的状态目录来自 `install_dir/.seatunnelx/seatunnelx-java-proxy/`，天然适合做集群安装目录级隔离。
- 当前实现已经新增详情页“部署节点选择”和“安装/修复”入口，并把后端 API 扩展为支持 `node_id`。
- 当前实现已经把后端返回的 managed endpoint 改成 `http://<node_host_ip>:<port>` 供 Control Plane 直连，同时保留 Agent 本机 endpoint。
- 代码证据显示同机多集群存在一个风险：如果默认端口上已有健康 Java Proxy，`ensureSeatunnelXJavaProxyService` 可能在未校验 `install_dir` 所属权的情况下复用该端口实例。
- 默认起始端口统一由 Control Plane `config.yaml` 的 `java_proxy.default_port` 配置；Agent 以该值为起点，冲突时按 `+1` 递增寻找可用端口。
- 产品约束：不同集群不会共享同一 `host + install_dir`，因此删除/卸载某集群时可以按该实例边界强制清理对应 Java Proxy，无需做跨集群引用计数。
- UI 信息架构约束：Java Proxy 不只服务运行时存储，还会被工作台调试、安装期探测等能力复用，因此集群详情中应作为独立“集群代理”Tab 管理，而不是嵌在“运行时存储”页。

## Requirements

1. 多集群隔离：不同 SeaTunnel 集群如果在不同 `install_dir`，必须各自管理自己的 Java Proxy 实例，不得因为端口健康就误复用别的 `install_dir` 实例。
2. 同机端口避让：同一 host 上多个不同 `install_dir` 的 Java Proxy 不应互相杀进程；`java_proxy.default_port` 被其他 `install_dir` 占用时，应跳过并按 `default_port + 1`、`default_port + 2` 递增选择可用端口。
3. 同目录边界：系统不支持多个集群共享同一 host + 同一 `install_dir`；Agent 仍以 `install_dir` 做进程安全边界，卸载集群时强制清理该 `install_dir` 对应的 Java Proxy。
4. Control Plane 直连：后端对 managed Java Proxy 返回 `direct_endpoint=http://<node_host_ip>:<actual_port>`，实际端口以后端/Agent 状态为准，不在 UI 固定假设 18080。
5. 详情页反馈：详情页应明确展示部署节点、Control Plane 直连地址、Agent 本机地址和实际端口语义，避免用户误以为所有集群固定使用 18080。
6. 安全停止/重启：停止、重启、强杀、启动脚本中的清理逻辑只能作用于同一 `install_dir` 的 Java Proxy 进程，不得误杀同 host 上其他集群的 Java Proxy。
7. 配置统一：Control Plane 下发的 Java Proxy 默认端口必须来自 `config.yaml`，一键安装、Agent 命令和运行时存储代理命令都应携带该默认端口。
8. UI 归位：Java Proxy 管理入口放入独立“集群代理”Tab；未安装时才提示安装并弹窗选择 Master 节点；已安装页面不再展示部署节点下拉，不展示后端原始 message 和过多技术说明。
9. 回归测试：覆盖同机两个安装目录时端口占用不能被误复用、安装/修复命令能打到选定节点、direct endpoint 使用实际端口。

## Acceptance Criteria

- [ ] 当 `install_dir A` 的 Java Proxy 已占用 `java_proxy.default_port` 且健康时，启动 `install_dir B` 不会返回 A 的 endpoint，而是从 `default_port + 1` 开始递增分配/使用另一个可用端口。
- [ ] Control Plane `config.yaml` 中的 `java_proxy.default_port` 会随 Java Proxy 管理命令和运行时存储代理命令下发给 Agent。
- [ ] `GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)` 不会把不属于该 `installDir` 的 Java Proxy PID/端口标记为本实例健康。
- [ ] `StopManagedSeatunnelXJavaProxyService` 和强制 kill 路径不会杀掉不同 `installDir` 的 Java Proxy。
- [ ] 删除/卸载集群时，会在停止 SeaTunnel 进程和删除安装目录前 best-effort 强制 kill 该 `install_dir` 对应的 Java Proxy。
- [ ] `scripts/seatunnelx-java-proxy.sh` 只清理当前 `SEATUNNEL_HOME` 对应的同端口旧实例，不清理其他集群实例。
- [ ] 前端文案不固定宣称 18080，而是说明 Control Plane 直连所选节点的实际端口。
- [ ] 集群详情存在独立“集群代理”Tab；运行时存储页只提示依赖状态并跳转，不承载 Java Proxy 管理卡。
- [ ] 未安装状态通过弹窗选择在线 Master 节点安装 Java Proxy 依赖；已安装状态不展示部署节点选择器。
- [ ] Go/Agent/前端类型检查和相关测试通过。

## Out of Scope

- 不新增数据库表持久化 Java Proxy 实例映射；本任务保持以 `install_dir` 状态目录为单一来源。
- 不实现多节点高可用 Java Proxy 自动漂移；本任务仍以用户选择的一个部署节点为准。
- 不实现跨网络/防火墙探测；如果 Control Plane 不能访问节点 IP:port，由后续连通性校验任务处理。
