/**
 * SeaTunnel Installer Types
 * SeaTunnel 安装管理类型定义
 */

// ==================== Enums 枚举 ====================

/**
 * Mirror source for downloading SeaTunnel packages
 * 下载 SeaTunnel 安装包的镜像源
 */
export type MirrorSource = 'aliyun' | 'apache' | 'huaweicloud';

/**
 * Installation mode
 * 安装模式
 */
export type InstallMode = 'online' | 'offline';

/**
 * Deployment mode for SeaTunnel cluster
 * SeaTunnel 集群部署模式
 */
export type DeploymentMode = 'hybrid' | 'separated';

/**
 * Node role in deployment
 * 部署中的节点角色
 */
export type NodeRole = 'master' | 'worker' | 'master/worker';

/**
 * Installation step status
 * 安装步骤状态
 */
export type StepStatus = 'pending' | 'running' | 'success' | 'failed' | 'skipped';

/**
 * Installation step names
 * 安装步骤名称
 */
export type InstallStep =
  | 'download'
  | 'verify'
  | 'extract'
  | 'configure_cluster'
  | 'configure_checkpoint'
  | 'configure_jvm'
  | 'install_plugins'
  | 'register_cluster'
  | 'complete';

/**
 * Precheck item status
 * 预检查项状态
 */
export type CheckStatus = 'passed' | 'failed' | 'warning';

/**
 * Checkpoint storage type
 * 检查点存储类型
 */
export type CheckpointStorageType = 'LOCAL_FILE' | 'HDFS' | 'OSS' | 'S3';

// ==================== Package Types 安装包类型 ====================

/**
 * Package information
 * 安装包信息
 */
export interface PackageInfo {
  version: string;
  file_name: string;
  file_size: number;
  checksum?: string;
  download_urls: Record<MirrorSource, string>;
  is_local: boolean;
  local_path?: string;
  uploaded_at?: string;
}

/**
 * Available versions response
 * 可用版本响应
 */
export interface AvailableVersions {
  versions: string[];
  recommended_version: string;
  local_packages: PackageInfo[];
}

// ==================== Configuration Types 配置类型 ====================

/**
 * JVM memory configuration (all sizes in GB)
 * JVM 内存配置（所有大小单位为 GB）
 */
export interface JVMConfig {
  /** Heap size for hybrid mode in GB / 混合模式堆大小（GB） */
  hybrid_heap_size: number;
  /** Heap size for master nodes in GB / Master 节点堆大小（GB） */
  master_heap_size: number;
  /** Heap size for worker nodes in GB / Worker 节点堆大小（GB） */
  worker_heap_size: number;
}

/**
 * Checkpoint storage configuration
 * 检查点存储配置
 */
export interface CheckpointConfig {
  storage_type: CheckpointStorageType;
  namespace: string;
  hdfs_namenode_host?: string;
  hdfs_namenode_port?: number;
  storage_endpoint?: string;
  storage_access_key?: string;
  storage_secret_key?: string;
  storage_bucket?: string;
}

/**
 * Plugin installation info for tracking plugin installation during SeaTunnel setup
 * 插件安装信息，用于跟踪 SeaTunnel 安装过程中的插件安装
 */
export interface PluginInstallInfo {
  name: string;           // 插件名称 / Plugin name
  category: string;       // 插件分类 (source/sink/transform) / Plugin category
  version: string;        // 插件版本 / Plugin version
  status: 'pending' | 'downloading' | 'installing' | 'completed' | 'failed';  // 安装状态 / Install status
  progress: number;       // 安装进度 (0-100) / Install progress
  message?: string;       // 状态消息 / Status message
  error?: string;         // 错误信息 / Error message
}

/**
 * Connector installation configuration
 * 连接器安装配置
 */
export interface ConnectorConfig {
  install_connectors: boolean;
  connectors?: string[];
  plugin_repo?: MirrorSource;
  /** Selected plugins for installation / 选择安装的插件列表 */
  selected_plugins?: string[];
}

// ==================== Installation Types 安装类型 ====================

/**
 * Installation request
 * 安装请求
 */
