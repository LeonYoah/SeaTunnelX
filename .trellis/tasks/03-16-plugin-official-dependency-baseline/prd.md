# 插件官方依赖基线设计

## Goal
为插件市场补齐“官方 lib 依赖基线”能力：
- 连接器在线加载后，系统可自动给出官方推荐 lib
- 发版前自动产出初始化 SQL，随项目一起发布
- 用户运行中可一键重新分析当前版本官方文档并落库
- 官方基线与用户自定义依赖不冲突，升级时不覆盖用户修改

## Core Decisions
- 不维护 Python / Shell 抓取脚本，统一使用 **Go 后端实现抓取、解析、落库、导出 SQL**。
- 官方来源优先使用 **seatunnel-website versioned docs**（而不是渲染后的 HTML 页面，也不是 seatunnel 主仓库源码）。
- 运行时依赖来源分层：
  1. 官方基线（精确版本）
  2. 官方基线（版本回退）
  3. 用户一键分析结果
  4. 用户手工覆盖
- 用户点击“一键分析”时，前端直接调后端接口即可，不需要前端自己解析文档。
- 官方基线与用户覆盖分表存储，避免发版种子覆盖用户自定义依赖。

## Why This Way
当前系统能自动抓 SeaTunnel connector jar，但 lib 依赖仍靠手工配置，存在几个问题：
- Hive / Oracle / Oracle-CDC / HiveJdbc 等连接器依赖需要人工维护，体验割裂
- 依赖定义散落在官方文档，不适合让前端或用户自己理解并手填
- 新版本连接器可实时出现，官方依赖基线如果只靠代码发版更新，会有滞后
- 升级时若直接覆盖现有依赖表，会与用户手工补充产生冲突

因此需要建立“官方依赖基线 + 用户覆盖”的双层模型。

## Scope
### In Scope
1. 官方文档抓取与解析（Go 后端）
2. 官方依赖基线表设计
3. 初始化 SQL 导出能力
4. 运行时精确版本 / 版本回退 / 一键分析
5. 与现有插件依赖配置、插件下载、插件安装链路对接
6. 前端插件详情 / 依赖弹窗展示官方推荐依赖与来源状态

### Out of Scope
1. 全量 connector 一次性全部支持；MVP 先覆盖高价值场景
2. 自动写回 SeaTunnel 远端安装目录结构策略变更（仍沿用现有安装链路）
3. AI 自由推理依赖；本阶段以官方文档与模板规则为主

## MVP Coverage
首批支持：
- Oracle
- Oracle-CDC
- Hive
- HiveJdbc

原因：
- 官方 2.3.12 文档明确写了依赖要求
- 依赖价值高
- 解析规则相对稳定

## Data Model

### 1. 官方 profile 表
表名建议：`plugin_dependency_profiles`

字段：
- `id`
- `seatunnel_version`
- `plugin_name`
- `artifact_id`
- `profile_key`（如 `default`、`hive_on_s3`）
- `doc_slug`（如 `connector-v2/sink/Hive`）
- `doc_source_url`
- `engine_scope`（`zeta` / `spark` / `flink`）
- `target_dir`
- `is_default`
- `confidence`
- `source_kind`（`official_seed` / `runtime_analyzed`）
- `content_hash`
- `created_at`
- `updated_at`

联合唯一键建议：
- `(seatunnel_version, plugin_name, profile_key, engine_scope, source_kind)`

### 2. 官方 profile item 表
表名建议：`plugin_dependency_profile_items`

字段：
- `id`
- `profile_id`
- `group_id`
- `artifact_id`
- `version`
- `target_dir`
- `required`
- `source_url`
- `note`
- `created_at`
- `updated_at`

联合唯一键建议：
- `(profile_id, group_id, artifact_id, version, target_dir)`

### 3. 用户覆盖表
基于现有 `plugin_dependency_configs` 扩展：
- 增加 `seatunnel_version`
- 增加 `profile_key`
- 增加 `action`（`add` / `remove` / `override`）
- 增加 `source`（固定 `user`）

说明：
- 官方基线单独维护
- 用户修改始终写入用户覆盖表，不直接修改官方表

## Effective Dependency Resolution
最终实际生效依赖：
1. 官方基线 exact version
2. 官方基线 fallback version（同 minor 最近 patch）
3. runtime analyze 结果
4. 用户覆盖（add/remove/override）

优先级：
- 用户覆盖 > runtime analyze > official exact > official fallback

### Version Fallback
例如当前版本 `2.3.13` 无官方基线时：
1. 查 `2.3.13`
2. 查 `2.3.12`
3. 查同 major/minor 最近可用版本
4. 若仍无，则提示可“一键分析官方文档”

## Official Source and Parsing
### Source
统一抓取：
- `https://raw.githubusercontent.com/apache/seatunnel-website/main/versioned_docs/version-<version>/connector-v2/...`

### Why
- 官方版本化文档源稳定
- 比 HTML 更适合解析
- 能直接按 `2.3.12` 这类版本定位

### Parsing Layers
1. **高置信规则**
   - 提取 mvnrepository 链接
   - 提取 Maven 坐标
   - 提取显式 jar 文件名
