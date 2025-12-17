/**
 * SeaTunnel Plugin Marketplace Service
 * SeaTunnel 插件市场服务
 */

import apiClient from '../core/api-client';
import type {
  Plugin,
  InstalledPlugin,
  MirrorSource,
  InstallPluginRequest,
  ListPluginsResponse,
  GetPluginInfoResponse,
  ListInstalledPluginsResponse,
  InstallPluginResponse,
  UninstallPluginResponse,
  EnableDisablePluginResponse,
  AvailablePluginsResponse,
} from './types';

const API_PREFIX = '/api/v1';

// ==================== Available Plugins 可用插件 ====================

/**
 * List available plugins from marketplace
 * 从插件市场获取可用插件列表
 * @param version SeaTunnel version / SeaTunnel 版本
 * @param mirror Mirror source / 镜像源
 */
export async function listAvailablePlugins(
  version?: string,
  mirror?: MirrorSource
): Promise<AvailablePluginsResponse> {
  const params = new URLSearchParams();
  if (version) params.append('version', version);
  if (mirror) params.append('mirror', mirror);

  const url = `${API_PREFIX}/plugins${params.toString() ? `?${params.toString()}` : ''}`;
  const response = await apiClient.get<ListPluginsResponse>(url);
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Get plugin info by name
 * 根据名称获取插件详情
 * @param name Plugin name / 插件名称
 * @param version SeaTunnel version / SeaTunnel 版本
 */
export async function getPluginInfo(name: string, version?: string): Promise<Plugin> {
  const params = version ? `?version=${version}` : '';
  const response = await apiClient.get<GetPluginInfoResponse>(
    `${API_PREFIX}/plugins/${name}${params}`
  );
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}


// ==================== Installed Plugins 已安装插件 ====================

/**
 * List installed plugins on a host
 * 获取主机上已安装的插件列表
 * @param hostId Host ID / 主机 ID
 */
export async function listInstalledPlugins(hostId: number | string): Promise<InstalledPlugin[]> {
  const response = await apiClient.get<ListInstalledPluginsResponse>(
    `${API_PREFIX}/hosts/${hostId}/plugins`
  );
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data || [];
}

// ==================== Plugin Installation 插件安装 ====================

/**
 * Install a plugin on a host
 * 在主机上安装插件
 * @param hostId Host ID / 主机 ID
 * @param request Install request / 安装请求
 */
export async function installPlugin(
  hostId: number | string,
  request: InstallPluginRequest
): Promise<InstalledPlugin> {
  const response = await apiClient.post<InstallPluginResponse>(
    `${API_PREFIX}/hosts/${hostId}/plugins`,
    request
  );
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Uninstall a plugin from a host
 * 从主机卸载插件
 * @param hostId Host ID / 主机 ID
 * @param pluginName Plugin name / 插件名称
 */
export async function uninstallPlugin(
  hostId: number | string,
  pluginName: string
): Promise<void> {
  const response = await apiClient.delete<UninstallPluginResponse>(
    `${API_PREFIX}/hosts/${hostId}/plugins/${pluginName}`
  );
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
}

// ==================== Plugin Enable/Disable 插件启用/禁用 ====================

/**
 * Enable a plugin on a host
 * 在主机上启用插件
 * @param hostId Host ID / 主机 ID
 * @param pluginName Plugin name / 插件名称
 */
export async function enablePlugin(
  hostId: number | string,
  pluginName: string
): Promise<InstalledPlugin> {
  const response = await apiClient.put<EnableDisablePluginResponse>(
    `${API_PREFIX}/hosts/${hostId}/plugins/${pluginName}/enable`,
    {}
  );
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Disable a plugin on a host
 * 在主机上禁用插件
 * @param hostId Host ID / 主机 ID
 * @param pluginName Plugin name / 插件名称
 */
export async function disablePlugin(
  hostId: number | string,
  pluginName: string
): Promise<InstalledPlugin> {
  const response = await apiClient.put<EnableDisablePluginResponse>(
    `${API_PREFIX}/hosts/${hostId}/plugins/${pluginName}/disable`,
    {}
  );
  
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

// ==================== Export all functions 导出所有函数 ====================

export const pluginService = {
  // Available plugins / 可用插件
  listAvailablePlugins,
  getPluginInfo,
  // Installed plugins / 已安装插件
  listInstalledPlugins,
  // Installation / 安装
  installPlugin,
  uninstallPlugin,
  // Enable/Disable / 启用/禁用
  enablePlugin,
  disablePlugin,
};
