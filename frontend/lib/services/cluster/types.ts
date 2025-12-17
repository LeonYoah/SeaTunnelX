/**
 * Cluster Service Types
 * 集群服务类型定义
 *
 * This file defines all types related to cluster management.
 * 本文件定义所有与集群管理相关的类型。
 */

/* eslint-disable no-unused-vars */

/**
 * Deployment mode enumeration
 * 部署模式枚举
 */
export enum DeploymentMode {
  /** Hybrid mode - master and worker on same nodes / 混合模式 - master 和 worker 在同一节点 */
  HYBRID = 'hybrid',
  /** Separated mode - master and worker on different nodes / 分离模式 - master 和 worker 在不同节点 */
  SEPARATED = 'separated',
}

/**
 * Cluster status enumeration
 * 集群状态枚举
 */
export enum ClusterStatus {
  /** Cluster created but not deployed / 集群已创建但未部署 */
  CREATED = 'created',
  /** Cluster is being deployed / 集群正在部署中 */
  DEPLOYING = 'deploying',
  /** Cluster is running normally / 集群正常运行中 */
  RUNNING = 'running',
  /** Cluster has been stopped / 集群已停止 */
  STOPPED = 'stopped',
  /** Cluster is in error state / 集群处于错误状态 */
  ERROR = 'error',
}

/**
 * Node role enumeration
 * 节点角色枚举
 */
export enum NodeRole {
  /** Master node / 主节点 */
  MASTER = 'master',
  /** Worker node / 工作节点 */
  WORKER = 'worker',
}

/**
 * Node status enumeration
 * 节点状态枚举
 */
export enum NodeStatus {
  /** Node is pending deployment / 节点待部署 */
  PENDING = 'pending',
  /** Node is being installed / 节点正在安装 */
  INSTALLING = 'installing',
  /** Node is running normally / 节点正常运行 */
  RUNNING = 'running',
  /** Node has been stopped / 节点已停止 */
  STOPPED = 'stopped',
  /** Node is in error state / 节点处于错误状态 */
  ERROR = 'error',
}

/**
 * Health status enumeration
 * 健康状态枚举
 */
export enum HealthStatus {
  /** All nodes are online and running / 所有节点在线且运行正常 */
  HEALTHY = 'healthy',
  /** One or more nodes are offline or in error state / 一个或多个节点离线或错误 */
  UNHEALTHY = 'unhealthy',
  /** Health status cannot be determined / 无法确定健康状态 */
  UNKNOWN = 'unknown',
}

/**
 * Operation type enumeration
 * 操作类型枚举
 */
export enum OperationType {
  /** Start cluster / 启动集群 */
  START = 'start',
  /** Stop cluster / 停止集群 */
  STOP = 'stop',
  /** Restart cluster / 重启集群 */
  RESTART = 'restart',
}

/**
 * Cluster configuration type
 * 集群配置类型
 */
export type ClusterConfig = Record<string, unknown>;

/**
 * Cluster information returned from API
 * API 返回的集群信息
 */
export interface ClusterInfo {
  /** Cluster ID / 集群 ID */
  id: number;
  /** Cluster name / 集群名称 */
  name: string;
  /** Description / 描述 */
  description: string;
  /** Deployment mode / 部署模式 */
  deployment_mode: DeploymentMode;
  /** SeaTunnel version / SeaTunnel 版本 */
  version: string;
  /** Cluster status / 集群状态 */
  status: ClusterStatus;
  /** Installation directory / 安装目录 */
  install_dir: string;
  /** Cluster configuration / 集群配置 */
  config: ClusterConfig;
  /** Number of nodes / 节点数量 */
  node_count: number;
  /** Creation time / 创建时间 */
  created_at: string;
  /** Update time / 更新时间 */
  updated_at: string;
}

/**
 * Node information returned from API
 * API 返回的节点信息
 */
export interface NodeInfo {
  /** Node ID / 节点 ID */
  id: number;
  /** Cluster ID / 集群 ID */
  cluster_id: number;
  /** Host ID / 主机 ID */
  host_id: number;
  /** Host name / 主机名称 */
  host_name: string;
  /** Host IP address / 主机 IP 地址 */
  host_ip: string;
  /** Node role / 节点角色 */
  role: NodeRole;
  /** SeaTunnel installation directory / SeaTunnel 安装目录 */
  install_dir: string;
  /** Node status / 节点状态 */
  status: NodeStatus;
  /** Process PID / 进程 PID */
  process_pid: number;
  /** Process status / 进程状态 */
  process_status: string;
  /** Creation time / 创建时间 */
  created_at: string;
  /** Update time / 更新时间 */
  updated_at: string;
}

/**
 * Node status information (detailed)
 * 节点状态信息（详细）
 */
export interface NodeStatusInfo {
  /** Node ID / 节点 ID */
  node_id: number;
  /** Host ID / 主机 ID */
  host_id: number;
  /** Host name / 主机名称 */
  host_name: string;
  /** Host IP address / 主机 IP 地址 */
  host_ip: string;
  /** Node role / 节点角色 */
  role: NodeRole;
  /** Node status / 节点状态 */
  status: NodeStatus;
  /** Whether the node is online / 节点是否在线 */
  is_online: boolean;
  /** Process PID / 进程 PID */
  process_pid: number;
  /** Process status / 进程状态 */
  process_status: string;
}

