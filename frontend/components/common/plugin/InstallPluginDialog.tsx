/**
 * Install Plugin Dialog Component
 * 安装插件对话框组件
 *
 * Shows cluster list with install/enable/disable actions for each cluster
 * 显示集群列表，每个集群可以安装/启用/禁用插件
 */

'use client';

import { useState, useEffect, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Loader2,
  Download,
  Server,
  AlertCircle,
  Info,
  CloudDownload,
  Power,
  PowerOff,
} from 'lucide-react';
import { toast } from 'sonner';
import type { Plugin, PluginDownloadProgress, InstalledPlugin } from '@/lib/services/plugin';
import { PluginService } from '@/lib/services/plugin';
import { ClusterService } from '@/lib/services/cluster';
import type { ClusterInfo } from '@/lib/services/cluster';

interface InstallPluginDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  plugin: Plugin;
  version: string;
}

// Cluster plugin status info / 集群插件状态信息
interface ClusterPluginStatus {
  cluster: ClusterInfo;
  installedPlugin: InstalledPlugin | null;
  loading: boolean;
}

/**
 * Install Plugin Dialog Component
 * 安装插件对话框组件
 */
export function InstallPluginDialog({
  open,
  onOpenChange,
  plugin,
  version,
}: InstallPluginDialogProps) {
  const t = useTranslations();

  // State / 状态
  const [clusterStatuses, setClusterStatuses] = useState<ClusterPluginStatus[]>([]);
  const [loadingClusters, setLoadingClusters] = useState(true);
  const [downloading, setDownloading] = useState(false);
  const [downloadProgress, setDownloadProgress] = useState<PluginDownloadProgress | null>(null);
  const [isDownloaded, setIsDownloaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Track which cluster is being operated / 跟踪正在操作的集群
  const [operatingClusterId, setOperatingClusterId] = useState<number | null>(null);

  /**
   * Check download status
   * 检查下载状态
   */
  const checkDownloadStatus = useCallback(async () => {
    try {
      const status = await PluginService.getDownloadStatus(plugin.name, version);
      setDownloadProgress(status);
      if (status.status === 'completed') {
        setIsDownloaded(true);
        setDownloading(false);
      } else if (status.status === 'downloading') {
        setDownloading(true);
      } else if (status.status === 'failed') {
        setDownloading(false);
        setError(status.error || t('plugin.downloadFailed'));
      }
    } catch {
      // If status check fails, assume not downloaded / 如果状态检查失败，假设未下载
      setIsDownloaded(false);
    }
  }, [plugin.name, version, t]);

  /**
   * Load available clusters and check download status
   * 加载可用集群列表并检查下载状态
   */
  useEffect(() => {
    if (open) {
      loadClusters();
      checkDownloadStatus();
    }
  }, [open, checkDownloadStatus]);

  /**
   * Poll download status while downloading
   * 下载时轮询下载状态
   */
  useEffect(() => {
    if (!downloading) return;

    const interval = setInterval(checkDownloadStatus, 1000);
    return () => clearInterval(interval);
  }, [downloading, checkDownloadStatus]);

  /**
   * Load clusters and check plugin status for each
   * 加载集群列表并检查每个集群的插件状态
   */
  const loadClusters = async () => {
    setLoadingClusters(true);
    setError(null);
    try {
      const result = await ClusterService.getClusters({ current: 1, size: 100 });
      // Filter clusters that are online/running / 过滤在线/运行中的集群
      const availableClusters = result.clusters.filter(
        (c: ClusterInfo) => c.status === 'running' || c.status === 'stopped'
      );

      // Initialize cluster statuses / 初始化集群状态
      const statuses: ClusterPluginStatus[] = availableClusters.map((cluster: ClusterInfo) => ({
        cluster,
        installedPlugin: null,
        loading: true,
      }));
      setClusterStatuses(statuses);

      // Check plugin status for each cluster / 检查每个集群的插件状态
      const updatedStatuses = await Promise.all(
        availableClusters.map(async (cluster: ClusterInfo) => {
          try {
            const installedPlugins = await PluginService.listInstalledPlugins(cluster.id);
            const installedPlugin = installedPlugins.find(
              (p: InstalledPlugin) => p.plugin_name === plugin.name
            );
            return {
              cluster,
              installedPlugin: installedPlugin || null,
              loading: false,
            };
          } catch {
            return {
              cluster,
              installedPlugin: null,
              loading: false,
            };
          }
        })
      );
      setClusterStatuses(updatedStatuses);
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('cluster.loadError');
      setError(errorMsg);
      setClusterStatuses([]);
    } finally {
      setLoadingClusters(false);
    }
  };

  /**
   * Handle download plugin
   * 处理下载插件
   */
  const handleDownload = async () => {
    setDownloading(true);
    setError(null);
    try {
      await PluginService.downloadPlugin(plugin.name, version);
      toast.info(t('plugin.downloadStarted', { name: plugin.display_name || plugin.name }));
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.downloadFailed');
      setError(errorMsg);
      toast.error(errorMsg);
      setDownloading(false);
    }
  };

  /**
   * Handle install plugin to cluster
   * 处理安装插件到集群
   */
  const handleInstall = async (clusterId: number) => {
    if (!isDownloaded) {
      toast.error(t('plugin.downloadFirst'));
      return;
    }

    setOperatingClusterId(clusterId);
    try {
      await PluginService.installPlugin(clusterId, plugin.name, version);
      toast.success(t('plugin.installSuccess', { name: plugin.display_name || plugin.name }));
      // Refresh cluster status / 刷新集群状态
      await refreshClusterStatus(clusterId);
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.installFailed');
      toast.error(errorMsg);
    } finally {
      setOperatingClusterId(null);
    }
  };

  /**
   * Handle enable plugin on cluster
   * 处理在集群上启用插件
   */
  const handleEnable = async (clusterId: number) => {
    setOperatingClusterId(clusterId);
    try {
      await PluginService.enablePlugin(clusterId, plugin.name);
      toast.success(t('plugin.enableSuccess'));
      await refreshClusterStatus(clusterId);
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.enableError');
      toast.error(errorMsg);
    } finally {
      setOperatingClusterId(null);
    }
  };

  /**
   * Handle disable plugin on cluster
   * 处理在集群上禁用插件
   */
  const handleDisable = async (clusterId: number) => {
    setOperatingClusterId(clusterId);
    try {
      await PluginService.disablePlugin(clusterId, plugin.name);
      toast.success(t('plugin.disableSuccess'));
      await refreshClusterStatus(clusterId);
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : t('plugin.disableError');
      toast.error(errorMsg);
    } finally {
      setOperatingClusterId(null);
    }
  };

  /**
   * Refresh single cluster status
   * 刷新单个集群状态
   */
  const refreshClusterStatus = async (clusterId: number) => {
    try {
      const installedPlugins = await PluginService.listInstalledPlugins(clusterId);
      const installedPlugin = installedPlugins.find(
        (p: InstalledPlugin) => p.plugin_name === plugin.name
      );
      setClusterStatuses((prev) =>
        prev.map((cs) =>
          cs.cluster.id === clusterId
            ? { ...cs, installedPlugin: installedPlugin || null }
            : cs
        )
      );
    } catch {
      // Ignore refresh errors / 忽略刷新错误
    }
  };

  /**
   * Handle dialog close
   * 处理对话框关闭
   */
  const handleClose = () => {
    if (!downloading && !operatingClusterId) {
      setError(null);
      setDownloadProgress(null);
      onOpenChange(false);
    }
  };

  /**
   * Get status badge for installed plugin
   * 获取已安装插件的状态徽章
   */
  const getStatusBadge = (installedPlugin: InstalledPlugin | null) => {
    if (!installedPlugin) {
      return (
        <Badge variant="outline" className="text-muted-foreground">
          {t('plugin.notInstalled')}
        </Badge>
      );
    }
    const isEnabled = installedPlugin.status === 'enabled' || installedPlugin.status === 'installed';
    return (
      <Badge variant={isEnabled ? 'default' : 'secondary'}>
        {t(`plugin.statuses.${installedPlugin.status}`)}
      </Badge>
    );
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-[650px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Download className="h-5 w-5" />
            {t('plugin.managePlugin')}
          </DialogTitle>
          <DialogDescription>{t('plugin.managePluginDesc')}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Plugin info / 插件信息 */}
          <Card className="bg-muted/50">
            <CardContent className="pt-4 pb-4 space-y-2">
              <div className="flex items-center justify-between">
                <span className="font-medium">{plugin.display_name || plugin.name}</span>
                <Badge variant="secondary">v{version}</Badge>
              </div>
              <p className="text-sm text-muted-foreground">{plugin.name}</p>
            </CardContent>
          </Card>

          {/* Download section / 下载区域 */}
          {!isDownloaded && (
            <Card className="border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950">
              <CardContent className="pt-4 pb-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 text-amber-800 dark:text-amber-200">
                    <AlertCircle className="h-4 w-4" />
                    <span className="text-sm">{t('plugin.downloadFirst')}</span>
                  </div>
                  <Button size="sm" onClick={handleDownload} disabled={downloading}>
                    {downloading ? (
                      <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    ) : (
                      <CloudDownload className="h-4 w-4 mr-2" />
                    )}
                    {t('plugin.download')}
                  </Button>
                </div>
                {/* Download progress / 下载进度 */}
                {downloading && downloadProgress && (
                  <div className="mt-3 space-y-2">
                    <div className="flex items-center justify-between text-sm">
                      <span>{downloadProgress.current_step || t('plugin.downloading')}</span>
                      <span>{downloadProgress.progress}%</span>
                    </div>
                    <Progress value={downloadProgress.progress} className="h-2" />
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          {/* Error display / 错误显示 */}
          {error && (
            <Card className="border-destructive bg-destructive/10">
              <CardContent className="pt-4 pb-4">
                <div className="flex items-center gap-2 text-destructive">
                  <AlertCircle className="h-4 w-4" />
                  <span className="text-sm">{error}</span>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Cluster list / 集群列表 */}
          <div className="space-y-2">
            <div className="text-sm font-medium">{t('plugin.clusterList')}</div>
            {loadingClusters ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : clusterStatuses.length === 0 ? (
              <Card className="border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950">
                <CardContent className="pt-4 pb-4">
                  <div className="flex items-center gap-2 text-amber-800 dark:text-amber-200">
                    <AlertCircle className="h-4 w-4" />
                    <span className="text-sm">{t('plugin.noClustersAvailable')}</span>
                  </div>
                </CardContent>
              </Card>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('cluster.name')}</TableHead>
                    <TableHead>{t('cluster.status')}</TableHead>
                    <TableHead>{t('plugin.status.label')}</TableHead>
                    <TableHead className="text-right">{t('common.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {clusterStatuses.map(({ cluster, installedPlugin, loading }) => {
                    const isOperating = operatingClusterId === cluster.id;
                    const isInstalled = !!installedPlugin;
                    const isEnabled =
                      installedPlugin?.status === 'enabled' ||
                      installedPlugin?.status === 'installed';

                    return (
                      <TableRow key={cluster.id}>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <Server className="h-4 w-4 text-muted-foreground" />
                            <span className="font-medium">{cluster.name}</span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={cluster.status === 'running' ? 'default' : 'secondary'}
                          >
                            {t(`cluster.statuses.${cluster.status}`)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {loading ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            getStatusBadge(installedPlugin)
                          )}
                        </TableCell>
                        <TableCell className="text-right">
                          {loading ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : !isInstalled ? (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleInstall(cluster.id)}
                              disabled={!isDownloaded || isOperating}
                            >
                              {isOperating ? (
                                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                              ) : (
                                <Download className="h-4 w-4 mr-1" />
                              )}
                              {t('plugin.install')}
                            </Button>
                          ) : isEnabled ? (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleDisable(cluster.id)}
                              disabled={isOperating}
                            >
                              {isOperating ? (
                                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                              ) : (
                                <PowerOff className="h-4 w-4 mr-1 text-orange-600" />
                              )}
                              {t('plugin.disable')}
                            </Button>
                          ) : (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => handleEnable(cluster.id)}
                              disabled={isOperating}
                            >
                              {isOperating ? (
                                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                              ) : (
                                <Power className="h-4 w-4 mr-1 text-green-600" />
                              )}
                              {t('plugin.enable')}
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )}
          </div>

          {/* Install note / 安装说明 */}
          <Card className="bg-blue-50 dark:bg-blue-950 border-blue-200 dark:border-blue-800">
            <CardContent className="pt-4 pb-4">
              <div className="flex items-start gap-2">
                <Info className="h-4 w-4 text-blue-600 mt-0.5" />
                <p className="text-xs text-blue-800 dark:text-blue-200">
                  {t('plugin.installNote')}
                </p>
              </div>
            </CardContent>
          </Card>
        </div>
      </DialogContent>
    </Dialog>
  );
}