export interface InstallationRequest {
  host_id?: string;
  cluster_id?: string;
  version: string;
  install_dir?: string; // Installation directory / 安装目录
  install_mode: InstallMode;
  mirror?: MirrorSource;
  package_path?: string;
  deployment_mode: DeploymentMode;
  node_role: NodeRole;
  master_addresses?: string[]; // Master node addresses for cluster configuration / 集群配置的 master 节点地址
  worker_addresses?: string[]; // Worker node addresses for separated mode / 分离模式的 worker 节点地址
  cluster_port?: number; // Cluster communication port / 集群通信端口
  http_port?: number; // HTTP API port / HTTP API 端口
  jvm?: JVMConfig;
  checkpoint?: CheckpointConfig;
  connector?: ConnectorConfig;
}

/**
 * Step information
 * 步骤信息
 */
export interface StepInfo {
  step: InstallStep;
  name: string;
  description: string;
  status: StepStatus;
  progress: number;
  message?: string;
  error?: string;
  start_time?: string;
  end_time?: string;
  retryable: boolean;
}

/**
 * Installation status
 * 安装状态
 */
export interface InstallationStatus {
  id: string;
  host_id: string;
  status: StepStatus;
  current_step: InstallStep;
  steps: StepInfo[];
  progress: number;
  message?: string;
  error?: string;
  start_time: string;
  end_time?: string;
}

// ==================== Precheck Types 预检查类型 ====================

/**
 * Precheck request options
 * 预检查请求选项
 */
export interface PrecheckRequest {
  min_memory_mb?: number;
  min_cpu_cores?: number;
  min_disk_space_mb?: number;
  install_dir?: string;
  ports?: number[];
}

/**
 * Precheck item result
 * 预检查项结果
 */
export interface PrecheckItem {
  name: string;
  status: CheckStatus;
  message: string;
  details?: Record<string, unknown>;
}

/**
 * Precheck result
 * 预检查结果
 */
export interface PrecheckResult {
  items: PrecheckItem[];
  overall_status: CheckStatus;
  summary: string;
}

// ==================== API Response Types API 响应类型 ====================

/**
 * List packages response
 * 获取安装包列表响应
 */
export interface ListPackagesResponse {
  error_msg: string;
  data: AvailableVersions | null;
}

/**
 * Get package info response
 * 获取安装包信息响应
 */
export interface GetPackageInfoResponse {
  error_msg: string;
  data: PackageInfo | null;
}

/**
 * Upload package response
 * 上传安装包响应
 */
export interface UploadPackageResponse {
  error_msg: string;
  data: PackageInfo | null;
}

/**
 * Delete package response
 * 删除安装包响应
 */
export interface DeletePackageResponse {
  error_msg: string;
  data: unknown;
}

/**
 * Precheck response
 * 预检查响应
 */
export interface PrecheckResponse {
  error_msg: string;
  data: PrecheckResult | null;
}

/**
 * Installation response
 * 安装响应
 */
export interface InstallResponse {
  error_msg: string;
  data: InstallationStatus | null;
}

// ==================== Download Types 下载类型 ====================

/**
 * Download status
 * 下载状态
 */
export type DownloadStatus = 'pending' | 'downloading' | 'completed' | 'failed' | 'cancelled';

/**
 * Download task
 * 下载任务
 */
export interface DownloadTask {
  id: string;
  version: string;
  mirror: MirrorSource;
  download_url: string;
  status: DownloadStatus;
  progress: number;
  downloaded_bytes: number;
  total_bytes: number;
  speed: number;
  message?: string;
  error?: string;
  start_time: string;
  end_time?: string;
}

/**
 * Download request
 * 下载请求
 */
export interface DownloadRequest {
  version: string;
  mirror?: MirrorSource;
}

/**
 * Download response
 * 下载响应
 */
export interface DownloadResponse {
  error_msg: string;
  data: DownloadTask | null;
}

/**
 * Download list response
 * 下载列表响应
 */
export interface DownloadListResponse {
  error_msg: string;
  data: DownloadTask[] | null;
}
