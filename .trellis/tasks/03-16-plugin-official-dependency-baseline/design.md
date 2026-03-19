# 插件官方依赖基线设计稿

## 1. 产品目标
让用户在“插件市场”里看到的仍然只是一个插件，但系统在后台自动处理：
- connector jar
- 官方推荐 lib 依赖
- 新版本的依赖回退
- 无依赖或缺依赖时的轻提示

目标体验：
1. 用户浏览插件时，不需要理解 Maven 坐标和 jar 放置目录
2. 用户安装插件时，默认自动安装 connector + effective libs
3. 只有在高级场景下，用户才进入依赖详情 / 手工修正

---

## 2. UX 设计原则

### 2.1 默认无感
默认用户认知：
- 一个插件 = connector + 系统自动附带的依赖

因此前端默认不展示复杂的 lib 编辑表单，不把用户逼到“先学依赖再安装插件”。

### 2.2 轻状态可见
考虑到有些连接器确实没有依赖，lib 不能彻底隐藏，应当在 UI 中体现轻状态：
- 自动附带依赖已就绪
- 依赖已按标准模板准备
- 无需额外依赖
- 未识别官方依赖，建议检查

### 2.3 高级入口后置
只有当用户点开插件详情 / 依赖详情时，才展示：
- 自动附带的连接器 / 依赖清单
- 实际安装目标目录
- 用户自定义依赖

说明：
- 前端不再暴露“分析官方依赖”按钮
- `/plugins/:name/dependencies` 仅用于用户自定义依赖，不联网
- 内部 exact / fallback / baseline 版本信息不直接暴露给终端用户

---

## 3. 页面层设计

## 3.1 插件市场列表页 `/plugins`
### 卡片新增轻状态区
每个插件卡片增加一个依赖状态 badge：

状态枚举：
- `ready_exact`
  - 文案：`自动依赖已就绪`
- `ready_fallback`
  - 文案：`自动依赖已就绪`
- `not_required`
  - 文案：`无需额外依赖`
- `unknown`
  - 文案：`依赖待确认`
- `runtime_analyzed`
  - 文案：`自动依赖已就绪`

特殊场景：
- `jdbc` 不展示“已就绪/待确认”，而是提示 `需选择数据源场景`

### 次级说明
次级文本只保留用户可理解的信息：
- 自动附带连接器数量
- 额外依赖数量
- 是否需要选择场景

例：
- `自动附带 1 个连接器、2 个依赖`
- `无需额外依赖`
- `请选择数据源场景`

### 列表页不直接展示
- lib 坐标表格
- target_dir
- 手工增删表单

这些都下钻到详情页。

---

## 3.2 插件详情弹窗 `PluginDetailDialog`
新增一个“自动附带内容”区块。

### 区块内容
1. 安装内容摘要
   - 自动附带的 companion connector
   - 自动附带的额外依赖
2. 目录信息
   - `connectors/`
   - `lib/`
   - `plugins/<mapping>`
3. profile 选择
   - 仅对 `jdbc` 等多场景插件展示
4. 高级入口
   - `自定义依赖`

### 当没有依赖时
不展示空白表格，展示自然文案：
- `官方资料未要求此插件额外安装 lib`
或
- `暂未识别到官方依赖，建议安装前检查`

区别：
- **not_required**：表示确定不需要
- **unknown**：表示还没识别出来，不等于不需要

---

## 3.3 依赖配置弹窗 `DependencyConfigDialog`
保留为**用户自定义依赖**入口：
- 默认不自动弹出
- 默认不加载官方分析结果
- 只读写 `plugin_dependency_configs`

作用：
- 用户补充标准模板未覆盖的依赖
- 用户为特殊环境补充私有驱动
- 不覆盖系统标准模板

## 3.4 JDBC profile 模板
`connector-jdbc` 不再只维护少量 profile，而是按人工确认后的驱动矩阵维护数据库场景模板。

