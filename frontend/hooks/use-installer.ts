/**
 * Installer Hooks
 * 安装管理 Hooks
 */

'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import { installerService } from '@/lib/services/installer';
import type {
  AvailableVersions,
  PackageInfo,
  PrecheckResult,
  PrecheckRequest,
  InstallationStatus,
  InstallationRequest,
  InstallStep,
  DeploymentMode,
  NodeRole,
  InstallMode,
  MirrorSource,
  JVMConfig,
  CheckpointConfig,
  ConnectorConfig,
  DownloadTask,
} from '@/lib/services/installer/types';

// ==================== usePackages Hook ====================

interface UsePackagesReturn {
  packages: AvailableVersions | null;
  loading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  uploadPackage: (file: File, version: string) => Promise<PackageInfo>;
  deletePackage: (version: string) => Promise<void>;
  startDownload: (version: string, mirror?: MirrorSource) => Promise<DownloadTask>;
  downloads: DownloadTask[];
  refreshDownloads: () => Promise<void>;
  refreshVersions: () => Promise<void>;
  refreshingVersions: boolean;
}

/**
 * Hook for managing packages
 * 管理安装包的 Hook
 */
export function usePackages(): UsePackagesReturn {
  const [packages, setPackages] = useState<AvailableVersions | null>(null);
  const [downloads, setDownloads] = useState<DownloadTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshingVersions, setRefreshingVersions] = useState(false);
  const pollingRef = useRef<NodeJS.Timeout | null>(null);

  const fetchPackages = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await installerService.listPackages();
      setPackages(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch packages');
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchDownloads = useCallback(async () => {
    try {
      const data = await installerService.listDownloads();
      setDownloads(data);
      
      // Check if any download is in progress / 检查是否有下载正在进行
      const hasActiveDownload = data.some(
        (d) => d.status === 'downloading' || d.status === 'pending'
      );
      
      // Start or stop polling based on active downloads / 根据活动下载启动或停止轮询
      if (hasActiveDownload && !pollingRef.current) {
        pollingRef.current = setInterval(async () => {
          const updated = await installerService.listDownloads();
          setDownloads(updated);
          
          // Check if download completed, refresh packages / 检查下载是否完成，刷新安装包列表
          const stillActive = updated.some(
            (d) => d.status === 'downloading' || d.status === 'pending'
          );
          if (!stillActive) {
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
            // Refresh packages after download completes / 下载完成后刷新安装包列表
            fetchPackages();
          }
        }, 1000);
      } else if (!hasActiveDownload && pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }
    } catch (err) {
      // Ignore errors for download list / 忽略下载列表的错误
    }
  }, [fetchPackages]);

  useEffect(() => {
    fetchPackages();
    fetchDownloads();
    
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
      }
    };
  }, [fetchPackages, fetchDownloads]);

  const uploadPackage = useCallback(async (file: File, version: string) => {
    const result = await installerService.uploadPackage(file, version);
    await fetchPackages(); // Refresh list after upload
    return result;
  }, [fetchPackages]);

  const deletePackage = useCallback(async (version: string) => {
    await installerService.deletePackage(version);
    await fetchPackages(); // Refresh list after delete
  }, [fetchPackages]);

  const startDownload = useCallback(async (version: string, mirror?: MirrorSource) => {
    const task = await installerService.startDownload(version, mirror);
    await fetchDownloads(); // Start polling
    return task;
  }, [fetchDownloads]);

  const refreshVersions = useCallback(async () => {
    try {
      setRefreshingVersions(true);
      setError(null);
      const result = await installerService.refreshVersions();
      // Update packages with new versions / 使用新版本更新安装包
      if (packages) {
        setPackages({
          ...packages,
          versions: result.versions,
        });
      }
      if (result.warning) {
        setError(result.warning);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh versions');
    } finally {
      setRefreshingVersions(false);
    }
  }, [packages]);

  return {
    packages,
    loading,
    error,
    refresh: fetchPackages,
    uploadPackage,
    deletePackage,
    startDownload,
    downloads,
    refreshDownloads: fetchDownloads,
    refreshVersions,
    refreshingVersions,
  };
}

// ==================== usePrecheck Hook ====================

interface UsePrecheckReturn {
  result: PrecheckResult | null;
  loading: boolean;
  error: string | null;
  runPrecheck: (options?: PrecheckRequest) => Promise<PrecheckResult>;
  reset: () => void;
}

/**
 * Hook for running precheck on a host
 * 在主机上运行预检查的 Hook
 */
