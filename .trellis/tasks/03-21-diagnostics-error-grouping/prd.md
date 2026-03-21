# 优化 Seatunnel 错误组解析与连续错误合并

## Goal
减少 SeaTunnel 连续 ERROR 包装日志被拆成多个错误事件/错误组的问题，让 diagnostics 更稳定地聚合到真实根因。

## Requirements
- Agent 采集器应能将同一故障块的连续 ERROR/FATAL 行合并为一个错误事件
- 后端错误指纹应优先提取真实根因，而不是 Fatal Error / bug report 包装行
- 为 Invalid YAML configuration 这一类连续错误块补回归测试

## Acceptance Criteria
- [ ] 给定连续的 Fatal Error / Please submit bug report / Reason / Exception StackTrace 日志块，采集后只形成一个错误事件
- [ ] 该事件与相同根因的变体可落到同一个错误组
- [ ] 相关 agent/backend 定向测试通过

## Technical Notes
- 优先修改 agent/internal/diagnostics/error_collector.go
- 同步优化 internal/apps/diagnostics/normalize.go
