/**
 * SeaTunnel Plugin Marketplace Types
 * SeaTunnel 插件市场类型定义
 */

// ==================== Enums 枚举 ====================

/**
 * Plugin category
 * 插件分类
 */
export type PluginCategory = 'source' | 'sink' | 'transform';

/**
 * Plugin status
 * 插件状态
 */
export type PluginStatus = 'available' | 'installed' | 'enabled' | 'disabled';

/**
 * Mirror source for downloading plugins
 * 下载插件的镜像源
 */
export type MirrorSource = 'apache' | 'aliyun' | 'huaweicloud';

// ==================== Plugin Types 插件类型 ====================

/**
 * Plugin dependency
 * 插件依赖项
 */
export interface PluginDependency {
  group_id: string;
  artifact_id: string;
  version: string;
  target_dir: string; // connectors/ or lib/
}

/**
 * Plugin information
 * 插件信息
 */
export interface Plugin {
  name: string;
  display_name: string;
  category: PluginCategory;
  version: string;
  description: string;
  group_id: string;
  artifact_id: string;
  dependencies?: PluginDependency[];
  icon?: string;
  doc_url?: string;
}

/**
 * Installed plugin on a cluster
 * 集群上已安装的插件
 * Note: Plugins are managed at cluster level, not host level.
 * 注意：插件在集群级别管理，而非主机级别。
 */
export interface InstalledPlugin {
  id: number;
  cluster_id: number;
  plugin_name: string;
  artifact_id?: string; // Maven artifact ID (e.g., connector-cdc-mysql)
  category: PluginCategory;
  version: string;
  status: PluginStatus;
  install_path: string;
  installed_at: string;
  updated_at: string;
  installed_by?: number;
}

// ==================== Request Types 请求类型 ====================

/**
 * Install plugin request
 * 安装插件请求
 */
export interface InstallPluginRequest {
  plugin_name: string;
  version: string;
  mirror?: MirrorSource;
}

/**
 * Plugin filter options
 * 插件过滤选项
 */
export interface PluginFilter {
  cluster_id?: number;
  category?: PluginCategory;
  status?: PluginStatus;
  keyword?: string;
  page?: number;
  page_size?: number;
}

// ==================== Response Types 响应类型 ====================

/**
 * Available plugins response
 * 可用插件响应
 */
export interface AvailablePluginsResponse {
  plugins: Plugin[];
  total: number;
  version: string;
  mirror: string;
}

/**
 * List available plugins API response
 * 获取可用插件列表 API 响应
 */
export interface ListPluginsResponse {
  error_msg: string;
  data: AvailablePluginsResponse | null;
}

/**
 * Get plugin info API response
 * 获取插件详情 API 响应
 */
export interface GetPluginInfoResponse {
  error_msg: string;
  data: Plugin | null;
}

/**
 * List installed plugins API response
 * 获取已安装插件列表 API 响应
 */
export interface ListInstalledPluginsResponse {
  error_msg: string;
  data: InstalledPlugin[] | null;
}

/**
 * Install plugin API response
 * 安装插件 API 响应
 */
export interface InstallPluginResponse {
  error_msg: string;
  data: InstalledPlugin | null;
}

/**
 * Uninstall plugin API response
 * 卸载插件 API 响应
 */
export interface UninstallPluginResponse {
  error_msg: string;
  data: unknown;
}

/**
 * Enable/Disable plugin API response
 * 启用/禁用插件 API 响应
 */
export interface EnableDisablePluginResponse {
  error_msg: string;
  data: InstalledPlugin | null;
}

/**
 * Plugin installation status
 * 插件安装状态
 */
export interface PluginInstallStatus {
  plugin_name: string;
  status: string;
  progress: number;
  message?: string;
  error?: string;
}


// ==================== Plugin Download Types 插件下载类型 ====================

/**
 * Plugin download progress
 * 插件下载进度
 */
export interface PluginDownloadProgress {
  plugin_name: string;
  version: string;
  status: 'not_started' | 'downloading' | 'completed' | 'failed';
  progress: number;
  current_step?: string;
  downloaded_bytes?: number;
  total_bytes?: number;
  speed?: number;
  message?: string;
  error?: string;
  start_time?: string;
  end_time?: string;
}

/**
 * Local plugin (downloaded to Control Plane)
 * 本地插件（已下载到 Control Plane）
 */
export interface LocalPlugin {
  name: string;
  version: string;
  category: PluginCategory;
  connector_path: string;
  size: number;
  downloaded_at: string;
}

/**
 * Download plugin request
 * 下载插件请求
 */
export interface DownloadPluginRequest {
  version: string;
  mirror?: MirrorSource;
}

/**
 * Download plugin response
 * 下载插件响应
 */
export interface DownloadPluginResponse {
  error_msg: string;
  data: PluginDownloadProgress | null;
}

/**
 * List local plugins response
 * 获取本地插件列表响应
 */
export interface ListLocalPluginsResponse {
  error_msg: string;
  data: LocalPlugin[] | null;
}

/**
 * Get install progress response
 * 获取安装进度响应
 */
export interface GetInstallProgressResponse {
  error_msg: string;
  data: PluginInstallStatus | null;
}
