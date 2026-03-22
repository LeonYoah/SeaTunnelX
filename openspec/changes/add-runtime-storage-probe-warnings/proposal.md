## Why

SeaTunnelX 目前对 checkpoint 和 IMAP 的“运行时存储校验”仍然只做到本地路径就绪或远端端点可达，无法真实验证凭证、bucket、HDFS/对象存储读写权限是否可用。我们已经引入了 `seatunnel-capability-proxy` 并补齐了 one-shot CLI 能力，但还没有把这条能力接入实际安装链路。

与此同时，这类探测天然依赖真实的 `SEATUNNEL_HOME` 和运行时依赖。如果因为存储探测失败就直接阻塞安装，用户会失去继续完成安装、后续再排查存储问题的机会，不符合当前安装流程对“尽量完成主链路、把外围问题显式提示出来”的体验目标。

## What Changes

- 在安装流程中新增 ck/imap 远端运行时存储探测，使用 `seatunnel-capability-proxy` 的 one-shot CLI 真实调用 SeaTunnel 存储实现。
- 仅对 `HDFS`、`S3`、`OSS` 这类远端存储启用探测；`LOCAL_FILE` 与 `DISABLED` 继续沿用现有行为。
- 探测时机放在 Agent 端安装流程的 `extract` 之后、`configure_checkpoint` / `configure_imap` 步骤内部，确保已有真实 `SEATUNNEL_HOME`。
- 探测失败不阻塞安装，而是记录为安装 warning，并在控制面与前端安装进度里明确提示。
- 第一阶段不改造安装前的 `/installer/runtime-storage/validate` 页面逻辑，避免在 proxy 资产分发方案尚未完全稳定前引入“假 runtime 校验”。

## Capabilities

### New Capabilities

- `installer-runtime-storage-probe`: 安装链路中的远端 checkpoint / IMAP 真实运行时探测与 warning 透传。

### Modified Capabilities

- `installer-progress`: 安装状态新增 warning 聚合与展示。

## Impact

- Agent：新增 runtime storage probe 执行器，负责组装 one-shot request、调用 proxy 脚本并解析结果。
- 安装流程：`configure_checkpoint` 和 `configure_imap` 在写入配置前后增加非阻塞 runtime probe。
- 控制面：安装状态需要保留 warnings，轮询进度时从步骤消息中聚合 warning。
- 前端：安装进度页/向导需要展示 warning 列表，避免用户误以为探测失败被吞掉。
- 约束：当前阶段依赖 Agent 运行环境能访问 proxy 脚本和非 `-bin` jar；若资产不存在，必须退化为 warning 而不是安装失败。