/**
 * Cluster status information (detailed)
 * 集群状态信息（详细）
 */
export interface ClusterStatusInfo {
  /** Cluster ID / 集群 ID */
  cluster_id: number;
  /** Cluster name / 集群名称 */
  cluster_name: string;
  /** Cluster status / 集群状态 */
  status: ClusterStatus;
  /** Health status / 健康状态 */
  health_status: HealthStatus;
  /** Total number of nodes / 节点总数 */
  total_nodes: number;
  /** Number of online nodes / 在线节点数 */
  online_nodes: number;
  /** Number of offline nodes / 离线节点数 */
  offline_nodes: number;
  /** Node status list / 节点状态列表 */
  nodes: NodeStatusInfo[];
}

/**
 * Node operation result
 * 节点操作结果
 */
export interface NodeOperationResult {
  /** Node ID / 节点 ID */
  node_id: number;
  /** Host ID / 主机 ID */
  host_id: number;
  /** Host name / 主机名称 */
  host_name: string;
  /** Whether the operation succeeded / 操作是否成功 */
  success: boolean;
  /** Result message / 结果消息 */
  message: string;
}

/**
 * Cluster operation result
 * 集群操作结果
 */
export interface OperationResult {
  /** Cluster ID / 集群 ID */
  cluster_id: number;
  /** Operation type / 操作类型 */
  operation: OperationType;
  /** Whether the operation succeeded / 操作是否成功 */
  success: boolean;
  /** Result message / 结果消息 */
  message: string;
  /** Node operation results / 节点操作结果 */
  node_results: NodeOperationResult[];
}

/**
 * Request to create a new cluster
 * 创建新集群的请求
 */
export interface CreateClusterRequest {
  /** Cluster name (required) / 集群名称（必填） */
  name: string;
  /** Description / 描述 */
  description?: string;
  /** Deployment mode (required) / 部署模式（必填） */
  deployment_mode: DeploymentMode;
  /** SeaTunnel version / SeaTunnel 版本 */
  version?: string;
  /** Installation directory / 安装目录 */
  install_dir?: string;
  /** Cluster configuration / 集群配置 */
  config?: ClusterConfig;
}

/**
 * Request to update an existing cluster
 * 更新现有集群的请求
 */
export interface UpdateClusterRequest {
  /** Cluster name / 集群名称 */
  name?: string;
  /** Description / 描述 */
  description?: string;
  /** SeaTunnel version / SeaTunnel 版本 */
  version?: string;
  /** Installation directory / 安装目录 */
  install_dir?: string;
  /** Cluster configuration / 集群配置 */
  config?: ClusterConfig;
}

/**
 * Request to add a node to a cluster
 * 向集群添加节点的请求
 */
export interface AddNodeRequest {
  /** Host ID (required) / 主机 ID（必填） */
  host_id: number;
  /** Node role (required) / 节点角色（必填） */
  role: NodeRole;
  /** SeaTunnel installation directory / SeaTunnel 安装目录 */
  install_dir?: string;
}

/**
 * Request parameters for listing clusters
 * 获取集群列表的请求参数
 */
export interface ListClustersRequest {
  /** Current page number (1-based) / 当前页码（从 1 开始） */
  current: number;
  /** Page size / 每页数量 */
  size: number;
  /** Filter by name / 按名称过滤 */
  name?: string;
  /** Filter by status / 按状态过滤 */
  status?: ClusterStatus;
  /** Filter by deployment mode / 按部署模式过滤 */
  deployment_mode?: DeploymentMode;
}

/**
 * Cluster list data
 * 集群列表数据
 */
export interface ClusterListData {
  /** Total count / 总数量 */
  total: number;
  /** Cluster list / 集群列表 */
  clusters: ClusterInfo[];
}

/**
 * Backend response structure
 * 后端响应结构
 */
export interface BackendResponse<T = unknown> {
  /** Error message, empty string means no error / 错误信息，空字符串表示无错误 */
  error_msg: string;
  /** Response data / 响应数据 */
  data: T;
}

// ==================== Response Types 响应类型 ====================

/** List clusters response type / 获取集群列表响应类型 */
export type ListClustersResponse = BackendResponse<ClusterListData>;

/** Create cluster response type / 创建集群响应类型 */
export type CreateClusterResponse = BackendResponse<ClusterInfo>;

/** Get cluster response type / 获取集群详情响应类型 */
export type GetClusterResponse = BackendResponse<ClusterInfo>;

/** Update cluster response type / 更新集群响应类型 */
export type UpdateClusterResponse = BackendResponse<ClusterInfo>;

/** Delete cluster response type / 删除集群响应类型 */
export type DeleteClusterResponse = BackendResponse<null>;

/** Get nodes response type / 获取节点列表响应类型 */
export type GetNodesResponse = BackendResponse<NodeInfo[]>;

/** Add node response type / 添加节点响应类型 */
export type AddNodeResponse = BackendResponse<NodeInfo>;

/** Remove node response type / 移除节点响应类型 */
export type RemoveNodeResponse = BackendResponse<null>;

/** Cluster operation response type / 集群操作响应类型 */
export type ClusterOperationResponse = BackendResponse<OperationResult>;

/** Get cluster status response type / 获取集群状态响应类型 */
export type GetClusterStatusResponse = BackendResponse<ClusterStatusInfo>;