2. **模板补全规则**
   - 文档只写 `ojdbc8` / `orai18n.jar` 时，用内置模板补 groupId / target_dir
3. **人工修正兜底**
   - 对 Hive 这类 groupId 不稳定场景，可人工修正规则后重新导出种子

### Profile Awareness
不能只按 connector 映射 lib，必须支持 profile：
- `connector-jdbc + oracle`
- `connector-jdbc + hivejdbc`
- `connector-hive + default`
- `connector-hive + hive_on_s3`

## Backend Design

### Internal Packages
建议新增模块：
- `internal/apps/plugindeps/`

职责：
- 拉取官方 markdown
- 解析 profile / item
- 落库
- 导出 seed SQL
- 计算 effective dependencies

### Export Entry
不单独维护脚本；统一使用 Go 后端入口，例如：
- 内部 service + admin command
- 或 `cmd/tools/plugin-deps-seed`（Go 命令，不是脚本）

发布前 CI / 本地发版流程只调用 Go 能力：
- 拉取版本化官方文档
- 生成/更新官方基线
- 导出初始化 SQL

## API Design

### 1. 查看官方依赖基线
- `GET /api/v1/plugins/:name/official-dependencies?version=2.3.12`

返回：
- exact / fallback 状态
- profile 列表
- item 列表
- 来源文档 URL

### 2. 一键分析官方文档
- `POST /api/v1/plugins/:name/official-dependencies/analyze`

请求：
- `version`
- `profile_key`（可选）
- `force_refresh`

行为：
- 后端抓官方文档
- 解析出依赖 profile
- 落为 `runtime_analyzed` 数据
- 返回 preview / 生效结果

### 3. 应用官方推荐到用户配置
- `POST /api/v1/plugins/:name/official-dependencies/apply`

行为：
- 将所选官方 profile 转成用户覆盖表记录
- 供下载/安装链路直接使用

### 4. 导出初始化 SQL（内部/管理接口，可选）
- `POST /api/v1/admin/plugin-dependency-seeds/export`

说明：
- 更推荐给内部命令/发布流程使用
- 不一定暴露给普通用户

## UI Flow
### 插件详情页 / 依赖配置弹窗
展示：
- 官方推荐依赖
- 来源：精确匹配 / 回退版本 / 运行时分析
- 来源文档链接
- 一键应用
- 一键重新分析

### 状态文案
- 官方依赖（精确匹配）
- 官方依赖（沿用 2.3.12）
- 暂无官方依赖，建议一键分析

## Seed SQL Strategy
### Canonical Source
真源不是 SQL，而是：
- 后端抓取并标准化后的 profile/item 数据

### Generated Artifacts
由 Go 导出：
- `support-files/sql/plugin_dependency_profiles.seed.sqlite.sql`
- `support-files/sql/plugin_dependency_profiles.seed.mysql.sql`
- `support-files/sql/plugin_dependency_profiles.seed.postgres.sql`

### Import Strategy
导入时只写官方基线表：
- 根据联合唯一键 upsert
- 更新官方 profile/item
- **不修改用户覆盖表**

这样用户升级时：
- 官方种子可更新
- 用户自定义不会被覆盖

## Conflict Rules
### Official vs User
- 官方基线只进官方表
- 用户修改只进用户表
- 生效计算阶段再 merge

### Seed Re-import
- 同一联合主键：更新官方记录
- 新 key：插入
- 已删除的官方 key：可标记失效，不直接删用户覆盖

## Acceptance Criteria
- [ ] 后端可从官方 versioned docs 解析 Hive / HiveJdbc / Oracle / Oracle-CDC 的依赖画像
- [ ] 可导出多数据库初始化 SQL
- [ ] 运行时可以按 exact / fallback 读取官方依赖
- [ ] 用户可通过接口一键分析当前版本官方文档
- [ ] 用户自定义依赖不会被官方种子覆盖
- [ ] 下载 / 安装插件链路可读取 effective dependencies
- [ ] 前端能展示官方推荐依赖与来源状态

## Open Questions
1. `plugin_dependency_configs` 是增强还是新建 `plugin_dependency_overrides` 更清晰？
2. profile 维度前端是否需要显式选择，还是只暴露 default profile？
3. Hive on S3 / OSS 这种环境增强依赖是否在 MVP 暴露，还是先只做 default？

## Recommended Next Step
先做一个设计优先的 MVP：
1. 新增官方基线表
2. 实现 2.3.12 的 4 个 profile 抓取解析
3. 打通 `GET official-dependencies` + `POST analyze`
4. 前端依赖弹窗展示官方推荐依赖


## UX Decisions Confirmed
- 插件市场默认按“一个插件 = connector + 系统自动附带 lib”心智设计
- lib 不完全隐藏，需要以轻状态在 UI 可见
- 对明确无依赖的连接器，显示“无需额外依赖”
- 对未识别出依赖的连接器，显示“依赖待确认，建议检查”
- 插件目录与官方依赖基线优先持久化到数据库，替代当前 24h 内存缓存
- 只有数据库里没有官方数据时，才在线抓取并分析官方文档
