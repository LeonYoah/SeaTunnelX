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
} from './types';

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
}