当前模板覆盖：
- `mysql`
- `postgresql`
- `dameng`
- `sqlserver`
- `oracle`
- `sqlite`
- `db2`
- `tablestore`
- `teradata`
- `redshift`
- `sap-hana`
- `snowflake`
- `vertica`
- `kingbasees`
- `hive`
- `oceanbase`
- `xugu`
- `iris`
- `tidb`
- `opengauss`
- `mariadb`
- `highgo`
- `presto`
- `trino`
- `duckdb`
- `aws-dsql`
- `doris`
- `starrocks`

说明：
- 版本变化由模板里的 `include_versions / exclude_versions` 控制。
- 对未来小版本默认沿用当前最新已确认模板，直到人工确认有变动。
- `Greenplum / GBase 8a` 这类源码里识别、但官方 `pom` 没锁版本的场景，当前不自动下发驱动，保留为后续人工增强。

---

## 4. 数据层设计

## 4.1 去掉插件列表内存缓存，改为数据库快照
当前 `ListAvailablePlugins` 用内存缓存 24h。
建议改为：

### 新表：`plugin_catalog_entries`
字段：
- `id`
- `seatunnel_version`
- `plugin_name`
- `display_name`
- `artifact_id`
- `group_id`
- `category`
- `doc_url`
- `source` (`seed` / `remote`)
- `created_at`
- `updated_at`

联合唯一键：
- `(seatunnel_version, plugin_name)`

### 读取逻辑
1. 优先查 DB
2. DB 有则直接返回
3. DB 没有才抓 Maven connector 列表
4. 抓到后立即落库

### 好处
- 去掉多实例不一致的内存缓存
- 重启不丢
- 版本发版后文档稳定，适合长期缓存

---

## 4.2 官方依赖基线表
### `plugin_dependency_profiles`
代表一个插件某个 profile 的官方依赖画像。

关键字段：
- `seatunnel_version`
- `plugin_name`
- `artifact_id`
- `profile_key`
- `engine_scope`
- `source_kind` (`official_seed` / `runtime_analyzed`)
- `baseline_version_used`
- `resolution_mode` (`exact` / `fallback` / `runtime`)
- `doc_source_url`
- `is_default`
- `confidence`
- `content_hash`

联合唯一键：
- `(seatunnel_version, plugin_name, profile_key, engine_scope, source_kind)`

### `plugin_dependency_profile_items`
字段：
- `profile_id`
- `group_id`
- `artifact_id`
- `version`
- `target_dir`
- `required`
- `note`

---

## 4.3 用户补充依赖
现有 `plugin_dependency_configs` 暂时保留为“用户补充层”。

MVP 先不引入 remove/override，先做：
- 官方基线
- 用户 add
- 按坐标去重 merge

后续若需要用户删除官方依赖，再扩展 action 字段。

---

## 5. Effective Dependency 计算

### 5.1 目标
安装时真正使用的依赖，不是“某张表原样返回”，而是合并后的 effective deps。

### 5.2 计算优先级
1. 官方 exact profile
2. 官方 fallback profile
3. runtime analyzed profile
4. 用户补充依赖

### 5.3 去重规则
按联合键去重：
- `group_id`
- `artifact_id`
- `version`
- `target_dir`

### 5.4 状态判定
- 有 exact 官方基线：`ready_exact`
- 没 exact 但有 fallback：`ready_fallback`
- 明确文档写了无依赖：`not_required`
- 有 runtime analyzed：`runtime_analyzed`
- 什么都没有：`unknown`

### 5.5 当前用户覆盖模型 v1
最终生效依赖 =

> 官方基线 - 用户禁用 + 用户新增

其中用户新增支持：
- Maven 坐标
- 上传自定义 jar

### 5.6 profile 与自定义上传 jar 的当前边界
当前用户禁用 / 用户新增 / 上传 jar 是按：
- `plugin_name`
- `seatunnel_version`

进行生效的，而**不是 profile 级别**。

这对绝大多数单 profile 插件没问题，但对 `jdbc` 这种多 profile 场景存在边界：
- 用户上传一个自定义 `oracle` 驱动后，如果同一插件又选择了 `mysql` profile，这个自定义 jar 也会一起参与生效依赖合并。
- 用户禁用某条官方依赖时，也是对同版本整插件生效，而不是仅对某个 profile 生效。

