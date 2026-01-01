/**
 * SeaTunnel Installer Service
 * SeaTunnel 安装管理服务
 */

import apiClient from '../core/api-client';
import type {
  AvailableVersions,
  PackageInfo,
  PrecheckRequest,
  PrecheckResult,
  InstallationRequest,
  InstallationStatus,
  ListPackagesResponse,
  GetPackageInfoResponse,
  UploadPackageResponse,
  DeletePackageResponse,
  PrecheckResponse,
  InstallResponse,
  DownloadTask,
  DownloadRequest,
  DownloadResponse,
  DownloadListResponse,
  MirrorSource,
} from './types';

const API_PREFIX = '';

// ==================== Package Management 安装包管理 ====================

/**
 * List available packages and versions
 * 获取可用安装包和版本列表
 */
export async function listPackages(): Promise<AvailableVersions> {
  const response = await apiClient.get<ListPackagesResponse>(`${API_PREFIX}/packages`);
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Get package info by version
 * 根据版本获取安装包信息
 */
export async function getPackageInfo(version: string): Promise<PackageInfo> {
  const response = await apiClient.get<GetPackageInfoResponse>(
    `${API_PREFIX}/packages/${version}`
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Upload offline package
 * 上传离线安装包
 */
export async function uploadPackage(file: File, version: string): Promise<PackageInfo> {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('version', version);

  const response = await apiClient.post<UploadPackageResponse>(
    `${API_PREFIX}/packages/upload`,
    formData,
    {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    }
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Delete local package
 * 删除本地安装包
 */
export async function deletePackage(version: string): Promise<void> {
  const response = await apiClient.delete<DeletePackageResponse>(
    `${API_PREFIX}/packages/${version}`
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
}

// ==================== Precheck 预检查 ====================

/**
 * Run precheck on a host
 * 在主机上运行预检查
 */
export async function runPrecheck(
  hostId: number | string,
  options?: PrecheckRequest
): Promise<PrecheckResult> {
  const response = await apiClient.post<PrecheckResponse>(
    `${API_PREFIX}/hosts/${hostId}/precheck`,
    options || {}
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

// ==================== Installation 安装 ====================

/**
 * Start installation on a host
 * 在主机上开始安装
 */
export async function startInstallation(
  hostId: number | string,
  request: Omit<InstallationRequest, 'host_id'>
): Promise<InstallationStatus> {
  const response = await apiClient.post<InstallResponse>(
    `${API_PREFIX}/hosts/${hostId}/install`,
    request
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Get installation status
 * 获取安装状态
 */
export async function getInstallationStatus(hostId: number | string): Promise<InstallationStatus> {
  const response = await apiClient.get<InstallResponse>(
    `${API_PREFIX}/hosts/${hostId}/install/status`
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Retry a failed installation step
 * 重试失败的安装步骤
 */
export async function retryStep(
  hostId: number | string,
  step: string
): Promise<InstallationStatus> {
  const response = await apiClient.post<InstallResponse>(
    `${API_PREFIX}/hosts/${hostId}/install/retry`,
    { step }
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Cancel ongoing installation
 * 取消正在进行的安装
 */
export async function cancelInstallation(hostId: number | string): Promise<InstallationStatus> {
  const response = await apiClient.post<InstallResponse>(
    `${API_PREFIX}/hosts/${hostId}/install/cancel`,
    {}
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

// ==================== Package Download 安装包下载 ====================

/**
 * Start downloading a package to server
 * 开始下载安装包到服务器
 */
export async function startDownload(version: string, mirror?: MirrorSource): Promise<DownloadTask> {
  const request: DownloadRequest = { version, mirror };
  const response = await apiClient.post<DownloadResponse>(
    `${API_PREFIX}/packages/download`,
    request
  );
  if (response.data.error_msg && !response.data.data) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Get download status for a version
 * 获取某版本的下载状态
 */
export async function getDownloadStatus(version: string): Promise<DownloadTask> {
  const response = await apiClient.get<DownloadResponse>(
    `${API_PREFIX}/packages/download/${version}`
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * Cancel a download
 * 取消下载
 */
export async function cancelDownload(version: string): Promise<DownloadTask> {
  const response = await apiClient.post<DownloadResponse>(
    `${API_PREFIX}/packages/download/${version}/cancel`,
    {}
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data!;
}

/**
 * List all download tasks
 * 获取所有下载任务
 */
export async function listDownloads(): Promise<DownloadTask[]> {
  const response = await apiClient.get<DownloadListResponse>(
    `${API_PREFIX}/packages/downloads`
  );
  if (response.data.error_msg) {
    throw new Error(response.data.error_msg);
  }
  return response.data.data || [];
}

// ==================== Version Management 版本管理 ====================

interface RefreshVersionsResponse {
  error_msg?: string;
  data: string[];
}

/**
 * Refresh version list from Apache Archive
 * 从 Apache Archive 刷新版本列表
 */
export async function refreshVersions(): Promise<{ versions: string[]; warning?: string }> {
  const response = await apiClient.post<RefreshVersionsResponse>(
    `${API_PREFIX}/packages/versions/refresh`,
    {}
  );
  return {
    versions: response.data.data || [],
    warning: response.data.error_msg,
  };
}

// ==================== Export all functions 导出所有函数 ====================

export const installerService = {
  // Package management / 安装包管理
  listPackages,
  getPackageInfo,
  uploadPackage,
  deletePackage,
  // Package download / 安装包下载
  startDownload,
  getDownloadStatus,
  cancelDownload,
  listDownloads,
  // Version management / 版本管理
  refreshVersions,
  // Precheck / 预检查
  runPrecheck,
  // Installation / 安装
  startInstallation,
  getInstallationStatus,
  retryStep,
  cancelInstallation,
};
