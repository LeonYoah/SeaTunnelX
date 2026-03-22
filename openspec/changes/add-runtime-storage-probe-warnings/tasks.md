## 1. 规格与范围

- [x] 1.1 明确第一阶段只做安装链路内真实 runtime probe，不直接替换安装前 validate 页面
- [x] 1.2 明确 probe 失败只 warning、不阻塞安装

## 2. Agent 安装链路

- [x] 2.1 新增 checkpoint / IMAP runtime probe request builder，复用现有存储配置映射
- [x] 2.2 新增 proxy 资产发现与 one-shot CLI 执行逻辑，支持 request/response JSON 文件交换
- [x] 2.3 在 `configure_checkpoint` 步骤中接入远端存储 runtime probe，失败仅 warning
- [x] 2.4 在 `configure_imap` 步骤中接入远端存储 runtime probe，失败仅 warning
- [x] 2.5 为 proxy 缺失、probe 失败、probe 成功场景补充 Agent 单测

## 3. 控制面与前端展示

- [x] 3.1 在安装状态模型中新增 `warnings` 字段
- [x] 3.2 在安装状态轮询中聚合 warning message，并保留去重后的 warnings
- [x] 3.3 在安装向导/进度页展示 warnings

## 4. 验证

- [x] 4.1 验证远端 checkpoint probe 失败后安装仍能继续
- [x] 4.2 验证远端 IMAP probe 失败后安装仍能继续
- [x] 4.3 验证 proxy 资产缺失时用户能看到明确 warning