当前策略：
- v1 先保持简单模型，满足“官方基线 + 用户禁用 + 用户新增/上传 jar”
- 后续如果 `jdbc` 多 profile 的用户覆盖需求变强，再补 `profile_key` 维度

---

## 6. 新版本 fallback 策略
当用户加载一个新版本，比如 `2.3.13`，但我们尚未发版种子：

查找顺序：
1. `2.3.13`
2. 同 minor 最近 patch，例如 `2.3.12`
3. 同 major 最近 minor
4. runtime analyze
5. unknown

### UI 表达
不要把这种 fallback 做成错误，而是轻状态提示：
- `依赖沿用 2.3.12`

这样用户几乎无感，可以先安装使用。

---

## 7. Seed 策略

## 7.1 Seed 来源
发版前由 Go 后端抓取并标准化官方 docs，生成官方基线。

## 7.1.1 当前 MVP 调整：先内置一版 2.3.12 通用模板
本轮不再继续逐个 POM 深挖，而是直接按已确认规则内置一份 **2.3.12 通用模板**：

1. **catalog**：固化 83 个真实 connector 元数据
2. **hidden_plugins**：隐藏 `file-base`、`file-base-hadoop`
3. **default_not_required**：仅保留普通 connector-only 插件
4. **profiles**：对需要伴生 connector / 插件专属依赖的插件手工维护 profile

### 已确认的基线规则
- **官方包已自带，排除重复下载**
  - `connector-cdc-base`
  - `seatunnel-hadoop3-3.1.4-uber`
  - `seatunnel-hadoop-aws`
- **隐藏但自动附带**
  - `file-base`
  - `file-base-hadoop`
- **保留展示并可作为伴生 connector 自动附带**
  - `http-base`
- **2.3.12+ 仅对 Zeta 启用隔离依赖目录**
  - 插件专属依赖放到 `plugins/<plugin-mapping value>`
  - 共享依赖仍可放 `lib/`
  - 伴生 connector 仍放 `connectors/`
- **已确认的伴生/依赖关系**
  - file 系列：`file-s3` / `file-cos` / `file-jindo-oss` / `file-local` / `file-sftp` / `file-ftp` 自动附带 `file-base`
  - `file-hadoop`：自动附带 `file-base` + `file-base-hadoop`
  - `file-oss`：自动附带 `file-base`，额外依赖按模板放置
  - `file-obs`：自动附带 `file-base`，额外依赖固定落 `lib/`
  - http 系列：自动附带 `http-base`
  - `prometheus`：无依赖，不附带 `http-base`
  - `easysearch`：无依赖
  - CDC 系列：按已确认规则自动附带 `connector-jdbc` / `connector-cdc-postgres`
  - `jdbc`：继续按 profile 处理，不给默认 lib
- **特殊排除**
  - `file-s3` 不再追加 `hadoop-aws`
  - `file-jindo-oss` 当前不补额外第三方 jar，只自动附带 `file-base`

### 模板适用版本
- `2.3.12` 作为标准模板版本
- 默认 `applies_to = "*"`，即新旧版本都优先沿用这份模板
- 当某个插件在特定版本不适用时，再通过：
  - `include_versions`
  - `exclude_versions`
  做增量覆盖
- 例如：某条规则默认全版本生效，但可增加 `exclude_versions: ["2.3.14"]`

### 当前 seed 的目标
- 插件市场里默认看到的还是“一个插件”
- 一键安装时系统自动补齐：
  - companion connectors -> `connectors/`
  - isolated dependency jars -> `plugins/<mapping>/`
- 诊断包目录清单同步纳入 `plugins/`

后续新版本继续沿用：
- 2.3.12 通用模板
- 版本 include/exclude 增量覆盖
- 人工 review 补强

## 7.2 产物
放在初始化 SQL 目录：
- `support-files/sql/plugin_catalog_entries.seed.sqlite.sql`
- `support-files/sql/plugin_catalog_entries.seed.mysql.sql`
- `support-files/sql/plugin_catalog_entries.seed.postgres.sql`
- `support-files/sql/plugin_dependency_profiles.seed.sqlite.sql`
- `support-files/sql/plugin_dependency_profiles.seed.mysql.sql`
- `support-files/sql/plugin_dependency_profiles.seed.postgres.sql`
- `support-files/sql/plugin_dependency_profile_items.seed.sqlite.sql`
- `support-files/sql/plugin_dependency_profile_items.seed.mysql.sql`
- `support-files/sql/plugin_dependency_profile_items.seed.postgres.sql`

