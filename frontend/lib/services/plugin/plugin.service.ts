/**
 * SeaTunnel Plugin Marketplace Service
 * SeaTunnel 插件市场服务
 */

import { BaseService } from '../core/base.service';
import type {
  Plugin,
  InstalledPlugin,
  AvailablePluginsResponse,
  MirrorSource,
  InstallPluginRequest,
  PluginDownloadProgress,
  LocalPlugin,
  PluginInstallStatus,
  PluginDependencyConfig,
  AddDependencyRequest,
} from './types';

// Import DownloadAllPluginsProgress type / 导入下载所有插件进度类型
type DownloadAllPluginsProgress = {
  total: number;
  downloaded: number;
  failed: number;
  skipped: number;
  status: string;
  message: string;
};

/**
 * Plugin service for managing SeaTunnel plugins
 * 插件服务，用于管理 SeaTunnel 插件
 */
export class PluginService extends BaseService {
  protected static basePath = '/api/v1';

  // ==================== Available Plugins 可用插件 ====================

  /**
   * List available plugins from Maven repository
   * 从 Maven 仓库获取可用插件列表
   * @param version - SeaTunnel version / SeaTunnel 版本
   * @param mirror - Mirror source / 镜像源
   * @returns Available plugins response / 可用插件响应
   */
  static async listAvailablePlugins(
    version?: string,
    mirror?: MirrorSource
  ): Promise<AvailablePluginsResponse> {
    const params: Record<string, string> = {};
    if (version) {params.version = version;}
    if (mirror) {params.mirror = mirror;}
    return this.get<AvailablePluginsResponse>('/plugins', params);
  }

  /**
   * Get plugin information by name
   * 根据名称获取插件详情
   * @param name - Plugin name / 插件名称
   * @param version - SeaTunnel version / SeaTunnel 版本
   * @returns Plugin information / 插件信息
   */
  static async getPluginInfo(name: string, version?: string): Promise<Plugin> {
    const params: Record<string, string> = {};
    if (version) {params.version = version;}
    return this.get<Plugin>(`/plugins/${encodeURIComponent(name)}`, params);
  }

  // ==================== Installed Plugins 已安装插件 ====================

  /**
   * List installed plugins on a cluster
   * 获取集群上已安装的插件列表
   * @param clusterId - Cluster ID / 集群 ID
   * @returns List of installed plugins / 已安装插件列表
   */
  static async listInstalledPlugins(clusterId: number): Promise<InstalledPlugin[]> {
    return this.get<InstalledPlugin[]>(`/clusters/${clusterId}/plugins`);
  }

  // ==================== Plugin Installation 插件安装 ====================

  /**
   * Install a plugin on a cluster
   * 在集群上安装插件
   * @param clusterId - Cluster ID / 集群 ID
   * @param pluginName - Plugin name / 插件名称
   * @param version - Plugin version / 插件版本
   * @param mirror - Mirror source / 镜像源
   * @returns Installed plugin information / 已安装插件信息
   */
  static async installPlugin(
    clusterId: number,
    pluginName: string,
    version: string,
    mirror?: MirrorSource
  ): Promise<InstalledPlugin> {
    const request: InstallPluginRequest = {
      plugin_name: pluginName,
      version,
      mirror,
    };
    return this.post<InstalledPlugin>(`/clusters/${clusterId}/plugins`, request);
  }

  /**
   * Uninstall a plugin from a cluster
   * 从集群卸载插件
   * @param clusterId - Cluster ID / 集群 ID
   * @param pluginName - Plugin name / 插件名称
   */
  static async uninstallPlugin(clusterId: number, pluginName: string): Promise<void> {
    await this.delete<unknown>(`/clusters/${clusterId}/plugins/${encodeURIComponent(pluginName)}`);
  }

  // ==================== Plugin Enable/Disable 插件启用/禁用 ====================

  /**
   * Enable a plugin on a cluster
   * 在集群上启用插件
   * @param clusterId - Cluster ID / 集群 ID
   * @param pluginName - Plugin name / 插件名称
   * @returns Updated plugin information / 更新后的插件信息
   */
  static async enablePlugin(clusterId: number, pluginName: string): Promise<InstalledPlugin> {
    return this.put<InstalledPlugin>(
      `/clusters/${clusterId}/plugins/${encodeURIComponent(pluginName)}/enable`
    );
  }

  /**
   * Disable a plugin on a cluster
   * 在集群上禁用插件
   * @param clusterId - Cluster ID / 集群 ID
   * @param pluginName - Plugin name / 插件名称
   * @returns Updated plugin information / 更新后的插件信息
   */
  static async disablePlugin(clusterId: number, pluginName: string): Promise<InstalledPlugin> {
    return this.put<InstalledPlugin>(
      `/clusters/${clusterId}/plugins/${encodeURIComponent(pluginName)}/disable`
    );
  }