export function usePrecheck(hostId: number | string): UsePrecheckReturn {
  const [result, setResult] = useState<PrecheckResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runPrecheck = useCallback(async (options?: PrecheckRequest) => {
    try {
      setLoading(true);
      setError(null);
      const data = await installerService.runPrecheck(hostId, options);
      setResult(data);
      return data;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Precheck failed';
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [hostId]);

  const reset = useCallback(() => {
    setResult(null);
    setError(null);
  }, []);

  return {
    result,
    loading,
    error,
    runPrecheck,
    reset,
  };
}

// ==================== useInstallation Hook ====================

interface UseInstallationReturn {
  status: InstallationStatus | null;
  loading: boolean;
  error: string | null;
  startInstallation: (request: Omit<InstallationRequest, 'host_id'>) => Promise<InstallationStatus>;
  retryStep: (step: string) => Promise<InstallationStatus>;
  cancelInstallation: () => Promise<InstallationStatus>;
  refresh: () => Promise<InstallationStatus | null>;
  stopPolling: () => void;
}

/**
 * Hook for managing installation on a host
 * 管理主机安装的 Hook
 */
export function useInstallation(
  hostId: number | string,
  pollInterval: number = 2000
): UseInstallationReturn {
  const [status, setStatus] = useState<InstallationStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollingRef = useRef<NodeJS.Timeout | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const data = await installerService.getInstallationStatus(hostId);
      setStatus(data);
      setError(null);
      return data;
    } catch (err) {
      // Installation not found is not an error, just means no installation
      if (err instanceof Error && err.message.includes('not found')) {
        setStatus(null);
        return null;
      }
      setError(err instanceof Error ? err.message : 'Failed to fetch status');
      return null;
    }
  }, [hostId]);

  // Start polling when installation is running
  useEffect(() => {
    if (status?.status === 'running') {
      pollingRef.current = setInterval(fetchStatus, pollInterval);
    } else {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
      }
    }

    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
      }
    };
  }, [status?.status, fetchStatus, pollInterval]);

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  const startInstallation = useCallback(
    async (request: Omit<InstallationRequest, 'host_id'>) => {
      try {
        setLoading(true);
        setError(null);
        const data = await installerService.startInstallation(hostId, request);
        setStatus(data);
        return data;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Installation failed';
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [hostId]
  );

  const retryStep = useCallback(
    async (step: string) => {
      try {
        setLoading(true);
        setError(null);
        const data = await installerService.retryStep(hostId, step);
        setStatus(data);
        return data;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Retry failed';
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [hostId]
  );

  const cancelInstallation = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      stopPolling();
      const data = await installerService.cancelInstallation(hostId);
      setStatus(data);
      return data;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Cancel failed';
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [hostId, stopPolling]);

  return {
    status,
    loading,
    error,
    startInstallation,
    retryStep,
    cancelInstallation,
    refresh: fetchStatus,
    stopPolling,
  };
}

// ==================== useInstallWizard Hook ====================

export type WizardStep = 'precheck' | 'config' | 'install' | 'complete';

interface InstallWizardConfig {
  version: string;
  installMode: InstallMode;
  mirror: MirrorSource;
  packagePath: string;
  deploymentMode: DeploymentMode;
  nodeRole: NodeRole;
  clusterId: string;
  jvm: JVMConfig;
  checkpoint: CheckpointConfig;
  connector: ConnectorConfig;
}

interface UseInstallWizardReturn {
  currentStep: WizardStep;
  config: InstallWizardConfig;
  setCurrentStep: (step: WizardStep) => void;
  updateConfig: (updates: Partial<InstallWizardConfig>) => void;
  resetWizard: () => void;
  canProceed: boolean;
  buildInstallRequest: () => Omit<InstallationRequest, 'host_id'>;
}

const defaultConfig: InstallWizardConfig = {
  version: '2.3.12',
  installMode: 'online',
  mirror: 'aliyun',
  packagePath: '',
  deploymentMode: 'hybrid',
  nodeRole: 'master',
  clusterId: '',
  jvm: {
    hybrid_heap_size: 3, // GB
    master_heap_size: 2, // GB
    worker_heap_size: 2, // GB
  },
  checkpoint: {
    storage_type: 'LOCAL_FILE',
    namespace: '/tmp/seatunnel/checkpoint/',
  },
  connector: {
    install_connectors: false,
    connectors: [],
    selected_plugins: [],
  },
};

/**
 * Hook for managing install wizard state
 * 管理安装向导状态的 Hook
 */
export function useInstallWizard(): UseInstallWizardReturn {
  const [currentStep, setCurrentStep] = useState<WizardStep>('precheck');
  const [config, setConfig] = useState<InstallWizardConfig>(defaultConfig);

  const updateConfig = useCallback((updates: Partial<InstallWizardConfig>) => {
    setConfig((prev) => ({ ...prev, ...updates }));
  }, []);

  const resetWizard = useCallback(() => {
    setCurrentStep('precheck');
    setConfig(defaultConfig);
  }, []);

  const canProceed = (() => {
    switch (currentStep) {
      case 'precheck':
        return true; // Can always proceed from precheck (even with warnings)
      case 'config':
        if (!config.version) return false;
        if (config.installMode === 'offline' && !config.packagePath) return false;
        return true;
      case 'install':
        return false; // Cannot proceed during installation
      case 'complete':
        return true;
      default:
        return false;
    }
  })();

  const buildInstallRequest = useCallback((): Omit<InstallationRequest, 'host_id'> => {
    return {
      cluster_id: config.clusterId || undefined,
      version: config.version,
      install_mode: config.installMode,
      mirror: config.installMode === 'online' ? config.mirror : undefined,
      package_path: config.installMode === 'offline' ? config.packagePath : undefined,
      deployment_mode: config.deploymentMode,
      node_role: config.nodeRole,
      jvm: config.jvm,
      checkpoint: config.checkpoint,
      connector: config.connector.install_connectors ? config.connector : undefined,
    };
  }, [config]);

  return {
    currentStep,
    config,
    setCurrentStep,
    updateConfig,
    resetWizard,
    canProceed,
    buildInstallRequest,
  };
}