## 7.3 导入策略
只导入官方表：
- `plugin_catalog_entries`
- `plugin_dependency_profiles`
- `plugin_dependency_profile_items`

按联合主键 upsert：
- 有则更新
- 无则插入

不修改：
- `plugin_dependency_configs`

这样用户升级时不冲突。

---

## 8. 在线分析策略
在线分析仅保留为后台兜底能力，不作为当前插件市场的默认前端动作。

### 触发时机
- 数据库里没有标准模板命中
- 研发/运维通过后端接口手动触发补全

### 当前前端策略
- 不展示“分析官方依赖”按钮
- 不让用户感知 exact / fallback / runtime analyzed 内部状态

---

## 9. 接口设计

## 9.1 插件列表接口增强
### `GET /api/v1/plugins`
返回中给每个 plugin 增加：
- `dependency_status`
- `dependency_count`
- `dependency_baseline_version`
- `dependency_resolution_mode`

这样列表页无需额外逐个请求。

## 9.2 官方依赖详情
### `GET /api/v1/plugins/:name/official-dependencies?version=2.3.12`
返回：
- profile
- items
- status
- baseline_version_used
- source_kind
- doc_source_url

## 9.3 在线分析
### `POST /api/v1/plugins/:name/official-dependencies/analyze`
请求：
- `version`
- `profile_key`（可选）
- `force_refresh`

返回：
- 最新分析的 profile + items
- 新状态

说明：
- 该接口作为后台补全能力保留
- 当前前端默认不暴露

## 9.4 应用官方依赖
### `POST /api/v1/plugins/:name/official-dependencies/apply`
说明：
- 如需要让 runtime_analyzed 固化为用户补充依赖，可通过该接口写入用户依赖表
- MVP 可先不做 apply，先做“自动作为 effective deps 生效”

---

## 10. 安装链路
安装插件时，不再只下载 connector。

### 新流程
1. 查 `plugin_catalog_entries`
2. 计算 effective deps
3. 下载主 connector
4. 下载 companion connectors（若有）到 `connectors/`
5. 下载插件专属依赖到 `plugins/<mapping>/`（2.3.12+）
6. 一起传给 agent
7. 安装到远端

### 2.3.12+ 路径约定
- 主 connector / companion connector：`connectors/`
- 插件专属依赖：`plugins/<mapping value>/`
- 共享依赖：`lib/`

### 用户视角
仍然只是点击：
- 安装插件

系统自动附带 lib。

---

## 11. 无依赖场景处理
有些连接器确实没有依赖，这必须作为一等场景处理。

### not_required
表示：
- 官方资料明确该插件不需要额外 lib

UI：
- `无需额外依赖`
- 不告警，不打扰用户

### unknown
表示：
- 当前版本 / profile 暂未识别到官方依赖
- 不等于不需要

UI：
- `依赖待确认`
- 次级提示：`建议安装前检查`

这两个状态不能混淆。

---

## 12. MVP 范围建议
先做：
1. DB 持久化插件目录
2. 官方依赖 profile 表
3. exact / fallback 读取
4. `/api/v1/plugins` 轻状态增强
5. 插件详情页官方依赖区块
6. 一键安装链路支持 `plugins/<mapping>`
7. 诊断包目录清单纳入 `plugins/`
8. runtime analyze 接口（后台兜底）

第二阶段再做：
7. 依赖配置弹窗双层化
8. apply official profile
9. 更多 profile（Hive on S3 / OSS 等）

---

## 13. 关键设计结论
1. **用户默认无感**：插件 = connector + 自动依赖
2. **lib 状态可见但轻量**：ready / fallback / none / unknown
3. **不用内存缓存**：改 DB 快照
4. **seed + fallback + runtime analyze** 三层兜底
5. **官方与用户分层**：种子不覆盖用户自定义