  // ==================== Plugin Download 插件下载 ====================

  /**
   * Download a plugin to Control Plane local storage
   * 下载插件到 Control Plane 本地存储
   * @param pluginName - Plugin name / 插件名称
   * @param version - Plugin version / 插件版本
   * @param mirror - Mirror source / 镜像源
   * @returns Download progress / 下载进度
   */
  static async downloadPlugin(
    pluginName: string,
    version: string,
    mirror?: MirrorSource
  ): Promise<PluginDownloadProgress> {
    return this.post<PluginDownloadProgress>(
      `/plugins/${encodeURIComponent(pluginName)}/download`,
      { version, mirror }
    );
  }

  /**
   * Get download status for a plugin
   * 获取插件的下载状态
   * @param pluginName - Plugin name / 插件名称
   * @param version - Plugin version / 插件版本
   * @returns Download progress / 下载进度
   */
  static async getDownloadStatus(
    pluginName: string,
    version: string
  ): Promise<PluginDownloadProgress> {
    return this.get<PluginDownloadProgress>(
      `/plugins/${encodeURIComponent(pluginName)}/download/status`,
      { version }
    );
  }

  /**
   * List locally downloaded plugins
   * 获取已下载的本地插件列表
   * @returns List of local plugins / 本地插件列表
   */
  static async listLocalPlugins(): Promise<LocalPlugin[]> {
    return this.get<LocalPlugin[]>('/plugins/local');
  }

  /**
   * List active download tasks
   * 获取活动下载任务列表
   * @returns List of active downloads / 活动下载列表
   */
  static async listActiveDownloads(): Promise<PluginDownloadProgress[]> {
    return this.get<PluginDownloadProgress[]>('/plugins/downloads');
  }

  /**
   * Delete a locally downloaded plugin
   * 删除本地已下载的插件
   * @param pluginName - Plugin name / 插件名称
   * @param version - Plugin version / 插件版本
   */
  static async deleteLocalPlugin(pluginName: string, version: string): Promise<void> {
    await this.delete<unknown>(
      `/plugins/${encodeURIComponent(pluginName)}/local`,
      { version }
    );
  }

  /**
   * Download all plugins for a version
   * 一键下载指定版本的所有插件
   * @param version - Plugin version / 插件版本
   * @param mirror - Mirror source / 镜像源
   * @returns Download progress / 下载进度
   */
  static async downloadAllPlugins(
    version: string,
    mirror?: MirrorSource
  ): Promise<DownloadAllPluginsProgress> {
    return this.post<DownloadAllPluginsProgress>(
      '/plugins/download-all',
      { version, mirror }
    );
  }

  // ==================== Plugin Installation Progress 插件安装进度 ====================

  /**
   * Get plugin installation progress on a cluster
   * 获取集群上插件的安装进度
   * @param clusterId - Cluster ID / 集群 ID
   * @param pluginName - Plugin name / 插件名称
   * @returns Installation progress / 安装进度
   */
  static async getInstallProgress(
    clusterId: number,
    pluginName: string
  ): Promise<PluginInstallStatus | null> {
    return this.get<PluginInstallStatus | null>(
      `/clusters/${clusterId}/plugins/${encodeURIComponent(pluginName)}/progress`
    );
  }

  // ==================== Plugin Dependency Config 插件依赖配置 ====================

  /**
   * List dependencies for a plugin
   * 获取插件的依赖列表
   * @param pluginName - Plugin name / 插件名称
   * @returns List of dependencies / 依赖列表
   */
  static async listDependencies(pluginName: string): Promise<PluginDependencyConfig[]> {
    return this.get<PluginDependencyConfig[]>(
      `/plugins/${encodeURIComponent(pluginName)}/dependencies`
    );
  }

  /**
   * Add a dependency to a plugin
   * 为插件添加依赖
   * @param pluginName - Plugin name / 插件名称
   * @param dependency - Dependency info / 依赖信息
   * @returns Created dependency / 创建的依赖
   */
  static async addDependency(
    pluginName: string,
    dependency: AddDependencyRequest
  ): Promise<PluginDependencyConfig> {
    return this.post<PluginDependencyConfig>(
      `/plugins/${encodeURIComponent(pluginName)}/dependencies`,
      dependency
    );
  }

  /**
   * Delete a dependency from a plugin
   * 删除插件的依赖
   * @param pluginName - Plugin name / 插件名称
   * @param depId - Dependency ID / 依赖 ID
   */
  static async deleteDependency(pluginName: string, depId: number): Promise<void> {
    await this.delete<unknown>(
      `/plugins/${encodeURIComponent(pluginName)}/dependencies/${depId}`
    );
  }
}
